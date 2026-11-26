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
    [string[]]$CleanupCandidateDigestLockPaths = @(),
    [string[]]$CleanupKeepTags = @(),
    [string]$Platform = "linux/amd64",
    [string]$BuildxBuilder = $env:BUILDX_BUILDER,
    [string]$BuildNetwork = $env:SUPPLY_CHAIN_BUILD_NETWORK,
    [int]$MaxAttempts = 3,
    [int]$RetryDelaySeconds = 10,
    [int]$BuildTimeoutSeconds = 1800,
    [switch]$NoCache
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

# 从项目配置加载供应链网络参数,避免构建时遗漏代理导致依赖重复下载。
$projectConfigPath = Join-Path (Split-Path -Parent $PSScriptRoot) "deploy\config\chaimir.env"
$projectConfig = @{}
if (Test-Path -LiteralPath $projectConfigPath) {
    foreach ($line in Get-Content -LiteralPath $projectConfigPath) {
        if ($line -match '^\s*([A-Za-z_][A-Za-z0-9_]*)=(.*)\s*$') {
            $projectConfig[$Matches[1]] = $Matches[2]
        }
    }
    foreach ($key in @("SUPPLY_CHAIN_REGISTRY", "SUPPLY_CHAIN_REGISTRY_ENDPOINT", "SUPPLY_CHAIN_HTTP_PROXY", "SUPPLY_CHAIN_HTTPS_PROXY", "SUPPLY_CHAIN_NO_PROXY", "GO_MODULE_PROXY")) {
        $currentValue = [Environment]::GetEnvironmentVariable($key)
        if ([string]::IsNullOrWhiteSpace($currentValue) -and $projectConfig.ContainsKey($key)) {
            [Environment]::SetEnvironmentVariable($key, $projectConfig[$key], "Process")
        }
    }
    if ([string]::IsNullOrWhiteSpace($DockerConfig) -and $projectConfig.ContainsKey("SUPPLY_CHAIN_DOCKER_CONFIG_HOST_DIR")) {
        $dockerConfigValue = $projectConfig["SUPPLY_CHAIN_DOCKER_CONFIG_HOST_DIR"]
        $DockerConfig = if ([IO.Path]::IsPathRooted($dockerConfigValue)) {
            $dockerConfigValue
        } else {
            Join-Path (Join-Path $repoRoot "deploy") $dockerConfigValue
        }
    }
}
if (-not [string]::IsNullOrWhiteSpace($env:GO_MODULE_PROXY)) {
    $env:GO_MODULE_PROXY = $env:GO_MODULE_PROXY -replace '\|', ','
}

# 供应链 Registry 使用项目维护的 canonical CA;客户端必须显式校验证书链,不得退回 insecure。
$registryCaHostPath = $env:SUPPLY_CHAIN_REGISTRY_CA_HOST_PATH
if ([string]::IsNullOrWhiteSpace($registryCaHostPath) -and $projectConfig.ContainsKey("SUPPLY_CHAIN_REGISTRY_CA_HOST_PATH")) {
    $registryCaHostPath = $projectConfig["SUPPLY_CHAIN_REGISTRY_CA_HOST_PATH"]
}
if (-not [string]::IsNullOrWhiteSpace($registryCaHostPath) -and -not [IO.Path]::IsPathRooted($registryCaHostPath)) {
    $registryCaHostPath = Join-Path (Join-Path $repoRoot "deploy") $registryCaHostPath
}
if ([string]::IsNullOrWhiteSpace($registryCaHostPath) -or -not (Test-Path -LiteralPath $registryCaHostPath -PathType Leaf)) {
    throw "缺少供应链 Registry canonical CA,拒绝在未校验证书链的情况下构建: $registryCaHostPath"
}
if ($BuildTimeoutSeconds -le 0) {
    throw "BuildTimeoutSeconds 必须为正数"
}

Import-Module (Join-Path $PSScriptRoot "lib\ImageMetadata.psm1") -Force
Assert-ChaimirImageSelection -Images $Images


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

# Invoke-DockerCli 统一注入 Docker registry auth config。
function Invoke-DockerCli {
    param([string[]]$Arguments)
    $baseArguments = @()
    if (-not [string]::IsNullOrWhiteSpace($DockerConfig)) {
        $baseArguments += @("--config", $DockerConfig)
    }
    & docker @baseArguments @Arguments
}

function Invoke-DockerCliWithTimeout {
    param([string[]]$Arguments, [int]$TimeoutSeconds)
    if (-not [string]::IsNullOrWhiteSpace($DockerConfig)) {
        $Arguments = @('--config', $DockerConfig) + $Arguments
    }
    Invoke-ChaimirDocker -Arguments $Arguments -TimeoutSeconds $TimeoutSeconds
}

