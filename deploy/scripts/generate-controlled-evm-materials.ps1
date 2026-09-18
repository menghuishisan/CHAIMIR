# 本脚本在项目锁定的 EVM runtime 和 Foundry 镜像中生成受控协议合约材料。
param(
    [Parameter(Mandatory = $true)][string]$DigestLockPath,
    [Parameter(Mandatory = $true)][string]$AccountAbstractionArchive,
    [Parameter(Mandatory = $true)][string]$OpenZeppelinArchive,
    [Parameter(Mandatory = $true)][string]$OutputPath,
    [string]$RuntimeImage = "runtime/evm-foundry",
    [string]$ExistingNetwork = "",
    [string]$RpcHost = "evm",
    [string]$ControlledOpStackMaterialPath = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Import-Module (Join-Path $repoRoot "images\lib\ImageMetadata.psm1") -Force

function Read-PlatformSecrets {
    $path = Join-Path $repoRoot "deploy\config\secret.env"
    $values = Read-ChaimirEnvFile -Path $path -RequiredKeys @(
        "CHAIMIR_EVM_DEPLOYER_MNEMONIC", "CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY", "CHAIMIR_PAYMASTER_PRIVATE_KEY", "OP_GETH_JWT_SECRET"
    )
    foreach ($name in @("CHAIMIR_EVM_DEPLOYER_MNEMONIC", "CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY", "CHAIMIR_PAYMASTER_PRIVATE_KEY", "OP_GETH_JWT_SECRET")) {
        if (-not $values.ContainsKey($name) -or [string]::IsNullOrWhiteSpace($values[$name]) -or $values[$name] -match '^changeme') {
            throw "平台 Secret 源中的键无有效值: $name"
        }
    }
    return $values
}

function Invoke-Docker {
    param([string[]]$Arguments, [int]$TimeoutSeconds = 300)
    return Invoke-ChaimirDocker -Arguments $Arguments -TimeoutSeconds $TimeoutSeconds
}

function Get-LockedImageRef([hashtable]$Lock, [string]$Image) {
    $digest = $Lock[$Image]
    if ($digest -notmatch '^sha256:[0-9a-f]{64}$') { throw "候选 digest 锁缺少合法条目: $Image" }
    return "$registry/$Image@$digest"
}

function Get-ArtifactBytecode([string]$ContractName) {
    $artifactRoot = Join-Path $aaRoot 'out'
    if (-not (Test-Path -LiteralPath $artifactRoot -PathType Container)) { throw "Foundry 编译产物目录缺失: $artifactRoot" }
    $artifacts = @(Get-ChildItem -LiteralPath $artifactRoot -Recurse -File -Filter "$ContractName.json")
    if ($artifacts.Count -ne 1) { throw "Foundry 合约 $ContractName 的编译产物必须唯一，实际数量=$($artifacts.Count)" }
    $artifact = Get-Content -LiteralPath $artifacts[0].FullName -Raw | ConvertFrom-Json
    $bytecode = [string]$artifact.bytecode.object
    if ($bytecode -notmatch '^(0x)?[0-9a-fA-F]+$' -or ($bytecode.Length % 2) -ne 0) { throw "Foundry 合约 $ContractName 编译产物 bytecode 无效: $($artifacts[0].FullName)" }
    if (-not $bytecode.StartsWith('0x')) { $bytecode = "0x$bytecode" }
    return $bytecode
}

function Deploy-ControlledContract([string]$InitCode, [string]$PrivateKey, [string]$Label) {
    $rpcUrl = "http://${RpcHost}:8545"
    $initCodePath = Join-Path $workRoot ("deployment-init-" + [Guid]::NewGuid().ToString("N") + ".hex")
    [IO.File]::WriteAllText($initCodePath, $InitCode, [Text.ASCIIEncoding]::new())
    $sendShell = 'set -eu; cast send --async --json --rpc-url "{0}" --private-key "{1}" --create "$(cat /tmp/initcode)"' -f $rpcUrl, $PrivateKey
    try {
        $send = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $network, "--mount", "type=bind,source=$initCodePath,target=/tmp/initcode,readonly", "--entrypoint", "sh", $foundryRef, "-lc", $sendShell)
    } finally {
        if (Test-Path -LiteralPath $initCodePath) { Remove-Item -LiteralPath $initCodePath -Force }
    }
    if ($send.Code -ne 0) { throw "$Label 异步交易提交失败: $($send.Text)" }
    $hashMatch = [regex]::Match($send.Text, '0x[0-9a-fA-F]{64}')
    if (-not $hashMatch.Success) { throw "$Label 异步交易响应缺少合法交易哈希: $($send.Text)" }
    $transactionHash = $hashMatch.Value
    $receipt = $null
    for ($attempt = 0; $attempt -lt 90; $attempt++) {
        $receipt = Invoke-ControlledRpc 'eth_getTransactionReceipt' @($transactionHash)
        if ($null -ne $receipt) { break }
        Start-Sleep -Seconds 1
    }
    if ($null -eq $receipt) { throw "$Label 交易在限定时间内没有可读收据: tx=$transactionHash" }
    if ([string]$receipt.status -ne '0x1' -or [string]$receipt.contractAddress -notmatch '^0x[0-9a-fA-F]{40}$') { throw "$Label 链上交易未成功部署: tx=$transactionHash status=$($receipt.status)" }
    $address = [string]$receipt.contractAddress
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        $code = Invoke-ControlledRpc 'eth_getCode' @($address, 'latest')
        if ([string]$code -match '^0x[0-9a-fA-F]+$' -and [string]$code -ne '0x') { return $address }
        Start-Sleep -Seconds 1
    }
    throw "$Label 收据成功但部署地址没有可执行字节码: tx=$transactionHash address=$address"
}

