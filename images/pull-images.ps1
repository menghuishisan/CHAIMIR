# 本脚本按 manifest 与构建产物 digest 锁顺序拉取 Chaimir 镜像。
param(
    [string]$Root = (Split-Path -Parent $MyInvocation.MyCommand.Path),
    [ValidateSet("all", "upstream-pinned", "built")]
    [string]$Scope = "all",
    [string]$Registry = $env:CHAIMIR_IMAGE_REGISTRY,
    [string]$DockerConfig = $env:DOCKER_CONFIG,
    [string]$DigestLock = "",
    [switch]$PublishLocalBuilt,
    [string]$LocalSourceRegistry = "",
    [string]$LocalSourceTag = "",
    [string]$PublishTag = "",
    [switch]$PrintBuildArgs,
    [int]$MaxAttempts = 3,
    [int]$RetryDelaySeconds = 5,
    [switch]$FailFast,
    [switch]$NoCleanupFailedPull
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Registry)) {
    $Registry = $env:SUPPLY_CHAIN_REGISTRY
}
if ([string]::IsNullOrWhiteSpace($Registry)) {
    $Registry = $env:IMAGE_REGISTRY
}
if ([string]::IsNullOrWhiteSpace($Registry)) {
    $Registry = "harbor.chaimir:30080"
}
if ([string]::IsNullOrWhiteSpace($DigestLock)) {
    $DigestLock = Join-Path $Root "image-digests.lock"
}
if ($MaxAttempts -lt 1) {
    throw "MaxAttempts 必须大于等于 1"
}
if ($RetryDelaySeconds -lt 0) {
    throw "RetryDelaySeconds 不能小于 0"
}
if ($PublishLocalBuilt -and $Scope -eq "upstream-pinned") {
    throw "PublishLocalBuilt 只能与 Scope=all 或 Scope=built 一起使用"
}
if ($PrintBuildArgs -and $Scope -eq "upstream-pinned") {
    throw "PrintBuildArgs 只能与 Scope=all 或 Scope=built 一起使用"
}
if ($PublishLocalBuilt -and [string]::IsNullOrWhiteSpace($LocalSourceRegistry)) {
    $LocalSourceRegistry = $Registry
}
if ($PublishLocalBuilt -and [string]::IsNullOrWhiteSpace($LocalSourceTag)) {
    throw "启用 PublishLocalBuilt 时必须显式传入 LocalSourceTag,不得隐式使用本地可变调试标签"
}
if ($PublishLocalBuilt -and [string]::IsNullOrWhiteSpace($PublishTag)) {
    throw "启用 PublishLocalBuilt 时必须显式传入 PublishTag,不得隐式发布可变调试标签"
}

function Read-YamlValue {
    param(
        [string[]]$Lines,
        [string]$Key
    )
    foreach ($line in $Lines) {
        if ($line -match "^\s*$([regex]::Escape($Key)):\s*(.+?)\s*$") {
            return $Matches[1].Trim().Trim('"').Trim("'")
        }
    }
    return $null
}

function Read-TopLevelYamlValue {
    param(
        [string[]]$Lines,
        [string]$Key
    )
    foreach ($line in $Lines) {
        if ($line -match "^$([regex]::Escape($Key)):\s*(.+?)\s*$") {
            return $Matches[1].Trim().Trim('"').Trim("'")
        }
    }
    return $null
}

function Read-YamlBlock {
    param(
        [string]$Path,
        [string]$BlockName
    )
    $lines = Get-Content $Path
    $block = New-Object System.Collections.Generic.List[string]
    $inside = $false
    foreach ($line in $lines) {
        if ($line -match "^$([regex]::Escape($BlockName)):\s*$") {
            $inside = $true
            continue
        }
        if ($inside -and $line -match "^[A-Za-z_][A-Za-z0-9_]*:\s*") {
            break
        }
        if ($inside) {
            $block.Add($line)
        }
    }
    return ,$block.ToArray()
}

function Read-SourceType {
    param([string]$Path)
    $source = Read-YamlBlock -Path $Path -BlockName "source"
    return Read-YamlValue -Lines $source -Key "type"
}

# Test-DeployableManifest 判断 manifest 是否允许进入拉取、扫描和准入流程。
function Test-DeployableManifest {
    param([string]$Path)
    $supplyChain = Read-YamlBlock -Path $Path -BlockName "supply_chain"
    $deployable = Read-YamlValue -Lines $supplyChain -Key "deployable"
    if ($deployable -ne "false") {
        return $true
    }
    $reason = Read-YamlValue -Lines $supplyChain -Key "block_reason"
    if ([string]::IsNullOrWhiteSpace($reason)) {
        throw "${Path}: supply_chain.deployable=false 必须声明 block_reason"
    }
    Write-Host "Skipping non-deployable image manifest: $Path ($reason)"
    return $false
}