# Get-DockerConfigBasicAuth 从 Docker config.json 读取 registry basic auth。
function Get-DockerConfigBasicAuth {
    param([string]$RegistryHost)
    $effectiveDockerConfig = $DockerConfig
    if ([string]::IsNullOrWhiteSpace($effectiveDockerConfig)) {
        $effectiveDockerConfig = Join-Path $env:USERPROFILE ".docker"
    }
    $configPath = Join-Path $effectiveDockerConfig "config.json"
    if (-not (Test-Path -LiteralPath $configPath)) {
        return $null
    }
    $config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
    if ($config.auths) {
        foreach ($property in $config.auths.PSObject.Properties) {
            $name = $property.Name
            if ($name -eq $RegistryHost -or $name -eq "http://$RegistryHost" -or $name -eq "https://$RegistryHost") {
                $auth = $property.Value.auth
                if (-not [string]::IsNullOrWhiteSpace($auth)) {
                    return $auth
                }
            }
        }
    }

    # Docker Desktop 默认把凭据交给 credential helper,不能只读取 config.json.auths。
    $helperName = ""
    if ($config.credHelpers) {
        foreach ($property in $config.credHelpers.PSObject.Properties) {
            $name = $property.Name
            if ($name -eq $RegistryHost -or $name -eq "http://$RegistryHost" -or $name -eq "https://$RegistryHost") {
                $helperName = [string]$property.Value
                break
            }
        }
    }
    if ([string]::IsNullOrWhiteSpace($helperName)) {
        $helperName = [string]$config.credsStore
    }
    if ([string]::IsNullOrWhiteSpace($helperName)) {
        return $null
    }
    $helper = Get-Command "docker-credential-$helperName" -ErrorAction SilentlyContinue
    if (-not $helper) {
        return $null
    }
    $savedErrorActionPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = "SilentlyContinue"
        $credentialOutput = @($RegistryHost | & $helper.Source get 2>$null)
        $credentialExitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $savedErrorActionPreference
    }
    if ($credentialExitCode -ne 0 -or $credentialOutput.Count -eq 0) {
        return $null
    }
    try {
        $credential = ($credentialOutput -join "`n") | ConvertFrom-Json
    } catch {
        return $null
    }
    $username = [string]$credential.Username
    $secret = [string]$credential.Secret
    if ([string]::IsNullOrWhiteSpace($username) -or [string]::IsNullOrWhiteSpace($secret)) {
        return $null
    }
    return [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes("$username`:$secret"))
}