function Invoke-ControlledRpc([string]$Method, [object[]]$Params) {
    $body = @{ jsonrpc = '2.0'; id = 1; method = $Method; params = @($Params) } | ConvertTo-Json -Depth 8 -Compress
    $bodyBase64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($body))
    $shell = "set -eu; printf '%s' '$bodyBase64' | base64 -d >/tmp/rpc.json; curl --connect-timeout 2 --max-time 5 -fsS -H 'content-type: application/json' --data-binary @/tmp/rpc.json http://${RpcHost}:8545"
    $arguments = @('run', '--rm', '--pull', 'never', '--network', $network, '--read-only', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges', '--tmpfs', '/tmp:rw,nosuid,nodev,noexec,size=1m', '--entrypoint', 'sh', $toolsRef, '-c', $shell)
    $response = Invoke-Docker -Arguments $arguments -TimeoutSeconds 15
    if ($response.Code -ne 0) { throw "受控 EVM JSON-RPC $Method 请求失败: $($response.Text)" }
    try { $rpcResponse = $response.Text | ConvertFrom-Json } catch { throw "受控 EVM JSON-RPC $Method 返回无效 JSON: $($response.Text)" }
    if ($rpcResponse.PSObject.Properties.Name -notcontains 'result') { throw "受控 EVM JSON-RPC $Method 响应缺少 result 字段" }
    if ($rpcResponse.PSObject.Properties.Name -contains 'error' -and $null -ne $rpcResponse.error) { throw "受控 EVM JSON-RPC $Method 返回错误: $($rpcResponse.error.message)" }
    return $rpcResponse.result
}

$resolvedLockPath = (Resolve-Path -LiteralPath $DigestLockPath).Path
$lock = Read-ChaimirDigestLock -Path $resolvedLockPath -Required
$config = Read-ChaimirEnvFile -Path (Join-Path $repoRoot "deploy\config\chaimir.env") -RequiredKeys @(
    "SUPPLY_CHAIN_REGISTRY", "SUPPLY_CHAIN_REGISTRY_ENDPOINT", "SUPPLY_CHAIN_HARBOR_EXTERNAL_URL", "IMAGE_REGISTRY"
)
$registry = Get-ChaimirCanonicalRegistry -Config $config
$secrets = Read-PlatformSecrets
$runtimeRef = Get-LockedImageRef $lock $RuntimeImage
$foundryRef = Get-LockedImageRef $lock "runtime/evm-foundry"
$toolsRef = Get-LockedImageRef $lock "base/chain-tools"
$judgeRef = Get-LockedImageRef $lock "base/judge-min"
if ($RpcHost -notmatch '^[A-Za-z0-9][A-Za-z0-9.-]*$') { throw "JSON-RPC 主机名不合法: $RpcHost" }
$controlledOpStackMaterial = $null
if ($RuntimeImage -eq "runtime/op-geth") {
    if ([string]::IsNullOrWhiteSpace($ControlledOpStackMaterialPath)) { throw "runtime/op-geth 材料生成必须提供受控 OP-Stack 材料目录" }
    $controlledOpStackMaterial = (Resolve-Path -LiteralPath $ControlledOpStackMaterialPath).Path
    foreach ($requiredMaterial in @("genesis.json", "l1-execution-state.tar")) {
        $materialFile = Join-Path $controlledOpStackMaterial $requiredMaterial
        if (-not (Test-Path -LiteralPath $materialFile -PathType Leaf) -or (Get-Item -LiteralPath $materialFile).Length -le 0) {
            throw "受控 OP-Stack 材料缺少 ${requiredMaterial}: $materialFile"
        }
    }
}
$resolvedAaArchive = (Resolve-Path -LiteralPath $AccountAbstractionArchive).Path
$resolvedOzArchive = (Resolve-Path -LiteralPath $OpenZeppelinArchive).Path
$aaArchive = $resolvedAaArchive
$ozArchive = $resolvedOzArchive
$workRoot = Join-Path $repoRoot (".tmp\backend-functional-test\evidence\controlled-evm-materials-" + [Guid]::NewGuid().ToString("N"))
$aaRoot = Join-Path $workRoot "account-abstraction"
$ozRoot = Join-Path $workRoot "openzeppelin"
$ownsRuntime = [string]::IsNullOrWhiteSpace($ExistingNetwork)
$network = if ($ownsRuntime) { "chaimir-controlled-evm-$([Guid]::NewGuid().ToString('N').Substring(0, 12))" } else { $ExistingNetwork }
$volume = if ($ownsRuntime) { "$network-state" } else { "" }
$runtime = if ($ownsRuntime) { "$network-runtime" } else { "" }

