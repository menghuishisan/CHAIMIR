# 本脚本基于既有受控 OP-Stack 材料刷新 Beacon finalized checkpoint,不联网生成临时依赖。
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$DigestLockPath,
    [Parameter(Mandatory = $true)][string]$SourceMaterialPath,
    [Parameter(Mandatory = $true)][string]$OutputPath,
    [Parameter(Mandatory = $true)][string]$GeneratorArchivePath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Import-Module (Join-Path $repoRoot "images\lib\ImageMetadata.psm1") -Force

function Invoke-Docker {
    param([string[]]$Arguments, [int]$TimeoutSeconds = 300)
    $tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("chaimir-checkpoint-docker-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = [Diagnostics.ProcessStartInfo]::new()
    $process.StartInfo.FileName = "docker.exe"
    $process.StartInfo.Arguments = (($Arguments | ForEach-Object {
        $value = [string]$_
        if ($value -notmatch '[\s"]') { $value } else { '"' + ($value.Replace('"', '\"')) + '"' }
    }) -join ' ')
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.CreateNoWindow = $true
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    $process.StartInfo.StandardOutputEncoding = [Text.Encoding]::UTF8
    $process.StartInfo.StandardErrorEncoding = [Text.Encoding]::UTF8
    try {
        if (-not $process.Start()) { throw "Docker CLI 启动失败" }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
            try {
                $process.Kill($true)
            } catch {
                Write-Warning ("Docker 超时后的进程树终止失败,继续尝试终止主进程: {0}" -f $_.Exception.Message)
                try {
                    $process.Kill()
                } catch {
                    Write-Warning ("Docker 超时后的主进程终止也失败: {0}" -f $_.Exception.Message)
                }
            }
            return [pscustomobject]@{ Code = 124; Text = "Docker CLI 超时（$TimeoutSeconds 秒）: $($Arguments -join ' ')" }
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        return [pscustomobject]@{ Code = $process.ExitCode; Text = ((@($stdout, $stderr) | Where-Object { $_ }) -join "`n").Trim() }
    } finally {
        $process.Dispose()
        if (Test-Path -LiteralPath $tempRoot) { [IO.Directory]::Delete($tempRoot, $true) }
    }
}

function Get-LockedRef([hashtable]$Lock, [string]$Registry, [string]$Image) {
    $digest = $Lock[$Image]
    if ($digest -notmatch '^sha256:[0-9a-f]{64}$') { throw "候选锁缺少合法 digest: $Image" }
    return "$Registry/$Image@$digest"
}

function Read-PlatformSecret([string]$Name) {
    $path = Join-Path $repoRoot "deploy\config\secret.env"
    $values = Read-ChaimirEnvFile -Path $path -RequiredKeys @($Name)
    $value = ([string]$values[$Name]).Trim()
    if ([string]::IsNullOrWhiteSpace($value) -or $value -match '^changeme') { throw "平台 Secret 源中的键无有效值: $Name" }
    return $value
}

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    [IO.File]::WriteAllText($Path, $Content, [Text.UTF8Encoding]::new($false))
}

function Assert-Hash([string]$Path, [string]$Expected, [string]$Label) {
    $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $Expected.ToLowerInvariant()) { throw "$Label SHA-256 不匹配: $actual" }
}

function Assert-MaterialFile([string]$Root, [string]$Name, $Manifest) {
    $path = Join-Path $Root $Name
    if (-not (Test-Path -LiteralPath $path -PathType Leaf) -or (Get-Item -LiteralPath $path).Length -le 0) { throw "受控 OP-Stack 材料缺少必需文件: $Name" }
    $expected = $Manifest.files.PSObject.Properties[$Name]
    if ($null -eq $expected) { throw "受控 OP-Stack 材料清单缺少文件哈希: $Name" }
    Assert-Hash $path ([string]$expected.Value) "受控材料 $Name"
}

