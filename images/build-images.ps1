# 本脚本按当前 manifest 构建 Chaimir 自建、薄封装和构建基座镜像,并生成 Harbor digest 候选锁。
param(
    [string]$Root = (Split-Path -Parent $MyInvocation.MyCommand.Path),
    [string]$Registry = $env:CHAIMIR_IMAGE_REGISTRY,
    [string]$RegistryEndpoint = $env:SUPPLY_CHAIN_REGISTRY_ENDPOINT,
    [string]$DockerConfig = $env:DOCKER_CONFIG,
    [string]$DigestLock = "",
    [string]$DigestLockOut = "",
    [string]$Tag = "",
    [string[]]$Images = @(),
    [string]$Platform = "linux/amd64",
    [string]$BuildxBuilder = $env:BUILDX_BUILDER,
    [string]$BuildNetwork = $env:SUPPLY_CHAIN_BUILD_NETWORK,
    [int]$MaxAttempts = 3,
    [int]$RetryDelaySeconds = 10,
    [switch]$NoCache
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
    throw "必须通过 -Registry 或 CHAIMIR_IMAGE_REGISTRY/SUPPLY_CHAIN_REGISTRY/IMAGE_REGISTRY 指定生产 Harbor registry"
}
if ([string]::IsNullOrWhiteSpace($RegistryEndpoint)) {
    $RegistryEndpoint = $Registry
}
if ([string]::IsNullOrWhiteSpace($DigestLock)) {
    $DigestLock = Join-Path $Root "image-digests.lock"
}
if ([string]::IsNullOrWhiteSpace($DigestLockOut)) {
    $DigestLockOut = $DigestLock
}
if ([string]::IsNullOrWhiteSpace($Tag)) {
    throw "Tag 不能为空;生产候选构建必须显式声明发布标签"
}
if ([string]::IsNullOrWhiteSpace($Platform)) {
    throw "生产候选构建必须显式声明 Platform"
}
if ($MaxAttempts -lt 1) {
    throw "MaxAttempts 必须大于等于 1"
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
    return "$RegistryEndpoint/$Image@$digest"
}

# Get-PinnedUpstreamRef 从 manifest 读取上游镜像与 digest,拒绝可变或不完整的运行基座。
function Get-PinnedUpstreamRef {
    param(
        [string]$ManifestPath,
        [string]$ImageKey,
        [string]$DigestKey
    )
    $upstream = Get-ChaimirYamlBlock -Path $ManifestPath -BlockName "upstream"
    $image = Get-ChaimirYamlValue -Lines $upstream -Key $ImageKey
    $digest = Get-ChaimirYamlValue -Lines $upstream -Key $DigestKey
    if ([string]::IsNullOrWhiteSpace($image) -or $digest -notmatch "^sha256:[0-9a-f]{64}$") {
        throw "$ManifestPath`: upstream.$ImageKey 与 upstream.$DigestKey 必须组成不可变镜像引用"
    }
    if ($image.Contains("@")) {
        throw "$ManifestPath`: upstream.$ImageKey 只填写镜像与版本,digest 必须单独写入 upstream.$DigestKey"
    }
    return "$image@$digest"
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
    $registryEndpointForRequest = $RegistryEndpoint
    if ([string]::IsNullOrWhiteSpace($registryEndpointForRequest)) {
        $registryEndpointForRequest = $registryHost
    }
    if ($registryEndpointForRequest -match "^https?://") {
        $registryBaseUri = $registryEndpointForRequest.TrimEnd('/')
    } else {
        $registryBaseUri = "$scheme`://$($registryEndpointForRequest.TrimEnd('/'))"
    }
    $headers = @{
        Accept = "application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"
    }
    $basicAuth = Get-DockerConfigBasicAuth -RegistryHost $registryHost
    if (-not [string]::IsNullOrWhiteSpace($basicAuth)) {
        $headers["Authorization"] = "Basic $basicAuth"
    }
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Method Head -Uri "$registryBaseUri/v2/$repository/manifests/$tag" -Headers $headers -TimeoutSec 30
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
$catalog = Get-ChaimirImageCatalog -ImagesRoot $rootPath
$manifests = Get-ChildItem -Path $rootPath -Recurse -Filter manifest.yaml | Sort-Object FullName
$selected = New-Object System.Collections.Generic.List[object]
$lockedArgumentImages = Get-ChaimirInternalImageBuildArguments

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
    if (-not $catalog[$image].Deployable) {
        Write-Host "Skipping non-deployable image $image ($($catalog[$image].BlockReason))"
        continue
    }
    if ($Images.Count -gt 0 -and $image -notin $Images) {
        continue
    }
    $build = Get-ChaimirYamlBlock -Path $manifest.FullName -BlockName "build"
    $contextValue = Get-ChaimirYamlValue -Lines $build -Key "context"
    $dockerfileValue = Get-ChaimirYamlValue -Lines $build -Key "dockerfile"
    $buildPaths = Resolve-ChaimirImageBuildPaths -RepoRoot $repoRoot -ManifestPath $manifest.FullName -ContextValue $contextValue -DockerfileValue $dockerfileValue
    $contextPath = $buildPaths.Context
    $dockerfilePath = $buildPaths.Dockerfile
    if (-not (Test-Path -LiteralPath $contextPath)) {
        throw "$image 构建上下文不存在: $contextPath"
    }
    if (-not (Test-Path -LiteralPath $dockerfilePath)) {
        throw "$image Dockerfile 不存在: $dockerfilePath"
    }
    $dockerfileText = Get-Content -Raw -LiteralPath $dockerfilePath
    $dependencies = [System.Collections.Generic.List[string]]::new()
    foreach ($argumentName in $lockedArgumentImages.Keys) {
        $logicalDependency = $lockedArgumentImages[$argumentName]
        if ($image -ne $logicalDependency -and $dockerfileText -match "(?m)^\s*ARG\s+$argumentName\b") {
            $dependencies.Add($logicalDependency)
        }
    }
    $selected.Add([pscustomobject]@{
        Image = $image
        Manifest = $manifest.FullName
        Context = $contextPath
        Dockerfile = $dockerfilePath
        DockerfileText = $dockerfileText
        Dependencies = $dependencies.ToArray()
        Ref = "$RegistryEndpoint/$image`:$Tag"
    })
}