function Read-Components {
    param([string[]]$Upstream)
    $components = @()
    $current = $null
    foreach ($line in $Upstream) {
        if ($line -match "^\s*-\s+image:\s*(.+?)\s*$") {
            if ($current) {
                $components += $current
            }
            $current = [ordered]@{ image = $Matches[1].Trim().Trim('"').Trim("'") }
            continue
        }
        if ($current -and $line -match "^\s+registry:\s*(.+?)\s*$") {
            $current.registry = $Matches[1].Trim().Trim('"').Trim("'")
            continue
        }
        if ($current -and $line -match "^\s+version:\s*(.+?)\s*$") {
            $current.version = $Matches[1].Trim().Trim('"').Trim("'")
            continue
        }
        if ($current -and $line -match "^\s+digest:\s*(sha256:[0-9a-f]{64})\s*$") {
            $current.digest = $Matches[1]
        }
    }
    if ($current) {
        $components += $current
    }
    return $components
}

function Read-DigestLock {
    param([string]$Path)
    $items = @{}
    if (-not (Test-Path -LiteralPath $Path)) {
        return $items
    }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq "" -or $trimmed.StartsWith("#")) {
            continue
        }
        if ($trimmed -match "^([^:\s]+/[^:\s]+)\s*[:= ]\s*(sha256:[0-9a-f]{64})$") {
            $items[$Matches[1]] = $Matches[2]
            continue
        }
        throw "digest 锁格式非法: $Path -> $line"
    }
    return $items
}

function Get-BuildArgRef {
    param(
        [hashtable]$DigestLockItems,
        [string]$ImageName,
        [string]$ArgName
    )
    $digest = $DigestLockItems[$ImageName]
    if ([string]::IsNullOrWhiteSpace($digest)) {
        throw "无法生成 $ArgName, digest 锁缺少 $ImageName"
    }
    return "$ArgName=$Registry/$ImageName@$digest"
}

function Add-ImageRef {
    param(
        [System.Collections.Generic.List[object]]$Items,
        [hashtable]$Seen,
        [string]$Ref,
        [string]$Kind,
        [string]$Manifest
    )
    if ($Seen.ContainsKey($Ref)) {
        return
    }
    $Seen[$Ref] = $true
    $Items.Add([pscustomobject]@{ Ref = $Ref; Kind = $Kind; Manifest = $Manifest })
}

function Add-UpstreamRefs {
    param(
        [string]$ManifestPath,
        [System.Collections.Generic.List[object]]$Items,
        [hashtable]$Seen,
        [System.Collections.Generic.List[string]]$Missing
    )
    $upstream = Read-YamlBlock -Path $ManifestPath -BlockName "upstream"
    $registry = Read-YamlValue -Lines $upstream -Key "registry"
    $image = Read-YamlValue -Lines $upstream -Key "image"
    $digest = Read-YamlValue -Lines $upstream -Key "digest"
    $components = Read-Components -Upstream $upstream

    if ($components.Count -gt 0) {
        foreach ($component in $components) {
            if (-not $component.digest) {
                $Missing.Add(("{0}: component {1}:{2} 缺少 digest" -f $ManifestPath, $component.image, $component.version))
                continue
            }
            $componentRegistry = $component.registry
            if ([string]::IsNullOrWhiteSpace($componentRegistry)) {
                $componentRegistry = $registry
            }
            if ([string]::IsNullOrWhiteSpace($componentRegistry)) {
                $componentRegistry = "docker.io"
            }
            Add-ImageRef -Items $Items -Seen $Seen -Ref "$componentRegistry/$($component.image)@$($component.digest)" -Kind "upstream-pinned" -Manifest $ManifestPath
        }
        return
    }

    if ([string]::IsNullOrWhiteSpace($registry) -or [string]::IsNullOrWhiteSpace($image) -or [string]::IsNullOrWhiteSpace($digest)) {
        $Missing.Add(("{0}: upstream registry/image/digest 缺失" -f $ManifestPath))
        return
    }
    Add-ImageRef -Items $Items -Seen $Seen -Ref "$registry/$image@$digest" -Kind "upstream-pinned" -Manifest $ManifestPath
}