$config = Read-ChaimirEnvFile -Path (Join-Path $repoRoot "deploy\config\chaimir.env") -RequiredKeys @(
    "SUPPLY_CHAIN_REGISTRY", "SUPPLY_CHAIN_REGISTRY_ENDPOINT", "SUPPLY_CHAIN_HARBOR_EXTERNAL_URL", "IMAGE_REGISTRY"
)
$registry = Get-ChaimirCanonicalRegistry -Config $config
$lock = Read-ChaimirDigestLock -Path (Resolve-Path -LiteralPath $DigestLockPath).Path -Required
$beaconRef = Get-LockedRef $lock $registry "infra/ethereum-beacon"
$gethRef = Get-LockedRef $lock $registry "runtime/op-geth"
$toolsRef = Get-LockedRef $lock $registry "base/chain-tools"
$goBuilderRef = Get-LockedRef $lock $registry "base/go-builder"
$foundryRef = Get-LockedRef $lock $registry "runtime/evm-foundry"
$jwt = Read-PlatformSecret "OP_GETH_JWT_SECRET"
$mnemonic = Read-PlatformSecret "CHAIMIR_EVM_DEPLOYER_MNEMONIC"
$deployerKey = Read-PlatformSecret "CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY"
$deployerAddressResult = Invoke-Docker @("run","--rm","--pull","never","--entrypoint","cast",$foundryRef,"wallet","address","--private-key",$deployerKey)
if ($deployerAddressResult.Code -ne 0 -or $deployerAddressResult.Text -notmatch '(?m)^0x[0-9a-fA-F]{40}$') { throw "平台统一 EVM 部署账户地址生成失败" }
$deployerAddress = $deployerAddressResult.Text.Trim().ToLowerInvariant()
$sourceRoot = (Resolve-Path -LiteralPath $SourceMaterialPath).Path
$outputAbsolute = [IO.Path]::GetFullPath((Join-Path (Get-Location).Path $OutputPath))
$generatorArchive = (Resolve-Path -LiteralPath $GeneratorArchivePath).Path
Assert-Hash $generatorArchive "71dd5464b667c433ee85aa2329d9ec0ef478573512f16186a0a4012fcbe49a4d" "eth-beacon-genesis v0.0.7"
if ($sourceRoot.TrimEnd('\') -eq $outputAbsolute.TrimEnd('\')) { throw "刷新输出目录不能覆盖源材料目录" }
if (Test-Path -LiteralPath $outputAbsolute) { throw "刷新输出目录已存在,为避免覆盖已有材料而拒绝: $outputAbsolute" }

$manifestPath = Join-Path $sourceRoot "material-manifest.json"
if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) { throw "受控 OP-Stack 材料缺少 material-manifest.json" }
$sourceManifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
$requiredFiles = @("config.yaml","genesis.json","l1-genesis.json","l2-genesis.json","rollup.json","deploy-config.json","l1-allocs.json","l1-deployments.json","l2-allocs.json","superchain-configs.zip","genesis.ssz","genesis-block.ssz","parsedConsensusGenesis.json","validator_names.yaml","deposit_contract.txt","deposit_contract_block.txt","bootstrap_nodes.txt","l1-execution-state.tar")
foreach ($file in $requiredFiles) { Assert-MaterialFile $sourceRoot $file $sourceManifest }
$validatorCount = [int]$sourceManifest.validator_count
if ($validatorCount -lt 1 -or $validatorCount -gt 16) { throw "受控材料 validator_count 必须位于 1-16" }

$workRoot = Join-Path $repoRoot (".tmp\backend-functional-test\evidence\controlled-op-stack-checkpoint-refresh-" + [Guid]::NewGuid().ToString("N"))
$materialRoot = Join-Path $workRoot "material"
$generatorRoot = Join-Path $workRoot "generator"
$validatorMnemonicPath = Join-Path $workRoot "validator-mnemonic"
$validatorPasswordPath = Join-Path $workRoot "validator-password"
$network = "chaimir-op-stack-checkpoint-$([Guid]::NewGuid().ToString('N').Substring(0, 12))"
$l1Volume = "$network-l1-state"
$beaconVolume = "$network-beacon-state"
$validatorVolume = "$network-validator-state"
$l1Container = "$network-l1"
$sourceContainer = "$network-source"
$beaconContainer = "$network-beacon"
$validatorContainer = "$network-validator"
$createdContainers = @()
$createdVolumes = @()
$networkCreated = $false
$outputCreated = $false
$completed = $false

New-Item -ItemType Directory -Path $materialRoot, $generatorRoot -Force | Out-Null
try {
    Copy-Item -LiteralPath (Join-Path $sourceRoot "genesis.json") -Destination (Join-Path $materialRoot "genesis.json")
    $config = Get-Content -Raw -LiteralPath (Join-Path $sourceRoot "config.yaml")
    $freshGenesisTime = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() + 30
    if ($config -notmatch '(?m)^MIN_GENESIS_TIME:\s*[0-9]+\s*$') { throw "受控 Beacon config 缺少 MIN_GENESIS_TIME" }
    $config = [regex]::Replace($config, '(?m)^MIN_GENESIS_TIME:\s*[0-9]+\s*$', "MIN_GENESIS_TIME: $freshGenesisTime")
    Write-Utf8NoBom (Join-Path $materialRoot "config.yaml") $config
    Write-Utf8NoBom (Join-Path $materialRoot "mnemonics.yaml") ("- mnemonic: `"$mnemonic`"`n  name: `"chaimir-controlled`"`n  start: 0`n  count: $validatorCount`n  wd_address: null`n  wd_prefix: 0x00`n  balance: 32000000000`n  status: 0`n")
    foreach ($file in @("deposit_contract.txt","deposit_contract_block.txt","bootstrap_nodes.txt")) { Copy-Item -LiteralPath (Join-Path $sourceRoot $file) -Destination (Join-Path $materialRoot $file) }

    $extract = Invoke-Docker -TimeoutSeconds 900 @("run","--rm","--pull","never","--user","0:0","--mount","type=bind,source=$generatorArchive,target=/in/archive,readonly","--mount","type=bind,source=$generatorRoot,target=/out","--entrypoint","/bin/sh",$goBuilderRef,"-lc","set -eu; tar -xzf /in/archive -C /out")
    if ($extract.Code -ne 0) { throw "固定 Beacon 生成器解压失败: $($extract.Text)" }
    $generatorBinary = @(Get-ChildItem -LiteralPath $generatorRoot -Recurse -File -Filter "eth-genesis-state-generator" | Select-Object -First 1)[0]
    if ($null -eq $generatorBinary) { throw "固定 Beacon 生成器归档缺少 eth-genesis-state-generator" }
    $generatorMode = Invoke-Docker -TimeoutSeconds 60 @("run","--rm","--pull","never","--user","0:0","--mount","type=bind,source=$($generatorBinary.FullName),target=/tools/eth-genesis-state-generator","--entrypoint","/bin/sh",$goBuilderRef,"-lc","set -eu; chmod 0755 /tools/eth-genesis-state-generator; test -x /tools/eth-genesis-state-generator")
    if ($generatorMode.Code -ne 0) { throw "固定 Beacon 生成器执行权限初始化失败: $($generatorMode.Text)" }
    $generate = Invoke-Docker -TimeoutSeconds 300 @("run","--rm","--pull","never","--user","0:0","--mount","type=bind,source=$($generatorBinary.FullName),target=/tools/eth-genesis-state-generator,readonly","--mount","type=bind,source=$materialRoot,target=/material","--entrypoint","/tools/eth-genesis-state-generator",$beaconRef,"beaconchain","--config","/material/config.yaml","--eth1-config","/material/genesis.json","--mnemonics","/material/mnemonics.yaml","--state-output","/material/genesis.ssz","--json-output","/material/parsedConsensusGenesis.json","--validators-mapping-output","/material/validator_names.yaml")
    if ($generate.Code -ne 0) { throw "刷新 Beacon 初始 state 失败: $($generate.Text)" }

    $networkResult = Invoke-Docker @("network","create","--internal",$network)
    if ($networkResult.Code -ne 0) { throw "创建 checkpoint 刷新网络失败: $($networkResult.Text)" }
    $networkCreated = $true
    foreach ($volume in @($l1Volume,$beaconVolume,$validatorVolume)) {
        $volumeResult = Invoke-Docker @("volume","create",$volume)
        if ($volumeResult.Code -ne 0) { throw "创建 checkpoint 刷新卷失败: $($volumeResult.Text)" }
        $createdVolumes += $volume
    }
    # 本轮刷新使用与新 Beacon genesis 同源的 L1 genesis 重新初始化,不能混入旧 checkpoint 的 execution head。
    $prepareL1 = Invoke-Docker @("run","--rm","--pull","never","--user","0:0","--mount","source=$l1Volume,target=/runtime-state","--mount","type=bind,source=$(Join-Path $sourceRoot 'l1-genesis.json'),target=/seed/l1-genesis.json,readonly","--entrypoint","sh",$toolsRef,"-lc","set -eu; mkdir -p /runtime-state/op-geth; cp /seed/l1-genesis.json /runtime-state/op-geth/genesis.json; chown -R 1000:1000 /runtime-state")
    if ($prepareL1.Code -ne 0) { throw "受控 L1 execution 状态注入失败: $($prepareL1.Text)" }
    $l1Start = Invoke-Docker @("run","-d","--pull","never","--name",$l1Container,"--network",$network,"--network-alias","l1exec","--mount","source=$l1Volume,target=/runtime-state","-e","OP_GETH_DATA_DIR=/runtime-state/op-geth","-e","OP_GETH_GENESIS_PATH=/runtime-state/op-geth/genesis.json","-e","OP_GETH_JWT_SECRET=$jwt","-e","OP_GETH_RPC_PORT=8545","-e","OP_GETH_ENGINE_PORT=8551",$gethRef)
    if ($l1Start.Code -ne 0) { throw "受控 L1 execution 启动失败: $($l1Start.Text)" }
    $createdContainers += $l1Container
    $l1Ready = Invoke-Docker -TimeoutSeconds 180 @("run","--rm","--pull","never","--network",$network,"--entrypoint","sh",$toolsRef,"-lc","set -eu; for n in `$(seq 1 180); do curl --fail --silent --show-error -H 'content-type: application/json' --data '{`"jsonrpc`":`"2.0`",`"id`":1,`"method`":`"eth_chainId`",`"params`":[]}' http://l1exec:8545/ | grep -q result && exit 0; sleep 1; done; exit 1")
    if ($l1Ready.Code -ne 0) { throw "受控 L1 execution readiness 失败: $($l1Ready.Text)" }

    $prepareBeacon = Invoke-Docker @("run","--rm","--pull","never","--user","0:0","--mount","source=$beaconVolume,target=/runtime-state","--mount","type=bind,source=$materialRoot,target=/seed,readonly","--entrypoint","sh",$toolsRef,"-lc","set -eu; mkdir -p /runtime-state/beacon-testnet; cp /seed/config.yaml /seed/genesis.ssz /seed/deposit_contract.txt /seed/deposit_contract_block.txt /seed/bootstrap_nodes.txt /runtime-state/beacon-testnet/; chown -R 1000:1000 /runtime-state")
    if ($prepareBeacon.Code -ne 0) { throw "刷新 Beacon 材料注入失败: $($prepareBeacon.Text)" }
    # 刷新阶段先按正式生成器从本轮受控 genesis state 启动, finalized 文件只在真实运行后导出。
    $stateSource = Invoke-Docker -TimeoutSeconds 60 @("run","-d","--pull","never","--name",$sourceContainer,"--network",$network,"--network-alias","source","--mount","type=bind,source=$materialRoot,target=/material,readonly","--entrypoint","sh",$toolsRef,"-lc","set -eu; size=`$(wc -c < /material/genesis.ssz); for n in `$(seq 1 30); do { printf 'HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %s\r\nConnection: close\r\n\r\n' `"`$size`"; cat /material/genesis.ssz; } | timeout 30 nc -l -p 8000 || exit `$?; done; exit 1")
    if ($stateSource.Code -ne 0) { throw "受控 Beacon genesis state 服务启动失败: $($stateSource.Text)" }
    $createdContainers += $sourceContainer
    $beaconStart = Invoke-Docker @("run","-d","--pull","never","--name",$beaconContainer,"--network",$network,"--network-alias","beacon","--mount","source=$beaconVolume,target=/runtime-state","--mount","type=bind,source=$materialRoot,target=/material,readonly","--entrypoint","lighthouse",$beaconRef,"beacon_node","--testnet-dir","/material","--datadir","/runtime-state/lighthouse","--execution-endpoint","http://l1exec:8551","--execution-jwt-secret-key",$jwt,"--genesis-state-url","http://source:8000/state","--http","--http-address","0.0.0.0","--http-port","5052")
    if ($beaconStart.Code -ne 0) { throw "Beacon 刷新实例启动失败: $($beaconStart.Text)" }
    $createdContainers += $beaconContainer
    $beaconReady = Invoke-Docker -TimeoutSeconds 180 @("run","--rm","--pull","never","--network",$network,"--entrypoint","sh",$toolsRef,"-lc","set -eu; for n in `$(seq 1 180); do curl --fail --silent --show-error http://beacon:5052/eth/v1/node/health >/dev/null && exit 0; sleep 1; done; exit 1")
    if ($beaconReady.Code -ne 0) { throw "刷新 Beacon HTTP readiness 失败: $($beaconReady.Text)`n$((Invoke-Docker @('logs','--tail','120',$beaconContainer)).Text)" }

    Set-Content -LiteralPath $validatorMnemonicPath -Value $mnemonic -NoNewline -Encoding ASCII
    $validatorPassword = [Guid]::NewGuid().ToString("N") + [Guid]::NewGuid().ToString("N")
    Set-Content -LiteralPath $validatorPasswordPath -Value $validatorPassword -NoNewline -Encoding ASCII
    $recover = Invoke-Docker @("run","--rm","--pull","never","--user","1000:1000","--mount","source=$validatorVolume,target=/runtime-state","--mount","type=bind,source=$materialRoot,target=/input/material,readonly","--mount","type=bind,source=$validatorMnemonicPath,target=/input/validator-mnemonic,readonly","--mount","type=bind,source=$validatorPasswordPath,target=/input/validator-password,readonly","--entrypoint","sh",$beaconRef,"-lc","set -eu; cat /input/validator-password /input/validator-password | lighthouse account_manager validator recover --testnet-dir /input/material --validator-dir /runtime-state/validators --secrets-dir /runtime-state/secrets --first-index 0 --count $validatorCount --mnemonic-path /input/validator-mnemonic --stdin-inputs")
    if ($recover.Code -ne 0) { throw "刷新 validator 密钥恢复失败: $($recover.Text)" }
    $validatorStart = Invoke-Docker @("run","-d","--pull","never","--user","1000:1000","--name",$validatorContainer,"--network",$network,"--network-alias","validator","--mount","source=$validatorVolume,target=/runtime-state","--mount","type=bind,source=$materialRoot,target=/material,readonly","--entrypoint","lighthouse",$beaconRef,"validator_client","--testnet-dir","/material","--validators-dir","/runtime-state/validators","--secrets-dir","/runtime-state/secrets","--beacon-nodes","http://beacon:5052","--suggested-fee-recipient",$deployerAddress,"--init-slashing-protection","--disable-log-timestamp")
    if ($validatorStart.Code -ne 0) { throw "刷新 validator client 启动失败: $($validatorStart.Text)" }
    $createdContainers += $validatorContainer
    $finalized = Invoke-Docker -TimeoutSeconds 660 @("run","--rm","--pull","never","--network",$network,"--entrypoint","sh",$toolsRef,"-lc","set -eu; for n in `$(seq 1 600); do slot=`$(curl --fail --silent --show-error http://beacon:5052/eth/v1/beacon/headers/finalized | jq -r .data.header.message.slot); if test x`$slot != x -a x`$slot != xnull -a x`$slot != x0 -a `$slot -ge 32; then printf %s `$slot; exit 0; fi; sleep 1; done; exit 1")
    if ($finalized.Code -ne 0) { throw "刷新 Beacon finalized checkpoint 在 600 秒内未形成: $($finalized.Text)`n$((Invoke-Docker @('logs','--tail','160',$validatorContainer)).Text)`n$((Invoke-Docker @('logs','--tail','160',$beaconContainer)).Text)" }
    $finalizedSlot = [UInt64]$finalized.Text.Trim()
    $export = Invoke-Docker -TimeoutSeconds 120 @("run","--rm","--pull","never","--user","0:0","--network",$network,"--mount","type=bind,source=$materialRoot,target=/material","--entrypoint","sh",$toolsRef,"-lc","set -eu; curl --fail --silent --show-error -H Accept:application/octet-stream -o /material/genesis.ssz http://beacon:5052/eth/v2/debug/beacon/states/finalized; curl --fail --silent --show-error -H Accept:application/octet-stream -o /material/genesis-block.ssz http://beacon:5052/eth/v2/beacon/blocks/finalized; test -s /material/genesis.ssz; test -s /material/genesis-block.ssz")
    if ($export.Code -ne 0) { throw "刷新 Beacon finalized 文件导出失败: $($export.Text)" }

    foreach ($container in @($validatorContainer,$beaconContainer,$l1Container)) {
        $stop = Invoke-Docker -TimeoutSeconds 45 @("stop","--time","30",$container)
        if ($stop.Code -ne 0) { throw "checkpoint 生成容器停止失败: $container $($stop.Text)" }
    }
    $executionStateExport = Invoke-Docker -TimeoutSeconds 900 @("run","--rm","--pull","never","--user","0:0","--mount","source=$l1Volume,target=/runtime-state,readonly","--mount","type=bind,source=$materialRoot,target=/out","--entrypoint","/bin/sh",$toolsRef,"-lc","set -eu; tar -cf /out/l1-execution-state.tar -C /runtime-state op-geth/geth op-geth/chaimir-genesis-initialized; test -s /out/l1-execution-state.tar")
    if ($executionStateExport.Code -ne 0) { throw "checkpoint 对应的 L1 execution 状态导出失败: $($executionStateExport.Text)" }
    $executionStateVerify = Invoke-Docker @("run","--rm","--pull","never","--mount","type=bind,source=$materialRoot,target=/material,readonly","--entrypoint","/bin/sh",$toolsRef,"-lc","set -eu; tar -tf /material/l1-execution-state.tar | grep -Eq '^op-geth/geth(/|$)'; tar -tf /material/l1-execution-state.tar | grep -Fxq 'op-geth/chaimir-genesis-initialized'")
    if ($executionStateVerify.Code -ne 0) { throw "checkpoint 对应的 L1 execution 归档结构无效: $($executionStateVerify.Text)" }

    New-Item -ItemType Directory -Path $outputAbsolute -Force | Out-Null
    $outputCreated = $true
    Copy-Item -Path (Join-Path $sourceRoot "*") -Destination $outputAbsolute -Recurse -Force
    Copy-Item -LiteralPath (Join-Path $materialRoot "config.yaml") -Destination (Join-Path $outputAbsolute "config.yaml") -Force
    Copy-Item -LiteralPath (Join-Path $materialRoot "genesis.ssz") -Destination (Join-Path $outputAbsolute "genesis.ssz") -Force
    Copy-Item -LiteralPath (Join-Path $materialRoot "genesis-block.ssz") -Destination (Join-Path $outputAbsolute "genesis-block.ssz") -Force
    Copy-Item -LiteralPath (Join-Path $materialRoot "parsedConsensusGenesis.json") -Destination (Join-Path $outputAbsolute "parsedConsensusGenesis.json") -Force
    Copy-Item -LiteralPath (Join-Path $materialRoot "validator_names.yaml") -Destination (Join-Path $outputAbsolute "validator_names.yaml") -Force
    Copy-Item -LiteralPath (Join-Path $materialRoot "l1-execution-state.tar") -Destination (Join-Path $outputAbsolute "l1-execution-state.tar") -Force
    $manifest = $sourceManifest
    if ($null -eq $manifest.checkpoint.PSObject.Properties['generated_at_unix']) { $manifest.checkpoint | Add-Member -NotePropertyName generated_at_unix -NotePropertyValue 0 }
    $manifest.checkpoint.generated_at_unix = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    $manifest.checkpoint.slot = $finalizedSlot
    $manifest.files = [ordered]@{}
    foreach ($file in Get-ChildItem -LiteralPath $outputAbsolute -File) {
        if ($file.Name -ne "material-manifest.json") { $manifest.files[$file.Name] = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant() }
    }
    $manifestJson = $manifest | ConvertTo-Json -Depth 12
    [IO.File]::WriteAllText((Join-Path $outputAbsolute "material-manifest.json"), $manifestJson + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
    Write-Output "受控 OP-Stack finalized checkpoint 已刷新: $outputAbsolute slot=$finalizedSlot generated_at=$($manifest.checkpoint.generated_at_unix)"
    $completed = $true
} finally {
    foreach ($container in $createdContainers) { [void](Invoke-Docker @("rm","-f","-v",$container)) }
    foreach ($volume in $createdVolumes) { [void](Invoke-Docker @("volume","rm",$volume)) }
    if ($networkCreated) { [void](Invoke-Docker @("network","rm",$network)) }
    if (-not $completed -and $outputCreated -and (Test-Path -LiteralPath $outputAbsolute)) { [IO.Directory]::Delete($outputAbsolute, $true) }
    if (Test-Path -LiteralPath $workRoot) { [IO.Directory]::Delete($workRoot, $true) }
}