# Get-RemoteDigestFromRegistryV2 使用 canonical CA 从 Registry manifest header 读取不可变 digest。
function Get-RemoteDigestFromRegistryV2 {
    param([string]$TaggedRef)
    if ($TaggedRef -notmatch "^([^/]+)/(.+):([^/:]+)$") {
        throw "待解析镜像引用必须是 registry/repository:tag: $TaggedRef"
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
    $basicCredential = ""
    if (-not [string]::IsNullOrWhiteSpace($basicAuth)) {
        try { $basicCredential = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($basicAuth)) } catch { throw "供应链 Registry Docker 凭据不是合法 Basic auth 编码" }
        if ($basicCredential -notmatch '^[^:]+:.+$') { throw "供应链 Registry Docker 凭据格式无效" }
    }
    $curlArguments = @(
        "--silent", "--show-error", "--head", "--connect-timeout", "10", "--max-time", "30",
        "--cacert", $registryCaHostPath,
        "--header", "Accept: $($headers["Accept"])"
    )
    if (-not [string]::IsNullOrWhiteSpace($basicCredential)) { $curlArguments += @("--user", $basicCredential) }
    $curlArguments += "$registryBaseUri/v2/$repository/manifests/$tag"
    $lastResponse = ""
    $bearerToken = ""
    for ($attempt = 1; $attempt -le 6; $attempt++) {
        $savedCurlPreference = $ErrorActionPreference
        try {
            $ErrorActionPreference = "Continue"
            $responseLines = @(& curl.exe @curlArguments 2>&1)
            $curlExitCode = $LASTEXITCODE
        } finally { $ErrorActionPreference = $savedCurlPreference }
        $headerText = ($responseLines | ForEach-Object { [string]$_ }) -join "`n"
        $digestMatch = [regex]::Match($headerText, '(?im)^Docker-Content-Digest:\s*(sha256:[0-9a-f]{64})\s*$')
        if ($curlExitCode -eq 0 -and $digestMatch.Success) { return $digestMatch.Groups[1].Value }
        if ([string]::IsNullOrWhiteSpace($bearerToken)) {
            $authMatch = [regex]::Match($headerText, '(?im)WWW-Authenticate:\s*Bearer\s+realm="([^"]+)"[^\r\n]*service="([^"]+)"[^\r\n]*scope="([^"]+)"')
            if ($authMatch.Success) {
                $tokenUri = $authMatch.Groups[1].Value + "?service=" + [uri]::EscapeDataString($authMatch.Groups[2].Value) + "&scope=" + [uri]::EscapeDataString($authMatch.Groups[3].Value)
                $tokenOutput = @(& curl.exe --silent --show-error --connect-timeout 10 --max-time 30 --cacert $registryCaHostPath $tokenUri 2>&1)
                $tokenExitCode = $LASTEXITCODE
                if ($tokenExitCode -ne 0) { throw "供应链 Registry Bearer token 获取失败: $($tokenOutput -join ' ')" }
                try { $tokenPayload = ($tokenOutput -join "`n") | ConvertFrom-Json } catch { throw "供应链 Registry Bearer token 响应不是合法 JSON" }
                $bearerToken = if (-not [string]::IsNullOrWhiteSpace([string]$tokenPayload.token)) { [string]$tokenPayload.token } else { [string]$tokenPayload.access_token }
                if ([string]::IsNullOrWhiteSpace($bearerToken)) { throw "供应链 Registry Bearer token 响应缺少 token" }
                $curlArguments += @("--header", "Authorization: Bearer $bearerToken")
                continue
            }
        }
        $lastResponse = (($responseLines | ForEach-Object { [string]$_ }) -join " ").Trim()
        if ($attempt -lt 6) { Start-Sleep -Seconds 2 }
    }
    $authState = if (-not [string]::IsNullOrWhiteSpace($basicCredential)) { "present" } else { "missing" }
    throw "读取刚推送镜像的 Registry digest 失败: $TaggedRef; auth=$authState; $lastResponse"
}

# Get-RegistryDigest 解析刚推送到 Harbor 的 tag digest。
function Get-RegistryDigest {
    param([string]$Ref)
    return Get-RemoteDigestFromRegistryV2 -TaggedRef $Ref
}

# Invoke-DockerBuildWithRetry 只重试同一 builder 的完整构建推送,失败不发布本机同名旧镜像。
function Invoke-DockerBuildWithRetry {
    param(
        [string[]]$Arguments,
        [string]$Image
    )
    $exitCode = 1
    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        Write-Host "Docker build attempt [$attempt/$MaxAttempts] $Image"
        # 远端 builder 的导出日志不证明本机 tag 属于本次构建,只接受本次命令成功退出。
        $buildResult = Invoke-DockerCliWithTimeout -Arguments $Arguments -TimeoutSeconds $BuildTimeoutSeconds
        $buildExitCode = $buildResult.Code
        if (-not [string]::IsNullOrWhiteSpace($buildResult.Text)) { Write-Host $buildResult.Text }
        if ($buildExitCode -eq 0) {
            return
        }
        $exitCode = $buildExitCode
        Write-Warning "docker build 失败: $Image, attempt=$attempt, exit=$exitCode"

        if ($attempt -lt $MaxAttempts -and $RetryDelaySeconds -gt 0) {
            Start-Sleep -Seconds $RetryDelaySeconds
        }
    }
    throw "docker build 失败: $Image, exit=$exitCode"
}