function Add-BuiltRef {
    param(
        [string]$ManifestPath,
        [hashtable]$DigestLockItems,
        [System.Collections.Generic.List[object]]$Items,
        [hashtable]$Seen,
        [System.Collections.Generic.List[string]]$Missing
    )
    $lines = Get-Content $ManifestPath
    $image = Read-TopLevelYamlValue -Lines $lines -Key "image"
    if ([string]::IsNullOrWhiteSpace($image)) {
        $Missing.Add(("{0}: image 缺失" -f $ManifestPath))
        return
    }
    $digest = $DigestLockItems[$image]
    if ([string]::IsNullOrWhiteSpace($digest)) {
        $Missing.Add(("{0}: 缺少构建产物 digest,请在 image-digests.lock 中声明 {1} 的不可变 digest" -f $ManifestPath, $image))
        return
    }
    Add-ImageRef -Items $Items -Seen $Seen -Ref "$Registry/$image@$digest" -Kind "built" -Manifest $ManifestPath
}

function Invoke-CheckedDocker {
    param(
        [string[]]$Arguments,
        [string]$Context
    )
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $output = Invoke-DockerCli -Arguments $Arguments 2>&1
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    if ($exitCode -ne 0) {
        foreach ($line in $output) {
            Write-Warning $line
        }
        throw "$Context 失败, exit=$exitCode"
    }
    foreach ($line in $output) {
        Write-Host $line
    }
    return ,$output
}

function Invoke-DockerCli {
    param([string[]]$Arguments)
    $baseArguments = @()
    if (-not [string]::IsNullOrWhiteSpace($DockerConfig)) {
        $baseArguments += @("--config", $DockerConfig)
    }
    & docker @baseArguments @Arguments
}

function Get-RegistryDigest {
    param([string]$TaggedRef)
    $remoteDigest = Get-RemoteDigestFromHarbor -TaggedRef $TaggedRef
    if (-not [string]::IsNullOrWhiteSpace($remoteDigest)) {
        return $remoteDigest
    }

    $repository = $TaggedRef
    if ($TaggedRef -match "^(.+):[^/:]+$") {
        $repository = $Matches[1]
    }
    $inspectOutput = Invoke-CheckedDocker -Arguments @("image", "inspect", "--format", "{{json .RepoDigests}}", $TaggedRef) -Context "读取本地 push digest: $TaggedRef"
    foreach ($line in $inspectOutput) {
        if ($line -match "$([regex]::Escape($repository))@(sha256:[0-9a-f]{64})") {
            return $Matches[1]
        }
    }
    $output = Invoke-CheckedDocker -Arguments @("buildx", "imagetools", "inspect", $TaggedRef) -Context "读取 registry digest: $TaggedRef"
    foreach ($line in $output) {
        if ($line -match "^\s*Digest:\s*(sha256:[0-9a-f]{64})\s*$") {
            return $Matches[1]
        }
    }
    throw "未能从 registry 返回结果中解析 digest: $TaggedRef"
}

function Get-DockerConfigBasicAuth {
    param([string]$RegistryHost)
    if ([string]::IsNullOrWhiteSpace($DockerConfig)) {
        return $null
    }
    $configPath = Join-Path $DockerConfig "config.json"
    if (-not (Test-Path -LiteralPath $configPath)) {
        return $null
    }
    try {
        $config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
    } catch {
        throw "Docker config 无法解析: $configPath"
    }
    if (-not $config.auths) {
        return $null
    }
    foreach ($property in $config.auths.PSObject.Properties) {
        $name = $property.Name
        if ($name -eq $RegistryHost -or $name -eq "http://$RegistryHost" -or $name -eq "https://$RegistryHost") {
            $auth = $property.Value.auth
            if (-not [string]::IsNullOrWhiteSpace($auth)) {
                return $auth
            }
        }
    }
    return $null
}

