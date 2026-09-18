# 本脚本使用项目锁定的官方 Optimism、Beacon 和构建工具生成完整受控材料归档。
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$DigestLockPath,
    [Parameter(Mandatory = $true)][string]$OutputPath,
    [Parameter(Mandatory = $true)][string]$GeneratorArchivePath,
    [Parameter(Mandatory = $true)][string]$BeaconSourceArchivePath,
    [Parameter(Mandatory = $true)][string]$OptimismSourceArchivePath,
    [Parameter(Mandatory = $true)][string]$SuperchainRegistryArchivePath,
    [Parameter(Mandatory = $true)][string]$SuperchainConfigsArchivePath,
    [string]$FoundryBuildSeedPath = "",
    [Parameter(Mandatory = $true)][string]$DependencyArchiveRootPath,
    [string]$CacheVolumePath = "chaimir-composition-go-mod-cache",
    [int]$ValidatorCount = 2
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Import-Module (Join-Path $repoRoot "images\lib\ImageMetadata.psm1") -Force

$generatorUrl = "https://github.com/ethpandaops/eth-beacon-genesis/releases/download/v0.0.7/eth-beacon-genesis_0.0.7_linux_amd64.tar.gz"
$generatorSha256 = "71dd5464b667c433ee85aa2329d9ec0ef478573512f16186a0a4012fcbe49a4d"
$beaconSourceUrl = "https://github.com/ethpandaops/ethereum-genesis-generator/archive/0bd028e7217307593f1bc441607d45d26ca62d19.tar.gz"
$beaconSourceSha256 = "810820d2608f7fd6b55ffd0ecfef6e8b23302f9fe2fbb05e210b729f610ce229"
$optimismSourceUrl = "https://github.com/ethereum-optimism/optimism/archive/da197e45ed44b9fca258b3b0d0709e8dfca1c7cd.tar.gz"
$optimismSourceSha256 = "0ff5ab74e76b35f0225c6e417ec9fae1025fff46e104a77341593919ae4d17de"
$registryUrl = "https://github.com/ethereum-optimism/superchain-registry/archive/d84b53b7ee008f863c1d3bbb49e06f713f713978.tar.gz"
$registrySha256 = "08600ee12e2d8933275813e4ef74e30929f95c249f6bce714c9c337527c20d4c"
$superchainConfigsSha256 = "91e5a7bf4d6f9a5fb555ff6747075e87f6e4d12d053aced5127d19b297543ae6"
$dependencySpecs = @(
    [pscustomobject]@{ Name = "forge-std"; Repository = "foundry-rs/forge-std"; Commit = "6853b9ec7df5dc0c213b05ae67785ad4f4baa0ea"; Sha256 = "783aecfe046ced756f6b46d007502cc647f2db3fda83e361cf68c454cfccee69" }
    [pscustomobject]@{ Name = "kontrol-cheatcodes"; Repository = "runtimeverification/kontrol-cheatcodes"; Commit = "2c48ae1ab44228c199dca29414c0b4b18a3434e6"; Sha256 = "bba725e1018f1fd52caefe949a22e12853c84e94a2bf7e34bc69a05f52d565f1" }
    [pscustomobject]@{ Name = "lib-keccak"; Repository = "ethereum-optimism/lib-keccak"; Commit = "3b1e7bbb4cc23e9228097cfebe42aedaf3b8f2b9"; Sha256 = "f29503c105f5eedf650e26c86fb33e57bca112ec9e4b91ebd8b1128b4a02e5bf" }
    [pscustomobject]@{ Name = "openzeppelin-contracts"; Repository = "OpenZeppelin/openzeppelin-contracts"; Commit = "ecd2ca2cd7cac116f7a37d0e474bbb3d7d5e1c4d"; Sha256 = "628b3ffeedf0a74d857fa4ce30b277c4918684f717d9df82a4a250f2745b2543" }
    [pscustomobject]@{ Name = "openzeppelin-contracts-upgradeable"; Repository = "OpenZeppelin/openzeppelin-contracts-upgradeable"; Commit = "0a2cb9a445c365870ed7a8ab461b12acf3e27d63"; Sha256 = "220f0c066fda5618e1d16590d2619f48bbaf2202f238957734ff381ec8051507" }
    [pscustomobject]@{ Name = "openzeppelin-contracts-v5"; Repository = "OpenZeppelin/openzeppelin-contracts"; Commit = "dbb6104ce834628e473d2173bbc9d47f81a9eec3"; Sha256 = "4a9c091852052f4319173629e0bf0b5cdf7a39d70e5e2298a7a6a149eff9f385" }
    [pscustomobject]@{ Name = "safe-contracts"; Repository = "safe-global/safe-contracts"; Commit = "bf943f80fec5ac647159d26161446ac5d716a294"; Sha256 = "3e8970c21af00edee1f0f9667251cd6468b6072ab360d16bbbb106d1011281ee" }
    [pscustomobject]@{ Name = "solady"; Repository = "Vectorized/solady"; Commit = "502cc1ea718e6fa73b380635ee0868b0740595f0"; Sha256 = "5ae76fff6d136ef3a878056e32ef860a21c5fb6de692150991442343ea915de9" }
    [pscustomobject]@{ Name = "solmate"; Repository = "transmissions11/solmate"; Commit = "8f9b23f8838670afda0fd8983f2c41e8037ae6bc"; Sha256 = "97038b0f62ecbcc37440261a5eddb990d80480f4538b85967ea471c5954709b2" }
)

