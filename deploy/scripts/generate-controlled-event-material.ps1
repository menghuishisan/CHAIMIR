# 本脚本在当前 CompositionSpec 的 EVM runtime 中部署平台受控事件合约并输出绑定材料。
param(
    [Parameter(Mandatory = $true)][string]$DigestLockPath,
    [Parameter(Mandatory = $true)][string]$ExistingNetwork,
    [Parameter(Mandatory = $true)][string]$OutputPath,
    [string]$RuntimeImage = "runtime/evm-foundry",
    [string]$RpcHost = "evm"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Import-Module (Join-Path $repoRoot "images\lib\ImageMetadata.psm1") -Force
$config = Read-ChaimirEnvFile -Path (Join-Path $repoRoot "deploy\config\chaimir.env") -RequiredKeys @(
    "SUPPLY_CHAIN_REGISTRY", "SUPPLY_CHAIN_REGISTRY_ENDPOINT", "SUPPLY_CHAIN_HARBOR_EXTERNAL_URL", "IMAGE_REGISTRY"
)
$registry = Get-ChaimirCanonicalRegistry -Config $config

function Invoke-Docker {
    param([string[]]$Arguments, [int]$TimeoutSeconds = 300)
    return Invoke-ChaimirDocker -Arguments $Arguments -TimeoutSeconds $TimeoutSeconds
}

function Invoke-ControlledRpc([string]$Method, [object[]]$Params) {
    $body = @{ jsonrpc = '2.0'; id = 1; method = $Method; params = @($Params) } | ConvertTo-Json -Depth 8 -Compress
    $bodyBase64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($body))
    $shell = "set -eu; printf '%s' '$bodyBase64' | base64 -d >/tmp/rpc.json; curl --connect-timeout 2 --max-time 5 -fsS -H 'content-type: application/json' --data-binary @/tmp/rpc.json http://${RpcHost}:8545"
    $arguments = @('run', '--rm', '--pull', 'never', '--network', $networkName, '--read-only', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges', '--tmpfs', '/tmp:rw,nosuid,nodev,noexec,size=1m', '--entrypoint', 'sh', $toolsRef, '-c', $shell)
    $response = Invoke-Docker -Arguments $arguments -TimeoutSeconds 15
    if ($response.Code -ne 0) { throw "受控事件 JSON-RPC $Method 请求失败: $($response.Text)" }
    try { $rpcResponse = $response.Text | ConvertFrom-Json } catch { throw "受控事件 JSON-RPC $Method 返回无效 JSON: $($response.Text)" }
    if ($rpcResponse.PSObject.Properties.Name -notcontains 'result') { throw "受控事件 JSON-RPC $Method 响应缺少 result 字段" }
    if ($rpcResponse.PSObject.Properties.Name -contains 'error' -and $null -ne $rpcResponse.error) { throw "受控事件 JSON-RPC $Method 返回错误: $($rpcResponse.error.message)" }
    return $rpcResponse.result
}

$lock = Read-ChaimirDigestLock -Path (Resolve-Path -LiteralPath $DigestLockPath).Path -Required
$digest = $lock[$RuntimeImage]
if ($digest -notmatch '^sha256:[0-9a-f]{64}$') { throw "事件材料生成缺少 runtime digest: $RuntimeImage" }
$runtimeRef = "$registry/$RuntimeImage@$digest"
$foundryDigest = $lock['runtime/evm-foundry']
if ($foundryDigest -notmatch '^sha256:[0-9a-f]{64}$') { throw "事件材料生成缺少受控 Foundry 工具 digest" }
$foundryRef = "$registry/runtime/evm-foundry@$foundryDigest"
$toolsDigest = $lock['base/chain-tools']
if ($toolsDigest -notmatch '^sha256:[0-9a-f]{64}$') { throw "事件材料生成缺少受控 chain-tools digest" }
$toolsRef = "$registry/base/chain-tools@$toolsDigest"
if ($RpcHost -notmatch '^[A-Za-z0-9][A-Za-z0-9.-]*$') { throw "JSON-RPC 主机名不合法: $RpcHost" }
$sourcePath = (Resolve-Path (Join-Path $repoRoot "deploy\scripts\assets\chaimir-transfer-token.sol")).Path
$sourceDir = Split-Path -Parent $sourcePath
$sourceName = Split-Path -Leaf $sourcePath
$networkCheck = Invoke-Docker @("network", "inspect", $ExistingNetwork)
if ($networkCheck.Code -ne 0) { throw "CompositionSpec runtime 网络不存在: $ExistingNetwork" }
$sourceHash = (Get-FileHash -LiteralPath $sourcePath -Algorithm SHA256).Hash.ToLowerInvariant()
$platformSecrets = Read-ChaimirEnvFile -Path (Join-Path $repoRoot "deploy\config\secret.env") -RequiredKeys @(
    "CHAIMIR_EVM_DEPLOYER_MNEMONIC", "CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY"
)
foreach ($name in @("CHAIMIR_EVM_DEPLOYER_MNEMONIC", "CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY")) {
    if ([string]$platformSecrets[$name] -match '^changeme' -or [string]::IsNullOrWhiteSpace([string]$platformSecrets[$name])) {
        throw "平台 Secret 源中的键无有效值: $name"
    }
}
$deployerKey = [string]$platformSecrets["CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY"]
$networkName = $ExistingNetwork
$compileShell = 'set -eu; solc-0.8.20 --bin --base-path /src -o /tmp/out --overwrite /src/{0} >/tmp/solc.log; code=$(tr -d ''[:space:]'' </tmp/out/ChaimirTransferToken.bin); test -n "$code"; printf ''%s\n'' "$code"' -f $sourceName
$compileArgs = @("run", "--rm", "--pull", "never", "--user", "0:0", "--network", $networkName, "--read-only", "--tmpfs", "/tmp:rw,exec,nosuid,mode=1777,size=512m", "-e", "HOME=/tmp", "--entrypoint", "sh", "-v", "${sourceDir}:/src:ro", $foundryRef, "-lc", $compileShell)
$compiled = Invoke-Docker $compileArgs
if ($compiled.Code -ne 0) { throw "事件合约在候选 runtime 工具链中编译失败: $($compiled.Text)" }
$compileLines = @($compiled.Text -split "`r?`n" | Where-Object { $_.Trim() -ne "" })
$bytecode = ($compileLines | Where-Object { $_ -match '^6080|^60[0-9a-fA-F]+' } | Select-Object -First 1).Trim()
if ($bytecode -notmatch '^[0-9a-fA-F]+$') { throw "事件合约编译结果缺少合法字节码" }
$rpcUrl = "http://${RpcHost}:8545"
$outputParent = Split-Path -Parent $OutputPath
New-Item -ItemType Directory -Path $outputParent -Force | Out-Null
$initCodePath = Join-Path $outputParent ("controlled-event-init-" + [Guid]::NewGuid().ToString("N") + ".hex")
[IO.File]::WriteAllText($initCodePath, "0x$bytecode", [Text.ASCIIEncoding]::new())
try {
    $sendShell = 'set -eu; cast send --async --json --rpc-url "{0}" --private-key "{1}" --create "$(cat /tmp/initcode)"' -f $rpcUrl, $deployerKey
    $send = Invoke-Docker @("run", "--rm", "--pull", "never", "--network", $networkName, "--mount", "type=bind,source=$initCodePath,target=/tmp/initcode,readonly", "--entrypoint", "sh", $foundryRef, "-lc", $sendShell)
    if ($send.Code -ne 0) { throw "事件合约异步交易提交失败: $($send.Text)" }
    $hashMatch = [regex]::Match($send.Text, '0x[0-9a-fA-F]{64}')
    if (-not $hashMatch.Success) { throw "事件合约异步交易响应缺少合法哈希: $($send.Text)" }
    $transactionHash = $hashMatch.Value
    $deployment = $null
    for ($attempt = 0; $attempt -lt 90; $attempt++) {
        $deployment = Invoke-ControlledRpc 'eth_getTransactionReceipt' @($transactionHash)
        if ($null -ne $deployment) { break }
        Start-Sleep -Seconds 1
    }
    if ($null -eq $deployment) { throw "事件合约交易在限定时间内没有可读收据: tx=$transactionHash" }
    if ([string]$deployment.status -ne '0x1') { throw "事件合约部署交易失败: tx=$transactionHash status=$($deployment.status)" }
    $address = [string]$deployment.contractAddress
    if ($address -notmatch '^0x[0-9a-fA-F]{40}$') { throw "事件合约收据缺少合法部署地址: tx=$transactionHash" }
    $deployed = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        $code = Invoke-ControlledRpc 'eth_getCode' @($address, 'latest')
        if ([string]$code -match '^0x[0-9a-fA-F]+$' -and [string]$code -ne '0x') { $deployed = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $deployed) { throw "事件合约收据成功但部署地址没有可执行字节码: tx=$transactionHash address=$address" }
} finally {
    if (Test-Path -LiteralPath $initCodePath) { Remove-Item -LiteralPath $initCodePath -Force }
}
$material = [ordered]@{
    schema_version = 1
    protocol = "evm-event"
    contract_address = $address
    event_name = "Transfer(address,address,uint256)"
    event_topic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
    runtime_digest = $digest
    source_sha256 = $sourceHash
}
$materialJson = ($material | ConvertTo-Json -Depth 4) + [Environment]::NewLine
[IO.File]::WriteAllText($OutputPath, $materialJson, [Text.UTF8Encoding]::new($false))
Write-Output "受控事件材料已生成: $OutputPath"