function Get-RemoteDigestFromHarbor {
    param([string]$TaggedRef)
    if ($TaggedRef -notmatch "^([^/]+)/([^/]+)/(.+):([^/:]+)$") {
        return $null
    }
    $registryHost = $Matches[1]
    $project = $Matches[2]
    $repository = [uri]::EscapeDataString($Matches[3])
    $tag = $Matches[4]
    if (-not $registryHost.StartsWith("harbor.chaimir")) {
        return $null
    }
    $scheme = "https"
    if ($registryHost -eq "harbor.chaimir" -or $registryHost.StartsWith("harbor.chaimir:")) {
        $scheme = "http"
    }
    $uri = "$scheme`://$registryHost/api/v2.0/projects/$project/repositories/$repository/artifacts/$tag"
    $envOverrideName = "CHAIMIR_IMAGE_REMOTE_DIGEST_" + (($project + "_" + $Matches[3]) -replace "[^A-Za-z0-9]", "_").ToUpperInvariant()
    $envOverride = [Environment]::GetEnvironmentVariable($envOverrideName)
    if ($envOverride -match "^sha256:[0-9a-f]{64}$") {
        return $envOverride
    }
    $headers = @{}
    $basicAuth = Get-DockerConfigBasicAuth -RegistryHost $registryHost
    if (-not [string]::IsNullOrWhiteSpace($basicAuth)) {
        $headers["Authorization"] = "Basic $basicAuth"
    }
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $uri -Headers $headers -TimeoutSec 20
        $artifact = $response.Content | ConvertFrom-Json
        if ($artifact.digest -match "^sha256:[0-9a-f]{64}$") {
            return $artifact.digest
        }
    } catch {
        return $null
    }
    return $null
}

function Write-BuiltDigestLock {
    param(
        [string]$Path,
        [System.Collections.Generic.SortedDictionary[string,string]]$Items
    )
    $lockDir = Split-Path -Parent $Path
    if (-not [string]::IsNullOrWhiteSpace($lockDir) -and -not (Test-Path -LiteralPath $lockDir)) {
        New-Item -ItemType Directory -Path $lockDir | Out-Null
    }
    $lines = New-Object System.Collections.Generic.List[string]
    foreach ($key in $Items.Keys) {
        $lines.Add("$key $($Items[$key])")
    }
    Set-Content -LiteralPath $Path -Value $lines -Encoding ascii
}

function Publish-LocalBuiltImages {
    param(
        [object[]]$Manifests,
        [string]$OutputLock
    )
    $records = New-Object System.Collections.Generic.List[object]
    $missing = New-Object System.Collections.Generic.List[string]
    $seenLogical = @{}
    $lockItems = New-Object "System.Collections.Generic.SortedDictionary[string,string]"

    foreach ($manifest in $Manifests) {
        $sourceType = Read-SourceType -Path $manifest.FullName
        if ($sourceType -notin @("platform-built", "thin-wrapper", "build-base")) {
            continue
        }

        $lines = Get-Content $manifest.FullName
        $image = Read-TopLevelYamlValue -Lines $lines -Key "image"
        if ([string]::IsNullOrWhiteSpace($image)) {
            $missing.Add(("{0}: image 缺失" -f $manifest.FullName))
            continue
        }
        if ($seenLogical.ContainsKey($image)) {
            $missing.Add(("镜像 {0} 被多个 manifest 声明: {1} / {2}" -f $image, $seenLogical[$image], $manifest.FullName))
            continue
        }
        $seenLogical[$image] = $manifest.FullName

        $sourceRef = "$LocalSourceRegistry/$image`:$LocalSourceTag"
        $targetRef = "$Registry/$image`:$PublishTag"
        if (-not (Test-LocalImageRef -Ref $sourceRef)) {
            $missing.Add(("{0}: 缺少本地源镜像 {1}" -f $manifest.FullName, $sourceRef))
            continue
        }
        $records.Add([pscustomobject]@{
            Image = $image
            SourceRef = $sourceRef
            TargetRef = $targetRef
            Manifest = $manifest.FullName
        })
    }

    if ($missing.Count -gt 0) {
        Write-Error ("Refusing to publish local images because required built images are missing or invalid:`n" + ($missing -join "`n"))
        exit 1
    }

    foreach ($record in ($records | Sort-Object Image)) {
        Write-Host "Publishing local image $($record.SourceRef) -> $($record.TargetRef)"
        Invoke-CheckedDocker -Arguments @("tag", $record.SourceRef, $record.TargetRef) -Context "标记镜像: $($record.TargetRef)" | Out-Null
        Invoke-CheckedDocker -Arguments @("push", $record.TargetRef) -Context "docker push: $($record.TargetRef)" | Out-Null
        $digest = Get-RegistryDigest -TaggedRef $record.TargetRef
        # 清理临时发布 tag,后续验证只能依赖 registry 返回的不可变 digest。
        Invoke-CheckedDocker -Arguments @("image", "rm", $record.TargetRef) -Context "清理临时发布 tag: $($record.TargetRef)" | Out-Null
        $lockItems[$record.Image] = $digest
        Write-BuiltDigestLock -Path $OutputLock -Items $lockItems
    }
    Write-Host "Wrote built image digest lock: $OutputLock"
}