New-Item -ItemType Directory -Path $workRoot -Force | Out-Null
try {
    tar -xzf $aaArchive -C $workRoot
    tar -xzf $ozArchive -C $workRoot
    $aaExtracted = @(Get-ChildItem -LiteralPath $workRoot -Directory | Where-Object { $_.Name -like "account-abstraction-*" })[0]
    $ozExtracted = @(Get-ChildItem -LiteralPath $workRoot -Directory | Where-Object { $_.Name -like "openzeppelin-contracts-*" })[0]
    if ($null -eq $aaExtracted -or $null -eq $ozExtracted) { throw "官方协议源码归档目录不完整" }
    Move-Item -LiteralPath $aaExtracted.FullName -Destination $aaRoot
    Move-Item -LiteralPath $ozExtracted.FullName -Destination $ozRoot
    New-Item -ItemType Directory -Path (Join-Path $aaRoot "lib") -Force | Out-Null
    $foundryConfig = @"
[profile.default]
src = "contracts"
out = "out"
libs = ["lib"]
solc = "/usr/local/bin/solc-0.8.20"
optimizer = true
optimizer_runs = 200
"@
    [IO.File]::WriteAllText((Join-Path $aaRoot "foundry.toml"), $foundryConfig, [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $aaRoot "remappings.txt"), "@openzeppelin/contracts/=/work/lib/openzeppelin-contracts/contracts/", [Text.UTF8Encoding]::new($false))

    if ($ownsRuntime) {
        $networkResult = Invoke-Docker @("network", "create", "--internal", $network)
        if ($networkResult.Code -ne 0) { throw "创建受控 EVM 材料网络失败: $($networkResult.Text)" }
        $volumeResult = Invoke-Docker @("volume", "create", $volume)
        if ($volumeResult.Code -ne 0) { throw "创建受控 EVM 材料卷失败: $($volumeResult.Text)" }
        if ($RuntimeImage -eq "runtime/op-geth") {
            $genesisPath = Join-Path $controlledOpStackMaterial "genesis.json"
            $executionStatePath = Join-Path $controlledOpStackMaterial "l1-execution-state.tar"
            $seed = Invoke-Docker @("run", "--rm", "--pull", "never", "--user", "0:0", "--mount", "source=$volume,target=/runtime-state", "--mount", "type=bind,source=$controlledOpStackMaterial,target=/seed/op-stack,readonly", "--entrypoint", "sh", $judgeRef, "-lc", "set -eu; mkdir -p /runtime-state/op-geth; cp /seed/op-stack/genesis.json /runtime-state/op-geth/genesis.json; tar -xf /seed/op-stack/l1-execution-state.tar -C /runtime-state; chown -R 1000:1000 /runtime-state")
            if ($seed.Code -ne 0) { throw "受控 OP-Geth execution 状态注入失败: $($seed.Text)" }
            $runtimeArguments = @("run", "-d", "--pull", "never", "--name", $runtime, "--network", $network, "--network-alias", "evm", "-v", "${volume}:/runtime-state", "-e", "OP_GETH_DATA_DIR=/runtime-state/op-geth", "-e", "OP_GETH_GENESIS_PATH=/runtime-state/op-geth/genesis.json", "-e", "OP_GETH_JWT_SECRET=$($secrets["OP_GETH_JWT_SECRET"])", "-e", "OP_GETH_RPC_PORT=8545", "-e", "OP_GETH_ENGINE_PORT=8551", $runtimeRef)
        } elseif ($RuntimeImage -eq "runtime/evm-geth") {
            $runtimeArguments = @("run", "-d", "--pull", "never", "--name", $runtime, "--network", $network, "--network-alias", "evm", "-v", "${volume}:/runtime-state", "-e", "CHAIMIR_CHAIN_STATE_DIR=/runtime-state", "-e", "CHAIMIR_RUNTIME_RPC_PORT=8545", "-e", "CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY=$($secrets["CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY"])", $runtimeRef)
        } else {
            $runtimeArguments = @("run", "-d", "--pull", "never", "--name", $runtime, "--network", $network, "--network-alias", "evm", "-v", "${volume}:/runtime-state", "-e", "CHAIMIR_CHAIN_STATE_DIR=/runtime-state", "-e", "CHAIMIR_RUNTIME_RPC_PORT=8545", "-e", "CHAIMIR_EVM_DEPLOYER_MNEMONIC=$($secrets["CHAIMIR_EVM_DEPLOYER_MNEMONIC"])", $runtimeRef)
        }
        $runtimeResult = Invoke-Docker $runtimeArguments
        if ($runtimeResult.Code -ne 0) { throw "启动受控 EVM runtime 失败: $($runtimeResult.Text)" }
    } else {
        $networkCheck = Invoke-Docker @("network", "inspect", $network)
        if ($networkCheck.Code -ne 0) { throw "CompositionSpec runtime 网络不存在: $network" }
    }

    $ready = $false
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        $probe = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $network, "--entrypoint", "cast", $foundryRef, "chain-id", "--rpc-url", "http://${RpcHost}:8545")
        if ($probe.Code -eq 0 -and $probe.Text -match '^0x|^[0-9]+$') { $ready = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) { throw "受控 EVM runtime 在限定时间内未就绪" }

    $build = Invoke-Docker @("run", "--rm", "--pull", "never", "--user", "0:0", "--entrypoint", "sh", "-v", "${aaRoot}:/work", "-v", "${ozRoot}:/deps/openzeppelin:ro", $foundryRef, "-lc", "set -eu; ln -s /deps/openzeppelin /work/lib/openzeppelin-contracts; cd /work; forge build contracts/core/EntryPoint.sol contracts/samples/VerifyingPaymaster.sol contracts/samples/SimpleAccountFactory.sol --root /work --use /usr/local/bin/solc-0.8.20 --offline")
    if ($build.Code -ne 0) { throw "官方 ERC-4337 合约离线编译失败: $($build.Text)" }

    $deployerKey = $secrets["CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY"]
    $signerKey = $secrets["CHAIMIR_PAYMASTER_PRIVATE_KEY"]
    $deployerAddressResult = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $network, "--entrypoint", "cast", $foundryRef, "wallet", "address", "--private-key", $deployerKey)
    if ($deployerAddressResult.Code -ne 0 -or $deployerAddressResult.Text -notmatch '^0x[0-9a-fA-F]{40}$') { throw "平台 EVM 部署账户地址生成失败" }
    $deployerAddress = $deployerAddressResult.Text.Trim()
    $deployerBalance = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $network, "--entrypoint", "cast", $foundryRef, "balance", $deployerAddress, "--rpc-url", "http://${RpcHost}:8545")
    $deployerNonce = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $network, "--entrypoint", "cast", $foundryRef, "nonce", $deployerAddress, "--rpc-url", "http://${RpcHost}:8545")
    if ($deployerBalance.Code -ne 0 -or $deployerNonce.Code -ne 0 -or $deployerBalance.Text -notmatch '^[0-9]+$' -or $deployerNonce.Text -notmatch '^[0-9]+$') {
        throw "平台 EVM 部署账户链上预检失败: balance_exit=$($deployerBalance.Code) nonce_exit=$($deployerNonce.Code)"
    }
    $signer = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $network, "--entrypoint", "cast", $foundryRef, "wallet", "address", "--private-key", $signerKey)
    if ($signer.Code -ne 0 -or $signer.Text -notmatch '^0x[0-9a-fA-F]{40}$') { throw "paymaster 验证签名地址生成失败" }
    $signerAddress = $signer.Text.Trim()
    $entryBytecode = Get-ArtifactBytecode 'EntryPoint'
    $entryAddress = Deploy-ControlledContract $entryBytecode $deployerKey '官方 EntryPoint'
    if ($entryAddress -notmatch '^0x[0-9a-fA-F]{40}$') { throw "EntryPoint 部署结果缺少合法地址" }
    $paymasterBytecode = Get-ArtifactBytecode 'VerifyingPaymaster'
    $paymasterArguments = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $network, "--entrypoint", "cast", $foundryRef, "abi-encode", "constructor(address,address)", $entryAddress, $signerAddress)
    if ($paymasterArguments.Code -ne 0 -or $paymasterArguments.Text -notmatch '^0x[0-9a-fA-F]+$') { throw "VerifyingPaymaster 构造参数 ABI 编码失败: $($paymasterArguments.Text)" }
    $paymasterInitCode = $paymasterBytecode + $paymasterArguments.Text.Trim().Substring(2)
    $paymasterAddress = Deploy-ControlledContract $paymasterInitCode $deployerKey '官方 VerifyingPaymaster'
    if ($paymasterAddress -notmatch '^0x[0-9a-fA-F]{40}$') { throw "paymaster 部署结果缺少合法地址" }
    $factoryBytecode = Get-ArtifactBytecode 'SimpleAccountFactory'
    $factoryArguments = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $network, "--entrypoint", "cast", $foundryRef, "abi-encode", "constructor(address)", $entryAddress)
    if ($factoryArguments.Code -ne 0 -or $factoryArguments.Text -notmatch '^0x[0-9a-fA-F]+$') { throw "SimpleAccountFactory 构造参数 ABI 编码失败: $($factoryArguments.Text)" }
    $factoryInitCode = $factoryBytecode + $factoryArguments.Text.Trim().Substring(2)
    $factoryAddress = Deploy-ControlledContract $factoryInitCode $deployerKey '官方 SimpleAccountFactory'
    if ($factoryAddress -notmatch '^0x[0-9a-fA-F]{40}$') { throw "SimpleAccountFactory 部署结果缺少合法地址" }

    $entryCode = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $network, "--entrypoint", "cast", $foundryRef, "code", $entryAddress, "--rpc-url", "http://${RpcHost}:8545")
    $paymasterCode = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $network, "--entrypoint", "cast", $foundryRef, "code", $paymasterAddress, "--rpc-url", "http://${RpcHost}:8545")
    $factoryCode = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $network, "--entrypoint", "cast", $foundryRef, "code", $factoryAddress, "--rpc-url", "http://${RpcHost}:8545")
    if ($entryCode.Code -ne 0 -or $entryCode.Text -eq "0x" -or $paymasterCode.Code -ne 0 -or $paymasterCode.Text -eq "0x" -or $factoryCode.Code -ne 0 -or $factoryCode.Text -eq "0x") { throw "部署后的官方合约没有可执行字节码" }

    $chainIdText = $probe.Text.Trim()
    $chainId = if ($chainIdText -match '^0x[0-9a-fA-F]+$') { [Convert]::ToInt64($chainIdText.Substring(2), 16) } elseif ($chainIdText -match '^[0-9]+$') { [Convert]::ToInt64($chainIdText, 10) } else { throw "受控 EVM runtime 返回非法 chain id" }
    $material = [ordered]@{
        schema_version = 1
        protocol = "erc4337"
        version = "0.6.0"
        chain_id = $chainId
        entry_point = $entryAddress
        paymaster = $paymasterAddress
        simple_account_factory = $factoryAddress
        verifying_signer = $signerAddress
        source = [ordered]@{
            account_abstraction_archive = $resolvedAaArchive
            account_abstraction_sha256 = (Get-FileHash -LiteralPath $AccountAbstractionArchive -Algorithm SHA256).Hash.ToLowerInvariant()
            openzeppelin_archive = $resolvedOzArchive
            openzeppelin_sha256 = (Get-FileHash -LiteralPath $OpenZeppelinArchive -Algorithm SHA256).Hash.ToLowerInvariant()
            runtime_digest = $lock[$RuntimeImage]
        }
    }
    $parent = Split-Path -Parent $OutputPath
    New-Item -ItemType Directory -Path $parent -Force | Out-Null
    $materialJson = ($material | ConvertTo-Json -Depth 6) + [Environment]::NewLine
    [IO.File]::WriteAllText($OutputPath, $materialJson, [Text.UTF8Encoding]::new($false))
    Write-Output "受控 ERC-4337 材料已生成: $OutputPath"
} finally {
    if ($ownsRuntime) {
        Invoke-Docker @("rm", "-f", "-v", $runtime) | Out-Null
        Invoke-Docker @("network", "rm", $network) | Out-Null
        Invoke-Docker @("volume", "rm", $volume) | Out-Null
    }
    if (Test-Path -LiteralPath $workRoot) { Remove-Item -LiteralPath $workRoot -Recurse -Force }
}