$rootPath = (Resolve-Path -LiteralPath $Root).Path
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $rootPath ".."))
$digestLockItems = Read-ChaimirDigestLock -Path $DigestLock
$catalog = Get-ChaimirImageCatalog -ImagesRoot $rootPath
$selected = [System.Collections.Generic.List[object]]::new()
foreach ($entry in ($catalog.Values | Sort-Object Image)) {
    if ($entry.SourceType -notin @('platform-built', 'thin-wrapper', 'build-base')) { continue }
    $image = $entry.Image
    if (-not $entry.Deployable) {
        Write-Host "Skipping non-deployable image $image ($($entry.BlockReason))"
        continue
    }
    if ($Images.Count -gt 0 -and $image -notin $Images) {
        continue
    }
    $build = Get-ChaimirYamlBlock -Path $entry.Manifest -BlockName "build"
    $contextValue = Get-ChaimirYamlValue -Lines $build -Key "context"
    $dockerfileValue = Get-ChaimirYamlValue -Lines $build -Key "dockerfile"
    $buildPaths = Resolve-ChaimirImageBuildPaths -RepoRoot $repoRoot -ManifestPath $entry.Manifest -ContextValue $contextValue -DockerfileValue $dockerfileValue
    $contextPath = $buildPaths.Context
    $dockerfilePath = $buildPaths.Dockerfile
    if (-not (Test-Path -LiteralPath $contextPath)) {
        throw "$image 构建上下文不存在: $contextPath"
    }
    if (-not (Test-Path -LiteralPath $dockerfilePath)) {
        throw "$image Dockerfile 不存在: $dockerfilePath"
    }
    $dockerfileText = Get-Content -Raw -LiteralPath $dockerfilePath
    $imageSources = Get-ChaimirImageBuildSources -ManifestPath $entry.Manifest -DockerfileText $dockerfileText
    $dependencies = @($imageSources.Values | Where-Object { -not $_.Contains('@') })
    $selected.Add([pscustomobject]@{
        Image = $image
        Context = $contextPath
        Dockerfile = $dockerfilePath
        DockerfileText = $dockerfileText
        ImageSources = $imageSources
        Dependencies = $dependencies
        Ref = "$RegistryEndpoint/$image`:$Tag"
    })
}

# 先构建本批次里的依赖；未选中的镜像继续使用 digest 锁。
$selectedByImage = @{}
$dependenciesByImage = @{}
foreach ($item in $selected) {
    $selectedByImage[$item.Image] = $item
    $dependenciesByImage[$item.Image] = $item.Dependencies
}
$selected = @(foreach ($wave in (Get-ChaimirImageBuildWaves -Dependencies $dependenciesByImage)) {
    foreach ($image in $wave.Images) { $selectedByImage[$image] }
})

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
    $buildArguments = Resolve-ChaimirImageBuildArguments -Sources $item.ImageSources -Registry $RegistryEndpoint -DigestLock $digestLockItems
    if ($item.Image -eq "service/frontend") {
        $frontendEnvPath = Join-Path $repoRoot "frontend\.env"
        $frontendArguments = Get-ChaimirFrontendBuildArguments -Path $frontendEnvPath -DockerfileText $item.DockerfileText
        foreach ($key in $frontendArguments.Keys) { $buildArguments[$key] = $frontendArguments[$key] }
        # 将前端源码内容摘要作为构建参数,避免未提交改动被 BuildKit 错误复用旧的源码缓存层。
        $frontendRelativeFiles = @(& git -C $repoRoot ls-files --cached --others --exclude-standard -- "frontend/*")
        if ($LASTEXITCODE -ne 0) {
            throw "读取前端源码清单失败"
        }
        $frontendFingerprintLines = @(
            $frontendRelativeFiles |
                Sort-Object |
                ForEach-Object {
                    $relativePath = $_ -replace '/', '\\'
                    $absolutePath = Join-Path $repoRoot $relativePath
                    if (-not (Test-Path -LiteralPath $absolutePath -PathType Leaf)) {
                        throw "前端源码文件不存在: $relativePath"
                    }
                    "$relativePath|$((Get-FileHash -LiteralPath $absolutePath -Algorithm SHA256).Hash)"
                }
        )
        $frontendFingerprintBytes = [System.Text.Encoding]::UTF8.GetBytes(($frontendFingerprintLines -join "`n"))
        $frontendHasher = [System.Security.Cryptography.SHA256]::Create()
        try {
            $buildArguments["CHAIMIR_FRONTEND_SOURCE_DIGEST"] = ([BitConverter]::ToString($frontendHasher.ComputeHash($frontendFingerprintBytes)).Replace('-', '')).ToLowerInvariant()
        } finally {
            $frontendHasher.Dispose()
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

$formalDigestLock = Join-Path $repoRoot "images\image-digests.lock"
$cleanupCandidateLocks = @($DigestLock, $DigestLockOut) + @($CleanupCandidateDigestLockPaths) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique
$cleanupKeepTags = @($Tag) + @($CleanupKeepTags) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique
& powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "cleanup-local-images.ps1") `
    -RepoRoot $repoRoot `
    -Registry $Registry `
    -DigestLockPath $formalDigestLock `
    -CandidateDigestLockPath ($cleanupCandidateLocks -join ',') `
    -KeepTags ($cleanupKeepTags -join ',') `
    -Apply
if ($LASTEXITCODE -ne 0) {
    throw "构建成功但项目镜像清理失败,拒绝继续: exit=$LASTEXITCODE"
}
Write-Host "Built and pushed $($selected.Count) image(s). registry=$Registry registry_endpoint=$RegistryEndpoint tag=$Tag digestLock=$DigestLockOut"
