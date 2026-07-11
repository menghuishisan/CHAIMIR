# 本脚本按当前 manifest 构建 Chaimir 自建、薄封装和构建基座镜像,并生成 Harbor digest 候选锁。
param(
    [string]$Root = (Split-Path -Parent $MyInvocation.MyCommand.Path),
    [string]$Registry = $env:CHAIMIR_IMAGE_REGISTRY,
    [string]$DockerConfig = $env:DOCKER_CONFIG,
    [string]$DigestLock = "",
    [string]$DigestLockOut = "",
    [string]$Tag = "",
    [string[]]$Images = @(),
    [string]$Platform = "linux/amd64",
    [string]$BuildxBuilder = $env:BUILDX_BUILDER,
    [int]$MaxAttempts = 3,
    [int]$RetryDelaySeconds = 10,
    [switch]$Push,
    [switch]$NoCache,
    [switch]$Pull
)

$ErrorActionPreference = "Stop"
Import-Module (Join-Path $PSScriptRoot "lib\ImageMetadata.psm1") -Force

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
if ([string]::IsNullOrWhiteSpace($DigestLockOut)) {
    $DigestLockOut = $DigestLock
}
if ([string]::IsNullOrWhiteSpace($Tag)) {
    throw "Tag 不能为空;本地和生产构建都必须显式声明不可混淆的发布标签"
}
if ($Push -and [string]::IsNullOrWhiteSpace($Platform)) {
    throw "启用 Push 时必须显式声明 Platform"
}
if ($MaxAttempts -lt 1) {
    throw "MaxAttempts 必须大于等于 1"
}

# Resolve-BuildPath 将 manifest build 路径解析为绝对路径。
function Resolve-BuildPath {
    param(
        [string]$RepoRoot,
        [string]$ManifestDir,
        [string]$Value
    )
    if ([string]::IsNullOrWhiteSpace($Value)) {
        $Value = "."
    }
    if ($Value -match "^images[\\/]") {
        return [System.IO.Path]::GetFullPath((Join-Path $RepoRoot $Value))
    }
    return [System.IO.Path]::GetFullPath((Join-Path $ManifestDir $Value))
}

# Get-LockedRef 返回构建参数需要的 Harbor digest 引用。
function Get-LockedRef {
    param(
        [hashtable]$DigestLockItems,
        [string]$Image
    )
    $digest = $DigestLockItems[$Image]
    if ([string]::IsNullOrWhiteSpace($digest)) {
        throw "digest 锁缺少 $Image,无法构建依赖它的镜像"
    }
    return "$Registry/$Image@$digest"
}

# Invoke-DockerCli 统一注入 Docker registry auth config。
function Invoke-DockerCli {
    param([string[]]$Arguments)
    $baseArguments = @()
    if (-not [string]::IsNullOrWhiteSpace($DockerConfig)) {
        $baseArguments += @("--config", $DockerConfig)
    }
    & docker @baseArguments @Arguments
}

# Get-DockerConfigBasicAuth 从 Docker config.json 读取 registry basic auth。
function Get-DockerConfigBasicAuth {
    param([string]$RegistryHost)
    if ([string]::IsNullOrWhiteSpace($DockerConfig)) {
        return $null
    }
    $configPath = Join-Path $DockerConfig "config.json"
    if (-not (Test-Path -LiteralPath $configPath)) {
        return $null
    }
    $config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
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

# Get-RemoteDigestFromRegistryV2 从 registry manifest header 读取不可变 digest。
function Get-RemoteDigestFromRegistryV2 {
    param([string]$TaggedRef)
    if ($TaggedRef -notmatch "^([^/]+)/(.+):([^/:]+)$") {
        return $null
    }
    $registryHost = $Matches[1]
    $repository = $Matches[2]
    $tag = $Matches[3]
    $scheme = "https"
    if ($registryHost -eq "harbor.chaimir" -or $registryHost.StartsWith("harbor.chaimir:")) {
        $scheme = "http"
    }
    $headers = @{
        Accept = "application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"
    }
    $basicAuth = Get-DockerConfigBasicAuth -RegistryHost $registryHost
    if (-not [string]::IsNullOrWhiteSpace($basicAuth)) {
        $headers["Authorization"] = "Basic $basicAuth"
    }
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Method Head -Uri "$scheme`://$registryHost/v2/$repository/manifests/$tag" -Headers $headers -TimeoutSec 30
        $digest = $response.Headers["Docker-Content-Digest"]
        if ($digest -match "^sha256:[0-9a-f]{64}$") {
            return $digest
        }
    } catch {
        return $null
    }
    return $null
}