function Invoke-DockerPullWithRetry {
    param([object]$Item)
    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        Write-Host "Pulling [$attempt/$MaxAttempts] $($Item.Ref)"
        $hadImageBeforePull = Test-LocalImageRef -Ref $Item.Ref
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        try {
            $pullOutput = Invoke-DockerCli -Arguments @("pull", $Item.Ref) 2>&1
            $pullExitCode = $LASTEXITCODE
        } finally {
            $ErrorActionPreference = $previousErrorActionPreference
        }
        if ($pullExitCode -eq 0) {
            foreach ($line in $pullOutput) {
                Write-Host $line
            }
            return $true
        }

        foreach ($line in $pullOutput) {
            Write-Warning $line
        }
        Write-Warning "镜像拉取失败: $($Item.Ref), attempt=$attempt, exit=$pullExitCode"
        if (-not $NoCleanupFailedPull -and -not $hadImageBeforePull) {
            $previousErrorActionPreference = $ErrorActionPreference
            $ErrorActionPreference = "Continue"
            try {
                Invoke-DockerCli -Arguments @("image", "rm", $Item.Ref) 1>$null 2>$null
                $cleanupExitCode = $LASTEXITCODE
            } finally {
                $ErrorActionPreference = $previousErrorActionPreference
            }
            if ($cleanupExitCode -ne 0) {
                Write-Warning "清理失败镜像引用未完成: $($Item.Ref), exit=$cleanupExitCode"
            }
        } elseif ($hadImageBeforePull) {
            Write-Warning "保留本地已有 digest 镜像: $($Item.Ref)"
        }
        if ($attempt -lt $MaxAttempts -and $RetryDelaySeconds -gt 0) {
            Start-Sleep -Seconds $RetryDelaySeconds
        }
    }
    return $false
}

function Test-LocalImageRef {
    param([string]$Ref)
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        Invoke-DockerCli -Arguments @("image", "inspect", $Ref) 1>$null 2>$null
        return ($LASTEXITCODE -eq 0)
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
}

$rootPath = (Resolve-Path -LiteralPath $Root).Path
$manifests = Get-ChildItem -Path $rootPath -Recurse -Filter manifest.yaml | Sort-Object FullName
if ($PublishLocalBuilt) {
    Publish-LocalBuiltImages -Manifests $manifests -OutputLock $DigestLock
}
$digestLockItems = Read-DigestLock -Path $DigestLock
if ($PrintBuildArgs) {
    Write-Output (Get-BuildArgRef -DigestLockItems $digestLockItems -ImageName "base/go-builder" -ArgName "GO_BUILDER_IMAGE")
    Write-Output (Get-BuildArgRef -DigestLockItems $digestLockItems -ImageName "base/judge-min" -ArgName "JUDGE_MIN_IMAGE")
    exit 0
}
$items = New-Object System.Collections.Generic.List[object]
$seen = @{}
$missing = New-Object System.Collections.Generic.List[string]

foreach ($manifest in $manifests) {
    if (-not (Test-DeployableManifest -Path $manifest.FullName)) {
        continue
    }
    $sourceType = Read-SourceType -Path $manifest.FullName
    switch ($sourceType) {
        "upstream-pinned" {
            if ($Scope -in @("all", "upstream-pinned")) {
                Add-UpstreamRefs -ManifestPath $manifest.FullName -Items $items -Seen $seen -Missing $missing
            }
        }
        { $_ -in @("platform-built", "thin-wrapper", "build-base") } {
            if ($Scope -in @("all", "built")) {
                Add-BuiltRef -ManifestPath $manifest.FullName -DigestLockItems $digestLockItems -Items $items -Seen $seen -Missing $missing
            }
        }
        default {
            $missing.Add("$($manifest.FullName): source.type 非法或缺失")
        }
    }
}

if ($missing.Count -gt 0) {
    Write-Error ("Refusing to pull images because immutable digests are missing or invalid:`n" + ($missing -join "`n"))
    exit 1
}

$failures = New-Object System.Collections.Generic.List[string]
foreach ($item in $items) {
    if (Test-LocalImageRef -Ref $item.Ref) {
        Write-Host "Skipping existing image $($item.Ref)"
        continue
    }
    if (-not (Invoke-DockerPullWithRetry -Item $item)) {
        $failures.Add("$($item.Ref) ($($item.Manifest))")
        break
    }
}

if ($failures.Count -gt 0) {
    Write-Error ("Image pull failed after retries:`n" + ($failures -join "`n"))
    exit 1
}

Write-Host "Pulled $($items.Count) image references successfully. scope=$Scope registry=$Registry"