# 构建内部依赖必须先于消费者产出新 digest;目录字母顺序不能表达该约束。
$selectedByImage = @{}
foreach ($item in $selected) {
    $selectedByImage[$item.Image] = $item
}
$remaining = [System.Collections.Generic.HashSet[string]]::new([string[]]@($selectedByImage.Keys))
$ordered = [System.Collections.Generic.List[object]]::new()
while ($remaining.Count -gt 0) {
    $progress = $false
    foreach ($image in @($remaining | Sort-Object)) {
        $item = $selectedByImage[$image]
        $unresolved = @($item.Dependencies | Where-Object { $selectedByImage.ContainsKey($_) -and $remaining.Contains($_) })
        if ($unresolved.Count -gt 0) {
            continue
        }
        $ordered.Add($item)
        [void]$remaining.Remove($image)
        $progress = $true
    }
    if (-not $progress) {
        throw "镜像内部构建依赖存在循环: $((@($remaining) | Sort-Object) -join ',')"
    }
}
$selected = $ordered

foreach ($item in $selected) {
    $args = @("buildx", "build")
    if (-not [string]::IsNullOrWhiteSpace($BuildxBuilder)) {
        $args += @("--builder", $BuildxBuilder)
    }
    $args += @("--platform", $Platform, "--push", "--provenance=false", "--sbom=false", "-f", $item.Dockerfile, "-t", $item.Ref)
    if ($RegistryEndpoint -match ":\d+$") {
        $registryHost = ($RegistryEndpoint -replace "^https?://", "").Split("/")[0].Split(":")[0]
        $args += @("--add-host", "$registryHost`:host-gateway")
    }
    if ($NoCache) {
        $args += "--no-cache"
    }
    $buildArguments = @{}
    foreach ($argumentName in $lockedArgumentImages.Keys) {
        $logicalImage = $lockedArgumentImages[$argumentName]
        if ($item.Image -ne $logicalImage -and $item.DockerfileText -match "(?m)^\s*ARG\s+$argumentName\b") {
            $buildArguments[$argumentName] = Get-LockedRef -DigestLockItems $digestLockItems -Image $logicalImage
        }
    }
    if ($item.DockerfileText -match "(?m)^\s*ARG\s+NGINX_RUNTIME_IMAGE\b") {
        $buildArguments["NGINX_RUNTIME_IMAGE"] = Get-PinnedUpstreamRef -ManifestPath $item.Manifest -ImageKey "runtime" -DigestKey "runtime_digest"
    }
    if ($item.Image -eq "service/frontend") {
        $frontendEnvPath = Join-Path $repoRoot "frontend\.env"
        if (-not (Test-Path -LiteralPath $frontendEnvPath)) {
            throw "service/frontend 构建缺少前端构建配置: $frontendEnvPath"
        }
        foreach ($line in Get-Content -LiteralPath $frontendEnvPath) {
            if ($line -match '^\s*(VITE_[A-Z0-9_]+)\s*=(.*)$') {
                $buildArguments[$Matches[1]] = $Matches[2].Trim()
            }
        }
        $requiredFrontendArgs = @(
            "VITE_API_BASE_URL",
            "VITE_API_TIMEOUT_MS",
            "VITE_SIM_WORKER_COMMAND_TIMEOUT_MS",
            "VITE_SIM_STEP_INTERVAL_MS"
        )
        foreach ($argumentName in $requiredFrontendArgs) {
            if (-not $buildArguments.ContainsKey($argumentName) -or [string]::IsNullOrWhiteSpace([string]$buildArguments[$argumentName])) {
                throw "service/frontend 构建配置缺少 $argumentName"
            }
        }
    }
    if ($item.DockerfileText -match "(?m)^\s*ARG\s+GO_MODULE_PROXY\b" -and -not [string]::IsNullOrWhiteSpace($env:GO_MODULE_PROXY)) {
        $buildArguments["GO_MODULE_PROXY"] = $env:GO_MODULE_PROXY
    }
    if ($item.DockerfileText -match "(?m)^\s*ARG\s+PIP_INDEX_URL\b" -and -not [string]::IsNullOrWhiteSpace($env:PIP_INDEX_URL)) {
        $buildArguments["PIP_INDEX_URL"] = $env:PIP_INDEX_URL
    }
    if ($item.DockerfileText -match "(?m)^\s*ARG\s+NPM_CONFIG_REGISTRY\b" -and -not [string]::IsNullOrWhiteSpace($env:NPM_CONFIG_REGISTRY)) {
        $buildArguments["NPM_CONFIG_REGISTRY"] = $env:NPM_CONFIG_REGISTRY
    }
    foreach ($key in $buildArguments.Keys | Sort-Object) {
        $args += @("--build-arg", "$key=$($buildArguments[$key])")
    }
    if ($BuildNetwork -ne "host") {
        $proxyBuildArgs = @(
            @('HTTP_PROXY', $env:SUPPLY_CHAIN_HTTP_PROXY),
            @('HTTPS_PROXY', $env:SUPPLY_CHAIN_HTTPS_PROXY),
            @('NO_PROXY', $env:SUPPLY_CHAIN_NO_PROXY),
            @('http_proxy', $env:SUPPLY_CHAIN_HTTP_PROXY),
            @('https_proxy', $env:SUPPLY_CHAIN_HTTPS_PROXY),
            @('no_proxy', $env:SUPPLY_CHAIN_NO_PROXY)
        )
        foreach ($proxyPair in $proxyBuildArgs) {
            if (-not [string]::IsNullOrWhiteSpace($proxyPair[1])) {
                $args += @('--build-arg', "$($proxyPair[0])=$($proxyPair[1])")
            }
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($BuildNetwork)) {
        $args += @("--network", $BuildNetwork)
    }
    $args += $item.Context
    Write-Host "Building $($item.Image) -> $($item.Ref)"
    Invoke-DockerBuildWithRetry -Arguments $args -Image $item.Image
    $digestLockItems[$item.Image] = Get-RegistryDigest -Ref $item.Ref
    Write-ChaimirDigestLock -Path $DigestLockOut -Items $digestLockItems
    Write-Host "Locked $($item.Image) $($digestLockItems[$item.Image])"
}

$cleanupArgs = @(
    "-RepoRoot", $repoRoot,
    "-Registry", $Registry,
    "-DigestLockPath", $DigestLock,
    "-CandidateDigestLockPath", $DigestLockOut,
    "-KeepTags", $Tag,
    "-Apply"
)
& powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "cleanup-local-images.ps1") @cleanupArgs
if ($LASTEXITCODE -ne 0) {
    throw "构建成功但项目镜像清理失败,拒绝继续: exit=$LASTEXITCODE"
}
Write-Host "Built and pushed $($selected.Count) image(s). registry=$Registry registry_endpoint=$RegistryEndpoint tag=$Tag digestLock=$DigestLockOut"