# Get-RegistryDigest 解析刚推送到 Harbor 的 tag digest。
function Get-RegistryDigest {
    param([string]$Ref)
    $digest = Get-RemoteDigestFromRegistryV2 -TaggedRef $Ref
    if (-not [string]::IsNullOrWhiteSpace($digest)) {
        return $digest
    }
    $repository = $Ref
    if ($Ref -match "^(.+):[^/:]+$") {
        $repository = $Matches[1]
    }
    $localDigestOutput = Invoke-DockerCli -Arguments @("image", "inspect", "--format", "{{json .RepoDigests}}", $Ref) 2>&1
    foreach ($line in $localDigestOutput) {
        if ($line -match "$([regex]::Escape($repository))@(sha256:[0-9a-f]{64})") {
            return $Matches[1]
        }
    }
    $output = Invoke-DockerCli -Arguments @("buildx", "imagetools", "inspect", $Ref) 2>&1
    foreach ($line in $output) {
        if ($line -match "^\s*Digest:\s*(sha256:[0-9a-f]{64})\s*$") {
            return $Matches[1]
        }
    }
    foreach ($line in $output) {
        Write-Warning $line
    }
    throw "registry digest 输出不可解析: $Ref"
}

# Invoke-DockerBuildWithRetry 执行构建并按配置重试。
function Invoke-DockerBuildWithRetry {
    param(
        [string[]]$Arguments,
        [string]$Image
    )
    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        Write-Host "Docker build attempt [$attempt/$MaxAttempts] $Image"
        Invoke-DockerCli -Arguments $Arguments
        if ($LASTEXITCODE -eq 0) {
            return
        }
        $exitCode = $LASTEXITCODE
        Write-Warning "docker build 失败: $Image, attempt=$attempt, exit=$exitCode"
        if ($attempt -lt $MaxAttempts -and $RetryDelaySeconds -gt 0) {
            Start-Sleep -Seconds $RetryDelaySeconds
        }
    }
    throw "docker build 失败: $Image, exit=$LASTEXITCODE"
}

$rootPath = (Resolve-Path -LiteralPath $Root).Path
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $rootPath ".."))
$digestLockItems = Read-ChaimirDigestLock -Path $DigestLock
$manifests = Get-ChildItem -Path $rootPath -Recurse -Filter manifest.yaml | Sort-Object FullName
$selected = New-Object System.Collections.Generic.List[object]

foreach ($manifest in $manifests) {
    $sourceType = Get-ChaimirImageSourceType -Path $manifest.FullName
    if ($sourceType -notin @("platform-built", "thin-wrapper", "build-base")) {
        continue
    }
    $lines = Get-Content -LiteralPath $manifest.FullName
    $image = Get-ChaimirTopLevelYamlValue -Lines $lines -Key "image"
    if ([string]::IsNullOrWhiteSpace($image)) {
        throw "$($manifest.FullName): image 缺失"
    }
    if ($Images.Count -gt 0 -and $image -notin $Images) {
        continue
    }
    $build = Get-ChaimirYamlBlock -Path $manifest.FullName -BlockName "build"
    $contextValue = Get-ChaimirYamlValue -Lines $build -Key "context"
    $dockerfileValue = Get-ChaimirYamlValue -Lines $build -Key "dockerfile"
    if ([string]::IsNullOrWhiteSpace($dockerfileValue)) {
        $dockerfileValue = "Dockerfile"
    }
    $manifestDir = Split-Path -Parent $manifest.FullName
    $contextPath = Resolve-BuildPath -RepoRoot $repoRoot -ManifestDir $manifestDir -Value $contextValue
    if ($dockerfileValue -match "^images[\\/]") {
        $dockerfilePath = [System.IO.Path]::GetFullPath((Join-Path $repoRoot $dockerfileValue))
    } else {
        $dockerfilePath = [System.IO.Path]::GetFullPath((Join-Path $contextPath $dockerfileValue))
    }
    if (-not (Test-Path -LiteralPath $contextPath)) {
        throw "$image 构建上下文不存在: $contextPath"
    }
    if (-not (Test-Path -LiteralPath $dockerfilePath)) {
        throw "$image Dockerfile 不存在: $dockerfilePath"
    }
    $selected.Add([pscustomobject]@{
        Image = $image
        Context = $contextPath
        Dockerfile = $dockerfilePath
        DockerfileText = Get-Content -Raw -LiteralPath $dockerfilePath
        Ref = "$Registry/$image`:$Tag"
    })
}

