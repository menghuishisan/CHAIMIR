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

# 兼容文档中以逗号传入多个镜像的调用方式,避免参数被 PowerShell 当成单个镜像名而静默跳过构建。
if ($Images.Count -eq 1 -and $Images[0] -match ',') {
    $Images = @($Images[0].Split(',') | ForEach-Object { $_.Trim() } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
}

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

# ConvertTo-ProcessArgument 按 Windows 原生参数规则转义单个参数,供有界 Docker 进程调用复用。
function ConvertTo-ProcessArgument([string]$Value) {
    if ($null -eq $Value -or $Value.Length -eq 0) { return '""' }
    if ($Value -notmatch '[\s"]') { return $Value }
    $builder = [Text.StringBuilder]::new()
    [void]$builder.Append('"')
    $slashes = 0
    foreach ($character in $Value.ToCharArray()) {
        if ($character -eq '\') { $slashes++; continue }
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

# Invoke-DockerCliWithTimeout 让构建在 Docker daemon 或网络无响应时有界返回,不允许无限等待。
function Invoke-DockerCliWithTimeout {
    param(
        [string[]]$Arguments,
        [int]$TimeoutSeconds
    )
    $baseArguments = @()
    if (-not [string]::IsNullOrWhiteSpace($DockerConfig)) {
        $baseArguments += @('--config', $DockerConfig)
    }
    $cliArguments = @($baseArguments + $Arguments)
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = [Diagnostics.ProcessStartInfo]::new()
    $process.StartInfo.FileName = 'docker.exe'
    $process.StartInfo.Arguments = (($cliArguments | ForEach-Object { ConvertTo-ProcessArgument ([string]$_) }) -join ' ')
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.CreateNoWindow = $true
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    $process.StartInfo.StandardOutputEncoding = [Text.Encoding]::UTF8
    $process.StartInfo.StandardErrorEncoding = [Text.Encoding]::UTF8
    try {
        if (-not $process.Start()) { throw 'Docker CLI 启动失败' }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
            # buildx 可能由 Docker CLI 派生子进程,只终止父进程会留下无主构建任务。
            try { & taskkill.exe /PID $process.Id /T /F | Out-Null } catch {}
            try { $process.Kill($true) } catch { try { $process.Kill() } catch {} }
            $stdout = $stdoutTask.GetAwaiter().GetResult()
            $stderr = $stderrTask.GetAwaiter().GetResult()
            return [pscustomobject]@{
                Code = 124
                Text = ("Docker build 超时（{0} 秒）: {1}`n{2}`n{3}" -f $TimeoutSeconds, ($cliArguments -join ' '), $stdout, $stderr).Trim()
            }
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        return [pscustomobject]@{ Code = $process.ExitCode; Text = ((@($stdout, $stderr) | Where-Object { $_ }) -join "`n").Trim() }
    } finally {
        $process.Dispose()
    }
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
    $upstream = Get-ChaimirYamlBlock -Path $item.Manifest -BlockName "upstream"
    $buildBaseArgument = Get-ChaimirYamlValue -Lines $upstream -Key "build_base_arg"
    if (-not [string]::IsNullOrWhiteSpace($buildBaseArgument)) {
        if ($buildBaseArgument -notmatch "^[A-Z0-9_]+_IMAGE$") {
            throw "$($item.Image): upstream.build_base_arg 必须是 *_IMAGE Dockerfile ARG"
        }
        if ($item.DockerfileText -notmatch "(?m)^\s*ARG\s+$([regex]::Escape($buildBaseArgument))\b") {
            throw "$($item.Image): upstream.build_base_arg 未在 Dockerfile 声明: $buildBaseArgument"
        }
        # 内部构建基座必须跟随本次候选 digest 锁;外部构建基座必须直接使用 manifest
        # 中单独锁定的不可变引用,两者都不能退回可变 tag。
        $buildBaseImage = Get-ChaimirYamlValue -Lines $upstream -Key "build_base_image"
        $logicalBuildBaseImage = $lockedArgumentImages[$buildBaseArgument]
        if (-not [string]::IsNullOrWhiteSpace($logicalBuildBaseImage)) {
            $buildArguments[$buildBaseArgument] = Get-LockedRef -DigestLockItems $digestLockItems -Image $logicalBuildBaseImage
        } else {
            if ([string]::IsNullOrWhiteSpace($buildBaseImage)) {
                throw "$($item.Image): upstream.build_base_image 不能为空: $buildBaseArgument"
            }
            $buildArguments[$buildBaseArgument] = Get-PinnedUpstreamRef -ManifestPath $item.Manifest -ImageKey "build_base_image" -DigestKey "build_base_digest"
        }
    }
    if ($item.Image -eq "service/frontend") {
        $frontendEnvPath = Join-Path $repoRoot "frontend\.env"
        if (-not (Test-Path -LiteralPath $frontendEnvPath)) {
            throw "service/frontend 构建缺少前端构建配置: $frontendEnvPath"
        }
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
        foreach ($line in Get-Content -LiteralPath $frontendEnvPath) {
            if ($line -match '^\s*(VITE_[A-Z0-9_]+)\s*=(.*)$') {
                $buildArguments[$Matches[1]] = $Matches[2].Trim()
            }
        }
        $requiredFrontendArgs = @(
            "VITE_API_BASE_URL",
            "VITE_API_TIMEOUT_MS",
            "VITE_RUNTIME_SELFTEST_POLL_INTERVAL_MS",
            "VITE_RUNTIME_SELFTEST_POLL_TIMEOUT_MS",
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