function ConvertTo-ProcessArgument([string]$Value) {
    if ($null -eq $Value -or $Value.Length -eq 0) { return '""' }
    if ($Value -notmatch '[\s"]') { return $Value }
    $builder = [Text.StringBuilder]::new()
    [void]$builder.Append('"')
    $slashes = 0
    foreach ($character in $Value.ToCharArray()) {
        if ($character -eq '\') {
            $slashes++
            continue
        }
        if ($character -eq '"') {
            if ($slashes -gt 0) { [void]$builder.Append(('\' * ($slashes * 2)) -join '') }
            [void]$builder.Append('\').Append('"')
            $slashes = 0
            continue
        }
        if ($slashes -gt 0) { [void]$builder.Append(('\' * $slashes) -join '') }
        [void]$builder.Append($character)
        $slashes = 0
    }
    if ($slashes -gt 0) { [void]$builder.Append(('\' * ($slashes * 2)) -join '') }
    [void]$builder.Append('"')
    return $builder.ToString()
}

function Invoke-Docker {
    param(
        [string[]]$Arguments,
        [int]$TimeoutSeconds = 300
    )
    $tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("chaimir-docker-call-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null
    $stdoutPath = Join-Path $tempRoot "stdout.txt"
    $stderrPath = Join-Path $tempRoot "stderr.txt"
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = [Diagnostics.ProcessStartInfo]::new()
    $process.StartInfo.FileName = "docker.exe"
    $process.StartInfo.Arguments = (($Arguments | ForEach-Object { ConvertTo-ProcessArgument ([string]$_) }) -join " ")
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
            $stdout = $stdoutTask.GetAwaiter().GetResult()
            $stderr = $stderrTask.GetAwaiter().GetResult()
            return [pscustomobject]@{ Code = 124; Text = ("Docker CLI 超时（{0} 秒）: {1}`n{2}`n{3}" -f $TimeoutSeconds, ($Arguments -join " "), $stdout, $stderr).Trim() }
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        return [pscustomobject]@{ Code = $process.ExitCode; Text = ((@($stdout, $stderr) | Where-Object { $_ }) -join "`n").Trim() }
    } finally {
        $process.Dispose()
        if (Test-Path -LiteralPath $tempRoot) { Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue }
    }
}

function Assert-SHA256([string]$Path, [string]$Expected, [string]$Label) {
    $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $Expected.ToLowerInvariant()) { throw "$Label SHA-256 不匹配: $actual" }
}

function Get-Or-Download([string]$Path, [string]$Url, [string]$Expected, [string]$Label) {
    if ([string]::IsNullOrWhiteSpace($Path)) {
        throw "$Label 必须由项目受控材料显式提供,禁止运行时下载公网依赖"
    }
    $Path = (Resolve-Path -LiteralPath $Path).Path
    Assert-SHA256 $Path $Expected $Label
    return $Path
}

function Get-Or-DownloadTo([string]$Path, [string]$Url, [string]$Expected, [string]$Label) {
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "$Label 必须存在于项目受控依赖归档目录,禁止运行时下载公网依赖: $Path"
    }
    $Path = (Resolve-Path -LiteralPath $Path).Path
    Assert-SHA256 $Path $Expected $Label
    return $Path
}

function Get-LockedImageRef([hashtable]$Lock, [string]$Registry, [string]$Image) {
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

function Assert-File([string]$Root, [string]$Name) {
    $path = Join-Path $Root $Name
    if (-not (Test-Path -LiteralPath $path -PathType Leaf) -or (Get-Item -LiteralPath $path).Length -le 0) { throw "受控 OP-Stack 材料缺少必需文件: $Name" }
}

function Write-Stage([string]$Message) {
    Write-Output ("[op-stack-material] " + $Message)
}

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    [IO.File]::WriteAllText($Path, $Content, [Text.UTF8Encoding]::new($false))
}

function Expand-ArchiveInBuilder([string]$Archive, [string]$Destination, [string]$Label, [string]$BuilderRef) {
    $result = Invoke-Docker -TimeoutSeconds 900 @("run","--rm","--pull","never","--user","0:0","--mount","type=bind,source=$Archive,target=/in/archive,readonly","--mount","type=bind,source=$Destination,target=/out","--entrypoint","bash",$BuilderRef,"-lc","set -eu; tar -xzf /in/archive -C /out")
    if ($result.Code -ne 0) { throw "$Label 解压失败: $($result.Text)" }
}

function Expand-DependencyInBuilder([string]$Archive, [string]$Destination, [string]$Label, [string]$BuilderRef) {
    New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    $result = Invoke-Docker -TimeoutSeconds 900 @("run","--rm","--pull","never","--user","0:0","--mount","type=bind,source=$Archive,target=/in/archive,readonly","--mount","type=bind,source=$Destination,target=/out","--entrypoint","bash",$BuilderRef,"-lc","set -eu; tar -xzf /in/archive --strip-components=1 -C /out")
    if ($result.Code -ne 0) { throw "$Label 依赖解压失败: $($result.Text)" }
}

$config = Read-ChaimirEnvFile -Path (Join-Path $repoRoot "deploy\config\chaimir.env") -RequiredKeys @(
    "SUPPLY_CHAIN_REGISTRY", "SUPPLY_CHAIN_REGISTRY_ENDPOINT", "SUPPLY_CHAIN_HARBOR_EXTERNAL_URL", "IMAGE_REGISTRY"
)
$registry = Get-ChaimirCanonicalRegistry -Config $config
$lock = Read-ChaimirDigestLock -Path (Resolve-Path -LiteralPath $DigestLockPath).Path -Required
$beaconRef = Get-LockedImageRef $lock $registry "infra/ethereum-beacon"
$gethRef = Get-LockedImageRef $lock $registry "runtime/op-geth"
$opNodeRef = Get-LockedImageRef $lock $registry "infra/op-stack"
$toolsRef = Get-LockedImageRef $lock $registry "base/chain-tools"
$goBuilderRef = Get-LockedImageRef $lock $registry "base/go-builder"
$foundryRef = Get-LockedImageRef $lock $registry "runtime/evm-foundry"
$jwt = Read-PlatformSecret "OP_GETH_JWT_SECRET"
$deployerKey = Read-PlatformSecret "CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY"
$deployerAddressResult = Invoke-Docker @("run","--rm","--pull","never","--entrypoint","cast",$foundryRef,"wallet","address","--private-key",$deployerKey)
if ($deployerAddressResult.Code -ne 0 -or $deployerAddressResult.Text -notmatch '(?m)^0x[0-9a-fA-F]{40}$') { throw "平台统一 EVM 部署账户地址生成失败" }
$deployerAddress = $deployerAddressResult.Text.Trim().ToLowerInvariant()
if ($ValidatorCount -lt 1 -or $ValidatorCount -gt 16) { throw "ValidatorCount 必须位于 1-16" }

$workRoot = Join-Path $repoRoot (".tmp\backend-functional-test\evidence\controlled-op-stack-generation-" + [Guid]::NewGuid().ToString("N"))
$materialRoot = Join-Path $workRoot "material"
$archiveRoot = Join-Path $workRoot "archive"
$generatorRoot = Join-Path $workRoot "beacon-generator"
$beaconSourceRoot = Join-Path $workRoot "beacon-source"
$optimismSourceRoot = Join-Path $workRoot "optimism-source"
$registryRoot = Join-Path $workRoot "registry-source"
$dependencyArchiveRoot = Join-Path $workRoot "dependency-archives"
$cacheVolumeOwned = $false
$cacheVolume = $CacheVolumePath.Trim()
if ([string]::IsNullOrWhiteSpace($cacheVolume)) { throw "必须使用项目统一的 Go 模块缓存卷,禁止创建临时缓存卷" }
$sourceVolume = "chaimir-op-stack-material-source-$([Guid]::NewGuid().ToString('N').Substring(0, 12))"
$network = "chaimir-op-stack-material-$([Guid]::NewGuid().ToString('N').Substring(0, 12))"
$gethVolume = "$network-geth-state"
$beaconVolume = "$network-beacon-state"
$validatorVolume = "$network-validator-state"
$gethContainer = "$network-geth"
$sourceContainer = "$network-source"
$beaconContainer = "$network-beacon"
$validatorContainer = "$network-validator"
$foundryContainer = "$network-foundry"
$validatorMnemonicPath = Join-Path $workRoot "validator-mnemonic"
$validatorPasswordPath = Join-Path $workRoot "validator-password"
$createdNetwork = $false
$createdVolumes = @()
$createdContainers = @()
$outputAbsolute = [IO.Path]::GetFullPath((Join-Path (Get-Location).Path $OutputPath))

New-Item -ItemType Directory -Path $materialRoot, $archiveRoot, $generatorRoot, $beaconSourceRoot, $optimismSourceRoot, $registryRoot, $dependencyArchiveRoot -Force | Out-Null
try {
    Write-Stage "读取固定源码和工具归档"
    $generatorArchive = Get-Or-Download $GeneratorArchivePath $generatorUrl $generatorSha256 "eth-beacon-genesis v0.0.7"
    $beaconSourceArchive = Get-Or-Download $BeaconSourceArchivePath $beaconSourceUrl $beaconSourceSha256 "ethereum-genesis-generator commit 0bd028e"
    $optimismSourceArchive = Get-Or-Download $OptimismSourceArchivePath $optimismSourceUrl $optimismSourceSha256 "Optimism commit da197e45"
    $registryArchive = Get-Or-Download $SuperchainRegistryArchivePath $registryUrl $registrySha256 "superchain-registry commit d84b53b"
    $superchainConfigsArchive = Get-Or-Download $SuperchainConfigsArchivePath "file:///superchain-configs.zip" $superchainConfigsSha256 "固定 superchain-configs.zip"
    $dependencyArchives = @{}
    $dependencyCacheRoot = $null
    if (-not [string]::IsNullOrWhiteSpace($DependencyArchiveRootPath)) {
        $dependencyCacheRoot = (Resolve-Path -LiteralPath $DependencyArchiveRootPath).Path
        if (-not (Test-Path -LiteralPath $dependencyCacheRoot -PathType Container)) {
            throw "固定 OP-Stack 依赖归档缓存必须是目录: $DependencyArchiveRootPath"
        }
    }
    foreach ($dependency in $dependencySpecs) {
        $dependencyUrl = "https://codeload.github.com/$($dependency.Repository)/tar.gz/$($dependency.Commit)"
        $dependencyPathRoot = if ($null -ne $dependencyCacheRoot) { $dependencyCacheRoot } else { $dependencyArchiveRoot }
        $dependencyPath = Join-Path $dependencyPathRoot ("{0}-{1}.tar.gz" -f $dependency.Name, $dependency.Commit)
        $dependencyArchives[$dependency.Name] = Get-Or-DownloadTo $dependencyPath $dependencyUrl $dependency.Sha256 ("$($dependency.Name) commit $($dependency.Commit.Substring(0, 8))")
    }

    Write-Stage "使用固定 go-builder 解压官方归档"
    Expand-ArchiveInBuilder $generatorArchive $generatorRoot "eth-beacon-genesis v0.0.7" $goBuilderRef
    Expand-ArchiveInBuilder $beaconSourceArchive $beaconSourceRoot "ethereum-genesis-generator" $goBuilderRef
    Expand-ArchiveInBuilder $optimismSourceArchive $optimismSourceRoot "Optimism 源码" $goBuilderRef
    Expand-ArchiveInBuilder $registryArchive $registryRoot "superchain-registry" $goBuilderRef
    $generatorBinary = Join-Path $generatorRoot "eth-genesis-state-generator"
    if (-not (Test-Path -LiteralPath $generatorBinary -PathType Leaf)) { throw "固定共识生成器归档缺少 eth-genesis-state-generator" }
    $generatorMode = Invoke-Docker -TimeoutSeconds 60 @("run","--rm","--pull","never","--user","0:0","--mount","type=bind,source=$generatorBinary,target=/tools/eth-genesis-state-generator","--entrypoint","bash",$goBuilderRef,"-lc","set -eu; chmod 0755 /tools/eth-genesis-state-generator; test -x /tools/eth-genesis-state-generator")
    if ($generatorMode.Code -ne 0) { throw "固定共识生成器执行权限初始化失败: $($generatorMode.Text)" }
    $beaconSourceDir = @(Get-ChildItem -LiteralPath $beaconSourceRoot -Directory | Select-Object -First 1)[0]
    $optimismSourceDir = @(Get-ChildItem -LiteralPath $optimismSourceRoot -Directory | Select-Object -First 1)[0]
    $registrySourceDir = @(Get-ChildItem -LiteralPath $registryRoot -Directory | Select-Object -First 1)[0]
    if ($null -eq $beaconSourceDir -or $null -eq $optimismSourceDir -or $null -eq $registrySourceDir) { throw "固定官方源码归档目录缺失" }

    Write-Stage "注入固定版本的官方 Foundry 依赖"
    $contractsLibRoot = Join-Path $optimismSourceDir.FullName "packages\contracts-bedrock\lib"
    foreach ($dependency in $dependencySpecs) {
        $dependencyDestination = Join-Path $contractsLibRoot $dependency.Name
        Expand-DependencyInBuilder $dependencyArchives[$dependency.Name] $dependencyDestination $dependency.Name $goBuilderRef
    }
    # 官方仓库同时存在 solady/src/... 和 @solady/... 两种导入写法,补齐官方依赖自身要求的根映射。
    $foundryTomlPath = Join-Path $optimismSourceDir.FullName "packages\contracts-bedrock\foundry.toml"
    $foundryToml = Get-Content -LiteralPath $foundryTomlPath -Raw
    if ($foundryToml -notmatch "'solady/=") {
        $foundryToml = $foundryToml.Replace("  '@solady/=lib/solady/src',", "  'solady/=lib/solady',`n  '@solady/=lib/solady/src',")
        Write-Utf8NoBom $foundryTomlPath $foundryToml
    }
    if (-not [string]::IsNullOrWhiteSpace($FoundryBuildSeedPath)) {
        $seedContractsRoot = Join-Path (Resolve-Path -LiteralPath $FoundryBuildSeedPath).Path "packages\contracts-bedrock"
        $seedCache = Join-Path $seedContractsRoot "cache"
        $seedArtifacts = Join-Path $seedContractsRoot "forge-artifacts"
        if (-not (Test-Path -LiteralPath (Join-Path $seedCache "solidity-files-cache.json") -PathType Leaf) -or -not (Test-Path -LiteralPath $seedArtifacts -PathType Container)) {
            throw "Foundry 编译缓存种子缺少 cache/solidity-files-cache.json 或 forge-artifacts: $FoundryBuildSeedPath"
        }
        Copy-Item -LiteralPath $seedCache -Destination (Join-Path $optimismSourceDir.FullName "packages\contracts-bedrock") -Recurse -Force
        Copy-Item -LiteralPath $seedArtifacts -Destination (Join-Path $optimismSourceDir.FullName "packages\contracts-bedrock") -Recurse -Force
        Write-Stage "复用同一官方源码和依赖的已审计 Foundry 缓存种子,仍由新候选执行离线校验"
    }

    # superchain 配置归档必须来自同一固定 registry commit,不能在生成期间联网或动态安装工具。
    Write-Stage "注入固定 superchain 配置归档"
    $superchainConfigDestination = Join-Path $optimismSourceDir.FullName "op-core\superchain\superchain-configs.zip"
    if (-not (Test-Path -LiteralPath (Split-Path -Parent $superchainConfigDestination) -PathType Container)) { throw "Optimism 源码缺少 op-core/superchain 目录" }
    Copy-Item -LiteralPath $superchainConfigsArchive -Destination $superchainConfigDestination -Force
    Assert-SHA256 $superchainConfigDestination $superchainConfigsSha256 "Optimism superchain-configs.zip"

    Write-Stage "使用固定 go-builder 导出官方部署材料"
    $exporterDir = Join-Path $optimismSourceDir.FullName "cmd\chaimir-material-export"
    New-Item -ItemType Directory -Path $exporterDir -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $repoRoot "deploy\scripts\op-stack-material-export\main.go") -Destination (Join-Path $exporterDir "main.go") -Force
    $export = Join-Path $workRoot "export"
    New-Item -ItemType Directory -Path $export -Force | Out-Null
    $cacheExisting = Invoke-Docker @("volume","inspect",$cacheVolume)
    if ($cacheExisting.Code -ne 0) { throw "项目统一的 Go 构建缓存卷不存在: $cacheVolume" }
    $cachePrepare = Invoke-Docker @("run","--rm","--pull","never","--user","0:0","--mount","type=volume,source=$cacheVolume,target=/runtime-state","--entrypoint","/bin/sh",$toolsRef,"-lc","mkdir -p /runtime-state/go-mod-cache /runtime-state/go-cache; chown -R 1000:1000 /runtime-state")
    if ($cachePrepare.Code -ne 0) { throw "Go 构建缓存卷权限初始化失败: $($cachePrepare.Text)" }
    $sourceCreated = Invoke-Docker @("volume","create",$sourceVolume)
    if ($sourceCreated.Code -ne 0) { throw "创建 OP-Stack Docker 源码卷失败: $($sourceCreated.Text)" }
    $createdVolumes += $sourceVolume
    $sourceCopy = Invoke-Docker -TimeoutSeconds 900 @("run","--rm","--pull","never","--user","0:0","--mount","type=bind,source=$($optimismSourceDir.FullName),target=/in,readonly","--mount","type=volume,source=$sourceVolume,target=/src","--entrypoint","bash",$goBuilderRef,"-lc","set -eu; cp -a /in/. /src/; chown -R 1000:1000 /src")
    if ($sourceCopy.Code -ne 0) { throw "OP-Stack 源码复制到 Docker named volume 失败: $($sourceCopy.Text)" }
    Write-Stage "使用新候选 Foundry 编译官方合约材料"
    $foundryCommand = 'set -eu; export HOME=/runtime-state/foundry-home; export FOUNDRY_DISABLE_NIGHTLY_INSTALLS=1; export PATH=/usr/local/go/bin:/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin; for version in 0.8.15 0.8.19 0.8.20 0.8.24 0.8.25 0.8.26; do mkdir -p "$HOME/.svm/$version"; cp "/usr/local/bin/solc-$version" "$HOME/.svm/$version/solc-$version"; chmod 0755 "$HOME/.svm/$version/solc-$version"; done; cd /src/op-stack/packages/contracts-bedrock; timeout --foreground 1200s forge build --offline --deny never --skip "test/**" --skip "script/**"'
    # 分离 create/start/inspect,使 Docker attach 的 unexpected EOF 不会覆盖容器真实退出码。
    $foundryCreate = Invoke-Docker -TimeoutSeconds 60 @("create","--pull","never","--user","1000:1000","--name",$foundryContainer,"--mount","type=volume,source=$sourceVolume,target=/src/op-stack","--mount","type=volume,source=$cacheVolume,target=/runtime-state","--entrypoint","bash",$foundryRef,"-lc",$foundryCommand)
    if ($foundryCreate.Code -ne 0) { throw "创建官方合约离线编译容器失败: $($foundryCreate.Text)" }
    $createdContainers += $foundryContainer
    $foundryStart = Invoke-Docker -TimeoutSeconds 60 @("start",$foundryContainer)
    if ($foundryStart.Code -ne 0) { throw "启动官方合约离线编译容器失败: $($foundryStart.Text)" }
    $foundryState = $null
    $foundryDeadline = [DateTime]::UtcNow.AddSeconds(1200)
    while ([DateTime]::UtcNow -lt $foundryDeadline) {
        $foundryState = Invoke-Docker -TimeoutSeconds 10 @("inspect","--format","{{.State.Status}}|{{.State.ExitCode}}",$foundryContainer)
        if ($foundryState.Code -eq 0 -and $foundryState.Text -match '^exited\|') { break }
        Start-Sleep -Seconds 1
    }
    if ($foundryState.Code -ne 0 -or $foundryState.Text -notmatch '^exited\|0$') {
        $foundryLogs = Invoke-Docker -TimeoutSeconds 30 @("logs","--tail","200",$foundryContainer)
        throw "官方合约材料离线编译失败: state=$($foundryState.Text) logs=$($foundryLogs.Text)"
    }
    # 材料导出只允许使用项目统一缓存,网络关闭时缺失模块会立即失败,避免隐式公网下载。
    $exportCommand = 'set -eu; export PATH=/usr/local/go/bin:/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin; export GOPROXY=off; export GOSUMDB=off; go version; cd /src/op-stack; timeout --foreground 180s env GOMODCACHE=/runtime-state/go-mod-cache GOCACHE=/runtime-state/go-cache go mod download; timeout --foreground 300s env GOMODCACHE=/runtime-state/go-mod-cache GOCACHE=/runtime-state/go-cache CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go run ./cmd/chaimir-material-export -out /out'
    $exportBuild = Invoke-Docker -TimeoutSeconds 1800 @("run","--rm","--pull","never","--network","none","--user","1000:1000","--mount","type=volume,source=$sourceVolume,target=/src/op-stack","--mount","type=bind,source=$export,target=/out","--mount","type=volume,source=$cacheVolume,target=/runtime-state","--entrypoint","bash",$goBuilderRef,"-lc",$exportCommand)
    if ($exportBuild.Code -ne 0) { throw "官方 OP-Stack 材料导出失败: $($exportBuild.Text)" }
    foreach ($file in @("deploy-config.json","l1-allocs.json","l1-deployments.json","l2-allocs.json")) { Copy-Item -LiteralPath (Join-Path $export $file) -Destination (Join-Path $materialRoot $file) -Force }
    Copy-Item -LiteralPath (Join-Path $optimismSourceDir.FullName "op-core\superchain\superchain-configs.zip") -Destination (Join-Path $materialRoot "superchain-configs.zip") -Force
    # 官方 e2e 配置默认带有 Anvil 测试角色;生产材料必须把所有实际签名角色统一绑定到平台受控账户。
    $normalizeDeployConfigCommand = "set -eu; jq --arg address '$deployerAddress' ' .proxyAdminOwner = `$address | .finalSystemOwner = `$address | .baseFeeVaultRecipient = `$address | .l1FeeVaultRecipient = `$address | .sequencerFeeVaultRecipient = `$address | .operatorFeeVaultRecipient = `$address | .governanceTokenOwner = `$address | .liquidityControllerOwner = `$address | .p2pSequencerAddress = `$address | .batchSenderAddress = `$address | .superchainConfigGuardian = `$address | .l2OutputOracleProposer = `$address | .l2OutputOracleChallenger = `$address ' /material/deploy-config.json > /material/deploy-config.json.next; mv /material/deploy-config.json.next /material/deploy-config.json; jq -e --arg address '$deployerAddress' '[.proxyAdminOwner,.finalSystemOwner,.baseFeeVaultRecipient,.l1FeeVaultRecipient,.sequencerFeeVaultRecipient,.operatorFeeVaultRecipient,.liquidityControllerOwner,.p2pSequencerAddress,.batchSenderAddress,.superchainConfigGuardian,.l2OutputOracleProposer,.l2OutputOracleChallenger] | all(.[]; . == `$address)' /material/deploy-config.json >/dev/null"
    $normalizeDeployConfig = Invoke-Docker @("run","--rm","--pull","never","--user","0:0","--mount","type=bind,source=$materialRoot,target=/material", "--entrypoint","sh",$toolsRef,"-lc",$normalizeDeployConfigCommand)
    if ($normalizeDeployConfig.Code -ne 0) { throw "OP-Stack 签名角色统一绑定失败: $($normalizeDeployConfig.Text)" }
    # L1/L2 所有真实操作都使用同一平台账户,因此两条链的 genesis allocs 都必须注入该账户资金。
    $fundDeployerCommand = "set -eu; jq --arg address '$deployerAddress' --arg balance '0x3635c9adc5dea00000' '.[(`$address)] = {balance: `$balance}' /material/l1-allocs.json > /material/l1-allocs.json.next; mv /material/l1-allocs.json.next /material/l1-allocs.json; jq --arg address '$deployerAddress' --arg balance '0x3635c9adc5dea00000' '.[(`$address)] = {balance: `$balance}' /material/l2-allocs.json > /material/l2-allocs.json.next; mv /material/l2-allocs.json.next /material/l2-allocs.json; jq -e --arg address '$deployerAddress' 'has(`$address)' /material/l1-allocs.json >/dev/null; jq -e --arg address '$deployerAddress' 'has(`$address)' /material/l2-allocs.json >/dev/null"
    $fundDeployer = Invoke-Docker @("run","--rm","--pull","never","--user","0:0","--mount","type=bind,source=$materialRoot,target=/material", "--entrypoint","sh",$toolsRef,"-lc",$fundDeployerCommand)
    if ($fundDeployer.Code -ne 0) { throw "平台统一 EVM 部署账户写入 L1/L2 allocs 失败: $($fundDeployer.Text)" }

    # 使用同一官方 op-node 生成带 Optimism 合约状态的 L1 genesis,禁止回退到通用执行层模板。
    Write-Stage "生成官方 OP-Stack L1 genesis"
    $genesis = Invoke-Docker @("run","--rm","--pull","never","--mount","type=bind,source=$materialRoot,target=/material","--entrypoint","op-node",$opNodeRef,"genesis","l1","--deploy-config","/material/deploy-config.json","--l1-allocs","/material/l1-allocs.json","--l1-deployments","/material/l1-deployments.json","--outfile.l1","/material/l1-genesis.json")
    if ($genesis.Code -ne 0) { throw "官方 OP-Stack L1 genesis 生成失败: $($genesis.Text)" }
    Copy-Item -LiteralPath (Join-Path $materialRoot "l1-genesis.json") -Destination (Join-Path $materialRoot "genesis.json") -Force

    $templatePath = Join-Path $beaconSourceDir.FullName "config-example\cl\config.yaml"
    if (-not (Test-Path -LiteralPath $templatePath -PathType Leaf)) { throw "固定共识生成器缺少共识配置模板" }
    $config = Get-Content -LiteralPath $templatePath -Raw
    $l1Genesis = Get-Content -LiteralPath (Join-Path $materialRoot "l1-genesis.json") -Raw | ConvertFrom-Json
    $l1ChainId = [int]$l1Genesis.config.chainId
    # 生成器模板只允许由本次材料生成明确替换,任何遗漏都必须失败,不能把协议参数静默改成 0。
    $values = [ordered]@{
        PRESET_BASE = "minimal"; CONFIG_NAME = "chaimir-op-stack"; TERMINAL_TOTAL_DIFFICULTY = "0"
        NUMBER_OF_VALIDATORS = [string]$ValidatorCount; GENESIS_TIMESTAMP = [string]([DateTimeOffset]::UtcNow.ToUnixTimeSeconds() + 30)
        GENESIS_FORK_VERSION = "0x10000000"; GENESIS_DELAY = "0"; ALTAIR_FORK_VERSION = "0x20000000"; ALTAIR_FORK_EPOCH = "0"
        BELLATRIX_FORK_VERSION = "0x30000000"; BELLATRIX_FORK_EPOCH = "0"; CAPELLA_FORK_VERSION = "0x40000000"; CAPELLA_FORK_EPOCH = "0"
        DENEB_FORK_VERSION = "0x50000000"; DENEB_FORK_EPOCH = "0"; ELECTRA_FORK_VERSION = "0x60000000"; ELECTRA_FORK_EPOCH = "18446744073709551615"
        FULU_FORK_VERSION = "0x70000000"; FULU_FORK_EPOCH = "18446744073709551615"; GLOAS_FORK_VERSION = "0x80000000"; GLOAS_FORK_EPOCH = "18446744073709551615"
        HEZE_FORK_VERSION = "0x90000000"; HEZE_FORK_EPOCH = "18446744073709551615"; SLOT_DURATION_MS = "6000"; SECONDS_PER_ETH1_BLOCK = "14"
        MIN_VALIDATOR_WITHDRAWABILITY_DELAY = "256"; SHARD_COMMITTEE_PERIOD = "64"; ETH1_FOLLOW_DISTANCE = "16"; MIN_BUILDER_WITHDRAWABILITY_DELAY = "2"
        ATTESTATION_DUE_BPS_GLOAS = "2500"; AGGREGATE_DUE_BPS_GLOAS = "5000"; SYNC_MESSAGE_DUE_BPS_GLOAS = "2500"; CONTRIBUTION_DUE_BPS_GLOAS = "5000"; PAYLOAD_DUE_BPS = "5000"; PAYLOAD_ATTESTATION_DUE_BPS = "7500"
        EJECTION_BALANCE = "16000000000"; MIN_PER_EPOCH_CHURN_LIMIT = "2"; CHURN_LIMIT_QUOTIENT = "32"; MAX_PER_EPOCH_ACTIVATION_CHURN_LIMIT = "4"
        MIN_PER_EPOCH_CHURN_LIMIT_ELECTRA = "64000000000"; MAX_PER_EPOCH_ACTIVATION_EXIT_CHURN_LIMIT = "128000000000"; CHURN_LIMIT_QUOTIENT_GLOAS = "16"; CONSOLIDATION_CHURN_LIMIT_QUOTIENT = "32"; MAX_PER_EPOCH_ACTIVATION_CHURN_LIMIT_GLOAS = "128000000000"
        MAX_PAYLOAD_SIZE = "10485760"; MAX_REQUEST_BLOCKS_DENEB = "128"; MIN_EPOCHS_FOR_BLOB_SIDECARS_REQUESTS = "4096"; MAX_BLOBS_PER_BLOCK_ELECTRA = "9"; SAMPLES_PER_SLOT = "8"; CUSTODY_REQUIREMENT = "4"; MIN_EPOCHS_FOR_DATA_COLUMN_SIDECARS_REQUESTS = "4096"
        CHAIN_ID = [string]$l1ChainId; DEPOSIT_CHAIN_ID = [string]$l1ChainId; DEPOSIT_NETWORK_ID = [string]$l1ChainId
        # 本项目从受控 mnemonic 直接生成创世 validator,不依赖 L1 deposit event;零地址是 Beacon 无 deposit 合约的协议哨兵值,不是业务合约地址。
        DEPOSIT_CONTRACT_ADDRESS = "0x0000000000000000000000000000000000000000"; CONFIRMATION_BYZANTINE_THRESHOLD = "25"
    }
    foreach ($entry in ($values.GetEnumerator() | Sort-Object { $_.Key.Length } -Descending)) { $config = $config.Replace(('$' + $entry.Key), [string]$entry.Value) }
    $unresolved = [regex]::Matches($config, '\$[A-Z][A-Z0-9_]*') | ForEach-Object { $_.Value } | Sort-Object -Unique
    if (@($unresolved).Count -gt 0) { throw "Beacon config 模板存在未解析占位符: $($unresolved -join ', ')" }
    Write-Utf8NoBom (Join-Path $materialRoot "config.yaml") $config
    $mnemonic = Read-PlatformSecret "CHAIMIR_EVM_DEPLOYER_MNEMONIC"
    Write-Utf8NoBom (Join-Path $materialRoot "mnemonics.yaml") ("- mnemonic: `"$mnemonic`"`n  name: `"chaimir-controlled`"`n  start: 0`n  count: $ValidatorCount`n  wd_address: null`n  wd_prefix: 0x00`n  balance: 32000000000`n  status: 0`n")
    Set-Content -LiteralPath (Join-Path $materialRoot "deposit_contract.txt") -Value "0x0000000000000000000000000000000000000000" -Encoding ASCII
    Set-Content -LiteralPath (Join-Path $materialRoot "deposit_contract_block.txt") -Value "0" -Encoding ASCII
    Set-Content -LiteralPath (Join-Path $materialRoot "bootstrap_nodes.txt") -Value "controlled" -Encoding ASCII

    Write-Stage "生成受控 Beacon genesis state"
    $binaryMount = "type=bind,source=$generatorBinary,target=/tools/eth-genesis-state-generator,readonly"
    $materialMount = "type=bind,source=$materialRoot,target=/material"
    $generate = Invoke-Docker @("run","--rm","--pull","never","--user","0:0","--mount",$binaryMount,"--mount",$materialMount,"--entrypoint","/tools/eth-genesis-state-generator",$beaconRef,"beaconchain","--config","/material/config.yaml","--eth1-config","/material/genesis.json","--mnemonics","/material/mnemonics.yaml","--state-output","/material/genesis.ssz","--json-output","/material/parsedConsensusGenesis.json","--validators-mapping-output","/material/validator_names.yaml")
    if ($generate.Code -ne 0) { throw "受控 Beacon genesis state 生成失败: $($generate.Text)" }

    Write-Stage "启动受控 L1 和 Beacon 生成 L2 genesis 与 rollup"
    $created = Invoke-Docker @("network","create","--internal",$network)
    if ($created.Code -ne 0) { throw "创建 OP-Stack 材料生成网络失败: $($created.Text)" }
    $createdNetwork = $true
    foreach ($volume in @($gethVolume,$beaconVolume,$validatorVolume)) {
        $created = Invoke-Docker @("volume","create",$volume)
        if ($created.Code -ne 0) { throw "创建材料卷失败: $($created.Text)" }
        $createdVolumes += $volume
    }
    $prepare = Invoke-Docker @("run","--rm","--pull","never","--user","0:0","--mount","source=$gethVolume,target=/runtime-state","--mount","type=bind,source=$(Join-Path $materialRoot 'genesis.json'),target=/seed/genesis.json,readonly","--entrypoint","/bin/sh",$toolsRef,"-lc","mkdir -p /runtime-state/op-geth; cp /seed/genesis.json /runtime-state/op-geth/genesis.json; chown -R 1000:1000 /runtime-state")
    if ($prepare.Code -ne 0) { throw "OP-Geth genesis 准备注入失败: $($prepare.Text)" }
    $started = Invoke-Docker @("run","-d","--pull","never","--name",$gethContainer,"--network",$network,"--network-alias","opgeth","-v","${gethVolume}:/runtime-state","-e","OP_GETH_DATA_DIR=/runtime-state/op-geth","-e","OP_GETH_GENESIS_PATH=/runtime-state/op-geth/genesis.json","-e","OP_GETH_JWT_SECRET=$jwt","-e","OP_GETH_RPC_PORT=8545","-e","OP_GETH_ENGINE_PORT=8551",$gethRef)
    if ($started.Code -ne 0) { throw "OP-Geth 启动失败: $($started.Text)" }; $createdContainers += $gethContainer
    $rpcReady = Invoke-Docker @("run","--rm","--pull","never","--network",$network,"--entrypoint","/bin/sh",$toolsRef,"-lc","set -eu; for n in `$(seq 1 60); do curl --fail --silent --show-error -H 'content-type: application/json' --data '{`"jsonrpc`":`"2.0`",`"id`":1,`"method`":`"eth_chainId`",`"params`":[]}' http://opgeth:8545/ | grep -q result && exit 0; sleep 1; done; exit 1")
    if ($rpcReady.Code -ne 0) { throw "OP-Geth L1 RPC 在 60 秒内未就绪: $($rpcReady.Text)" }
    # L2 genesis 和 rollup config 必须由同一 L1 genesis、同一 deployment 材料和真实 L1 RPC 生成。
    $l2 = Invoke-Docker @("run","--rm","--pull","never","--network",$network,"--mount",$materialMount,"--entrypoint","op-node",$opNodeRef,"genesis","l2","--deploy-config","/material/deploy-config.json","--l1-deployments","/material/l1-deployments.json","--l1-rpc","http://opgeth:8545","--l2-allocs","/material/l2-allocs.json","--outfile.l2","/material/l2-genesis.json","--outfile.rollup","/material/rollup.json")
    if ($l2.Code -ne 0) { throw "官方 OP-Stack L2 genesis/rollup 生成失败: $($l2.Text)" }
    $served = Invoke-Docker @("run","-d","--pull","never","--name",$sourceContainer,"--network",$network,"--network-alias","source","--mount","type=bind,source=$materialRoot,target=/material,readonly","--entrypoint","/bin/sh",$toolsRef,"-lc","size=`$(wc -c < /material/genesis.ssz); for n in `$(seq 1 30); do { printf 'HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %s\r\nConnection: close\r\n\r\n' `"`$size`"; cat /material/genesis.ssz; } | timeout 30 nc -l -p 8000 || exit `$?; done; exit 1")
    if ($served.Code -ne 0) { throw "受控 Beacon genesis state 服务启动失败: $($served.Text)" }; $createdContainers += $sourceContainer
    $bootstrap = Invoke-Docker @("run","-d","--pull","never","--name",$beaconContainer,"--network",$network,"--network-alias","beacon","-v","${beaconVolume}:/runtime-state","--mount","type=bind,source=$materialRoot,target=/material,readonly","--entrypoint","lighthouse",$beaconRef,"beacon_node","--testnet-dir","/material","--datadir","/runtime-state/lighthouse","--execution-endpoint","http://opgeth:8551","--execution-jwt-secret-key",$jwt,"--genesis-state-url","http://source:8000/state","--http","--http-address","0.0.0.0","--http-port","5052")
    if ($bootstrap.Code -ne 0) { throw "受控 Beacon bootstrap 启动失败: $($bootstrap.Text)" }; $createdContainers += $beaconContainer
    $beaconReady = Invoke-Docker @("run","--rm","--pull","never","--network",$network,"--entrypoint","/bin/sh",$toolsRef,"-lc","set -eu; for n in `$(seq 1 60); do curl --fail --silent --show-error --connect-timeout 2 --max-time 4 http://beacon:5052/eth/v1/node/health >/dev/null && exit 0; sleep 1; done; exit 1")
    if ($beaconReady.Code -ne 0) { throw "受控 Beacon HTTP API 在 60 秒内未就绪: $($beaconReady.Text)" }
    # genesis 状态只作为生成器输入,最终归档必须使用有 finalized head 的受控 checkpoint。
    # validator 密钥只在本轮受控卷和临时文件中存在,不写入镜像、仓库或最终材料归档。
    $mnemonic = Read-PlatformSecret "CHAIMIR_EVM_DEPLOYER_MNEMONIC"
    $validatorPassword = [Guid]::NewGuid().ToString("N") + [Guid]::NewGuid().ToString("N")
    Set-Content -LiteralPath $validatorMnemonicPath -Value $mnemonic -NoNewline -Encoding ASCII
    Set-Content -LiteralPath $validatorPasswordPath -Value $validatorPassword -NoNewline -Encoding ASCII
    $validatorRecoveryCommand = "set -eu; cat /input/validator-password /input/validator-password | lighthouse account_manager validator recover --testnet-dir /input/material --validator-dir /runtime-state/validators --secrets-dir /runtime-state/secrets --first-index 0 --count $ValidatorCount --mnemonic-path /input/validator-mnemonic --stdin-inputs"
    $validatorRecovery = Invoke-Docker @("run","--rm","--pull","never","--user","1000:1000","--mount","source=$validatorVolume,target=/runtime-state","--mount","type=bind,source=$workRoot,target=/input,readonly","--entrypoint","sh",$beaconRef,"-lc",$validatorRecoveryCommand)
    if ($validatorRecovery.Code -ne 0) { throw "受控 validator 密钥恢复失败: $($validatorRecovery.Text)" }
    $validatorStarted = Invoke-Docker @("run","-d","--pull","never","--user","1000:1000","--name",$validatorContainer,"--network",$network,"--network-alias","validator","--mount","source=$validatorVolume,target=/runtime-state","--mount","type=bind,source=$materialRoot,target=/material,readonly","--entrypoint","lighthouse",$beaconRef,"validator_client","--testnet-dir","/material","--validators-dir","/runtime-state/validators","--secrets-dir","/runtime-state/secrets","--beacon-nodes","http://beacon:5052","--suggested-fee-recipient",$deployerAddress,"--init-slashing-protection","--disable-log-timestamp")
    if ($validatorStarted.Code -ne 0) { throw "受控 validator client 启动失败: $($validatorStarted.Text)" }; $createdContainers += $validatorContainer
    $finalizedCommand = 'set -eu; for n in $(seq 1 600); do slot=$(curl --fail --silent --show-error --connect-timeout 2 --max-time 4 http://beacon:5052/eth/v1/beacon/headers/finalized | jq -r .data.header.message.slot); if test x$slot = x || test x$slot = xnull || test x$slot = x0; then :; elif test $slot -ge 32; then printf %s $slot; exit 0; fi; sleep 1; done; exit 1'
    # finalized 探针最多运行 600 秒, Docker CLI 超时必须覆盖探针上限并保持有界返回。
    $finalized = Invoke-Docker -TimeoutSeconds 660 @("run","--rm","--pull","never","--network",$network,"--entrypoint","/bin/sh",$toolsRef,"-lc",$finalizedCommand)
    if ($finalized.Code -ne 0) {
        $validatorLog = (Invoke-Docker @("logs","--tail","120",$validatorContainer)).Text
        $beaconLog = (Invoke-Docker @("logs","--tail","120",$beaconContainer)).Text
        throw "受控 Beacon finalized checkpoint 在 600 秒内未形成: $($finalized.Text)`nvalidator=$validatorLog`nbeacon=$beaconLog"
    }
    $finalizedSlot = [UInt64]$finalized.Text.Trim()
    $checkpointCommand = 'set -eu; curl --fail --silent --show-error -H Accept:application/octet-stream -o /material/genesis.ssz http://beacon:5052/eth/v2/debug/beacon/states/finalized; curl --fail --silent --show-error -H Accept:application/octet-stream -o /material/genesis-block.ssz http://beacon:5052/eth/v2/beacon/blocks/finalized; test -s /material/genesis.ssz; test -s /material/genesis-block.ssz'
    # finalized 文件先由共识生成器以受控材料身份创建,导出覆盖必须使用材料生成阶段的 root 权限边界。
    $checkpoint = Invoke-Docker @("run","--rm","--pull","never","--user","0:0","--network",$network,"--mount","type=bind,source=$materialRoot,target=/material","--entrypoint","/bin/sh",$toolsRef,"-lc",$checkpointCommand)
    if ($checkpoint.Code -ne 0) { throw "受控 Beacon finalized checkpoint 导出失败: $($checkpoint.Text)" }
    # finalized checkpoint 引用的 L1 execution head 必须随同归档保存,否则重新从 genesis 启动会丢失共识引用的区块。
    $executionStateExport = Invoke-Docker -TimeoutSeconds 900 @("run","--rm","--pull","never","--user","0:0","--mount","source=$gethVolume,target=/runtime-state","--mount","type=bind,source=$materialRoot,target=/out","--entrypoint","/bin/sh",$toolsRef,"-lc","set -eu; tar -cf /out/l1-execution-state.tar -C /runtime-state op-geth/geth op-geth/chaimir-genesis-initialized; test -s /out/l1-execution-state.tar")
    if ($executionStateExport.Code -ne 0) { throw "受控 L1 execution 状态导出失败: $($executionStateExport.Text)" }
    Write-Stage "校验完整材料并打包"
    foreach ($file in @("config.yaml","genesis.json","l1-genesis.json","l2-genesis.json","rollup.json","deploy-config.json","l1-allocs.json","l1-deployments.json","l2-allocs.json","superchain-configs.zip","genesis.ssz","genesis-block.ssz","parsedConsensusGenesis.json","validator_names.yaml","deposit_contract.txt","deposit_contract_block.txt","bootstrap_nodes.txt","l1-execution-state.tar")) {
        Assert-File $materialRoot $file
        Copy-Item -LiteralPath (Join-Path $materialRoot $file) -Destination (Join-Path $archiveRoot $file)
    }
    $deployConfig = Get-Content (Join-Path $materialRoot "deploy-config.json") -Raw | ConvertFrom-Json
    $dependencyManifest = [ordered]@{}
    foreach ($dependency in $dependencySpecs) { $dependencyManifest[$dependency.Name] = [ordered]@{ repository = $dependency.Repository; commit = $dependency.Commit; archive_sha256 = $dependency.Sha256 } }
    $manifest = [ordered]@{ schema_version = 2; protocol = "op-stack-beacon"; l1_chain_id = $l1ChainId; l2_chain_id = [int]$deployConfig.l2ChainID; validator_count = $ValidatorCount; funded_accounts = @([ordered]@{ address = $deployerAddress; balance_wei = "1000000000000000000000"; source = "platform Secret/KMS CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY" }); checkpoint = [ordered]@{ type = "finalized"; slot = $finalizedSlot }; generators = [ordered]@{ beacon_binary_sha256 = $generatorSha256; beacon_source_commit = "0bd028e7217307593f1bc441607d45d26ca62d19"; beacon_source_sha256 = $beaconSourceSha256; optimism_source_commit = "da197e45ed44b9fca258b3b0d0709e8dfca1c7cd"; optimism_source_sha256 = $optimismSourceSha256; superchain_registry_commit = "d84b53b7ee008f863c1d3bbb49e06f713f713978"; superchain_registry_sha256 = $registrySha256; foundry_dependencies = $dependencyManifest }; files = [ordered]@{} }
    foreach ($file in Get-ChildItem -LiteralPath $archiveRoot -File) { $manifest.files[$file.Name] = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant() }
    Write-Utf8NoBom (Join-Path $archiveRoot "material-manifest.json") (($manifest | ConvertTo-Json -Depth 8) + [Environment]::NewLine)
    New-Item -ItemType Directory -Path (Split-Path -Parent $outputAbsolute) -Force | Out-Null
    tar -czf $outputAbsolute -C $archiveRoot .
    if (-not (Test-Path -LiteralPath $outputAbsolute -PathType Leaf)) { throw "受控 OP-Stack 初始化归档未生成" }
    Write-Output "受控 OP-Stack 完整初始化归档已生成: $outputAbsolute"
} finally {
    foreach ($container in $createdContainers) { Invoke-Docker @("rm","-f","-v",$container) | Out-Null }
    foreach ($volume in $createdVolumes) { Invoke-Docker @("volume","rm",$volume) | Out-Null }
    if ($createdNetwork) { Invoke-Docker @("network","rm",$network) | Out-Null }
    if (Test-Path -LiteralPath $workRoot) { Remove-Item -LiteralPath $workRoot -Recurse -Force }
}