foreach ($item in $selected) {
    if ($Push) {
        $args = @("buildx", "build")
        if (-not [string]::IsNullOrWhiteSpace($BuildxBuilder)) {
            $args += @("--builder", $BuildxBuilder)
        }
        $args += @("--platform", $Platform, "--push", "--provenance=false", "--sbom=false", "-f", $item.Dockerfile, "-t", $item.Ref)
    } else {
        $args = @("build", "-f", $item.Dockerfile, "-t", $item.Ref)
    }
    if ($NoCache) {
        $args += "--no-cache"
    }
    if ($Pull) {
        $args += "--pull"
    }
    if ($item.Image -ne "base/go-builder" -and $item.DockerfileText -match "(?m)^\s*ARG\s+GO_BUILDER_IMAGE\b") {
        $args += @("--build-arg", ("GO_BUILDER_IMAGE=" + (Get-LockedRef -DigestLockItems $digestLockItems -Image "base/go-builder")))
    }
    if ($item.Image -ne "base/chain-tools" -and $item.DockerfileText -match "(?m)^\s*ARG\s+CHAIN_TOOLS_IMAGE\b") {
        $args += @("--build-arg", ("CHAIN_TOOLS_IMAGE=" + (Get-LockedRef -DigestLockItems $digestLockItems -Image "base/chain-tools")))
    }
    if ($item.Image -ne "base/node-builder" -and $item.DockerfileText -match "(?m)^\s*ARG\s+NODE_BUILDER_IMAGE\b") {
        $args += @("--build-arg", ("NODE_BUILDER_IMAGE=" + (Get-LockedRef -DigestLockItems $digestLockItems -Image "base/node-builder")))
    }
    if ($item.DockerfileText -match "(?m)^\s*ARG\s+JUDGE_MIN_IMAGE\b") {
        $args += @("--build-arg", ("JUDGE_MIN_IMAGE=" + (Get-LockedRef -DigestLockItems $digestLockItems -Image "base/judge-min")))
    }
    if ($item.DockerfileText -match "(?m)^\s*ARG\s+GO_MODULE_PROXY\b" -and -not [string]::IsNullOrWhiteSpace($env:GO_MODULE_PROXY)) {
        $args += @("--build-arg", ("GO_MODULE_PROXY=" + $env:GO_MODULE_PROXY))
    }
    $args += $item.Context
    Write-Host "Building $($item.Image) -> $($item.Ref)"
    Invoke-DockerBuildWithRetry -Arguments $args -Image $item.Image
    if ($Push) {
        $digestLockItems[$item.Image] = Get-RegistryDigest -Ref $item.Ref
        Write-ChaimirDigestLock -Path $DigestLockOut -Items $digestLockItems
        Write-Host "Locked $($item.Image) $($digestLockItems[$item.Image])"
    }
}

if ($Push) {
    Write-Host "Built and pushed $($selected.Count) image(s). registry=$Registry tag=$Tag digestLock=$DigestLockOut"
} else {
    Write-Host "Built $($selected.Count) image(s). registry=$Registry tag=$Tag"
}
