# 统一创建 Harbor 镜像分类 project 和供应链 robot,避免平台镜像落到默认 library 或使用无权限凭据。
param(
    [string]$ConfigPath = (Join-Path $PSScriptRoot "..\config\chaimir.env"),
    [string]$SecretPath = (Join-Path $PSScriptRoot "..\config\supply-chain.secret.env")
)

$ErrorActionPreference = "Stop"
Import-Module (Join-Path $PSScriptRoot "..\..\images\lib\ImageMetadata.psm1") -Force

# Get-EnvValue 读取必填 env 键,缺失时立即终止 Harbor 初始化。
function Get-EnvValue {
    param(
        [string]$Path,
        [string]$Key
    )
    $matchItems = @(Select-String -LiteralPath $Path -Pattern ("^{0}=(.*)$" -f [regex]::Escape($Key)))
    if ($matchItems.Count -eq 0) {
        throw "缺少配置项: $Key"
    }
    if ($matchItems.Count -ne 1) {
        throw "配置项重复: $Key"
    }
    $value = $matchItems[0].Matches[0].Groups[1].Value.Trim()
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "配置项为空: $Key"
    }
    return $value
}

# Get-OptionalEnvValue 读取可选 env 键,缺失时返回默认值。
function Get-OptionalEnvValue {
    param(
        [string]$Path,
        [string]$Key,
        [string]$DefaultValue
    )
    $matchItems = @(Select-String -LiteralPath $Path -Pattern ("^{0}=(.*)$" -f [regex]::Escape($Key)))
    if ($matchItems.Count -gt 1) {
        throw "配置项重复: $Key"
    }
    if ($matchItems.Count -eq 0) {
        return $DefaultValue
    }
    $value = $matchItems[0].Matches[0].Groups[1].Value.Trim()
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $DefaultValue
    }
    return $value
}

# Get-BasicAuthHeader 为 Harbor API 请求生成 Basic Authorization 值。
function Get-BasicAuthHeader {
    param(
        [string]$User,
        [string]$Password
    )
    $bytes = [Text.Encoding]::ASCII.GetBytes(("{0}:{1}" -f $User, $Password))
    return [Convert]::ToBase64String($bytes)
}

# Write-TextFile 统一写无 BOM UTF-8,避免 secret/env 被 PowerShell 5 写入 BOM。
function Write-TextFile {
    param(
        [string]$Path,
        [string[]]$Lines
    )
    $encoding = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllLines($Path, $Lines, $encoding)
}

# Set-EnvValue 原地更新 env 文件中的单个键;缺失时追加到文件尾部。
function Set-EnvValue {
    param(
        [string]$Path,
        [string]$Key,
        [string]$Value
    )
    $lines = [System.Collections.Generic.List[string]]::new()
    foreach ($line in Get-Content -LiteralPath $Path) {
        $lines.Add($line)
    }
    $updated = $false
    for ($i = 0; $i -lt $lines.Count; $i++) {
        if ($lines[$i] -match "^\s*$([regex]::Escape($Key))\s*=") {
            $lines[$i] = "$Key=$Value"
            $updated = $true
            break
        }
    }
    if (-not $updated) {
        $lines.Add("$Key=$Value")
    }
    Write-TextFile -Path $Path -Lines $lines
}

# Invoke-HarborApi 使用项目 CA 通过 curl 访问 Harbor API,保留真实 HTTP 状态码和严格 TLS 验证。
function Invoke-HarborApi {
    param(
        [string]$CAPath,
        [string]$Method,
        [string]$Uri,
        [hashtable]$Headers,
        [string]$Body = ""
    )
    if (-not (Test-Path -LiteralPath $CAPath -PathType Leaf)) {
        throw "Harbor CA 文件不存在: $CAPath"
    }
    $responsePath = [System.IO.Path]::GetTempFileName()
    $bodyPath = $null
    try {
        $curlArguments = @(
            "--noproxy", "*",
            "--cacert", $CAPath,
            "--silent", "--show-error",
            "--connect-timeout", "5",
            "--max-time", "20",
            "--output", $responsePath,
            "--write-out", "%{http_code}",
            "--request", $Method,
            "--url", $Uri
        )
        foreach ($header in $Headers.GetEnumerator()) {
            $curlArguments += @("--header", ("{0}: {1}" -f [string]$header.Key, [string]$header.Value))
        }
        if (-not [string]::IsNullOrWhiteSpace($Body)) {
            $bodyPath = [System.IO.Path]::GetTempFileName()
            $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
            [System.IO.File]::WriteAllText($bodyPath, $Body, $utf8NoBom)
            $curlArguments += @("--header", "Content-Type: application/json", "--data-binary", "@$bodyPath")
        }
        $statusText = (& curl.exe @curlArguments 2>&1 | Out-String).Trim()
        if ($LASTEXITCODE -ne 0) {
            throw "Harbor API TLS/transport 请求失败: method=$Method uri=$Uri detail=$statusText"
        }
        if ($statusText -notmatch "^\d{3}$") {
            throw "Harbor API 未返回合法 HTTP 状态码: method=$Method uri=$Uri status=$statusText"
        }
        return [pscustomobject]@{
            StatusCode = [int]$statusText
            Body = [System.IO.File]::ReadAllText($responsePath)
        }
    }
    finally {
        if (Test-Path -LiteralPath $responsePath) {
            [System.IO.File]::Delete($responsePath)
        }
        if ($null -ne $bodyPath -and (Test-Path -LiteralPath $bodyPath)) {
            [System.IO.File]::Delete($bodyPath)
        }
    }
}

# Resolve-DeployPath 支持配置使用 deploy/ 下相对路径或宿主机绝对路径。
function Resolve-DeployPath {
    param([string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) {
        return ""
    }
    if ([System.IO.Path]::IsPathRooted($Value)) {
        return $Value
    }
    return (Join-Path (Split-Path -Parent $ConfigPath) "..\$Value")
}

# Write-DockerAuthConfig 生成标准 Docker registry 认证文件,避免交互式 docker login 处理特殊字符失败。
function Write-DockerAuthConfig {
    param(
        [string]$Registry,
        [string]$Username,
        [string]$Password,
        [string]$DockerConfigDir
    )
    if ([string]::IsNullOrWhiteSpace($DockerConfigDir)) {
        return
    }
    New-Item -ItemType Directory -Force -Path $DockerConfigDir | Out-Null
    $auth = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes(("{0}:{1}" -f $Username, $Password)))
    $body = @{
        auths = @{
            $Registry = @{
                auth = $auth
            }
        }
    } | ConvertTo-Json -Compress -Depth 5
    Write-TextFile -Path (Join-Path $DockerConfigDir "config.json") -Lines @($body)
}

if (-not (Test-Path -LiteralPath $ConfigPath)) {
    throw "配置文件不存在: $ConfigPath"
}
if (-not (Test-Path -LiteralPath $SecretPath)) {
    throw "密钥文件不存在: $SecretPath"
}
[void](Read-ChaimirEnvFile -Path $SecretPath -RequiredKeys @("HARBOR_ADMIN_PASSWORD"))

$configValues = Read-ChaimirEnvFile -Path $ConfigPath -RequiredKeys @(
    "SUPPLY_CHAIN_REGISTRY", "SUPPLY_CHAIN_REGISTRY_ENDPOINT", "SUPPLY_CHAIN_HARBOR_EXTERNAL_URL", "IMAGE_REGISTRY", "SUPPLY_CHAIN_REGISTRY_CA_HOST_PATH"
)
$registry = Get-ChaimirCanonicalRegistry -Config $configValues
$externalUrl = ([string]$configValues["SUPPLY_CHAIN_HARBOR_EXTERNAL_URL"]).TrimEnd("/")
$registryEndpoint = $registry
$apiBase = $externalUrl
$registryCAPath = Resolve-DeployPath -Value ([string]$configValues["SUPPLY_CHAIN_REGISTRY_CA_HOST_PATH"])
$dockerConfigValue = Get-OptionalEnvValue -Path $ConfigPath -Key "SUPPLY_CHAIN_DOCKER_CONFIG_HOST_DIR" -DefaultValue "config/docker-auth"
$dockerConfigDir = Resolve-DeployPath -Value $dockerConfigValue
$adminPassword = Get-EnvValue -Path $SecretPath -Key "HARBOR_ADMIN_PASSWORD"
$auth = Get-BasicAuthHeader -User "admin" -Password $adminPassword
$headers = @{
    Authorization = "Basic $auth"
}
$projects = @(
    "service",
    "runtime",
    "infra",
    "tool",
    "judger",
    "sim",
    "sidecar",
    "init",
    "base",
    "middleware",
    "observability",
    "ingress",
    "network"
)

foreach ($project in $projects) {
    $projectUri = "$apiBase/api/v2.0/projects/$project"
    $projectResponse = Invoke-HarborApi -CAPath $registryCAPath -Method "GET" -Uri $projectUri -Headers $headers
    if ($projectResponse.StatusCode -ge 200 -and $projectResponse.StatusCode -lt 300) {
        Write-Host "Harbor project exists: $project"
        continue
    }
    if ($projectResponse.StatusCode -ne 404) {
        throw "Harbor project 查询失败: project=$project status=$($projectResponse.StatusCode)"
    }

    $body = @{
        project_name = $project
        metadata = @{
            public = "false"
        }
    } | ConvertTo-Json -Compress
    $createProjectResponse = Invoke-HarborApi -CAPath $registryCAPath -Method "POST" -Uri "$apiBase/api/v2.0/projects" -Headers $headers -Body $body
    if ($createProjectResponse.StatusCode -lt 200 -or $createProjectResponse.StatusCode -ge 300) {
        throw "Harbor project 创建失败: project=$project status=$($createProjectResponse.StatusCode)"
    }
    Write-Host "Created Harbor project: $project"
}

$robotName = Get-OptionalEnvValue -Path $SecretPath -Key "HARBOR_ROBOT_NAME" -DefaultValue "chaimir-supply-chain"
$registryUser = Get-OptionalEnvValue -Path $SecretPath -Key "HARBOR_ROBOT_USERNAME" -DefaultValue ""
$registryPassword = Get-OptionalEnvValue -Path $SecretPath -Key "HARBOR_ROBOT_PASSWORD" -DefaultValue ""
$access = @(
    @{ resource = "repository"; action = "pull" },
    @{ resource = "repository"; action = "push" },
    @{ resource = "tag"; action = "create" },
    @{ resource = "tag"; action = "delete" },
    @{ resource = "artifact-label"; action = "create" },
    @{ resource = "scan"; action = "create" }
)
$permissions = @()
foreach ($project in $projects) {
    $permissions += @{
        kind = "project"
        namespace = $project
        access = $access
    }
}
$robotsResponse = Invoke-HarborApi -CAPath $registryCAPath -Method "GET" -Uri "$apiBase/api/v2.0/robots" -Headers $headers
if ($robotsResponse.StatusCode -lt 200 -or $robotsResponse.StatusCode -ge 300) {
    throw "Harbor robot 查询失败: status=$($robotsResponse.StatusCode)"
}
$robots = $robotsResponse.Body | ConvertFrom-Json
$robotFullName = "robot`$$robotName"
$existingRobot = @($robots | Where-Object { $_.name -eq $robotFullName -or $_.name -eq $robotName }) | Select-Object -First 1
if ($existingRobot) {
    if ([string]::IsNullOrWhiteSpace($registryUser) -or [string]::IsNullOrWhiteSpace($registryPassword)) {
        throw "Harbor robot 已存在但配置源缺少 HARBOR_ROBOT_USERNAME/HARBOR_ROBOT_PASSWORD;请在 Harbor UI 刷新 Secret 后写入 $SecretPath"
    }
    $existingNamespaces = @($existingRobot.permissions | Where-Object { $_.kind -eq "project" } | ForEach-Object { $_.namespace })
    $missingNamespaces = @($projects | Where-Object { $_ -notin $existingNamespaces })
    $permissionMismatch = $false
    foreach ($project in $projects) {
        $existingPermission = @($existingRobot.permissions | Where-Object { $_.kind -eq "project" -and $_.namespace -eq $project }) | Select-Object -First 1
        $actualActions = @($existingPermission.access | ForEach-Object { "{0}/{1}" -f $_.resource, $_.action } | Sort-Object)
        $expectedActions = @($access | ForEach-Object { "{0}/{1}" -f $_.resource, $_.action } | Sort-Object)
        if ($null -eq $existingPermission -or ($actualActions -join ",") -ne ($expectedActions -join ",")) {
            $permissionMismatch = $true
            break
        }
    }
    $refreshRobot = $false
    if ($missingNamespaces.Count -gt 0 -or $permissionMismatch) {
        $refreshRobot = $true
        Write-Host "Harbor robot permissions are not the declared least-privilege set; refreshing: $($missingNamespaces -join ',')"
    } else {
        $registryAuth = Get-BasicAuthHeader -User $registryUser -Password $registryPassword
        try {
            $robotCheckResponse = Invoke-HarborApi -CAPath $registryCAPath -Method "GET" -Uri "$apiBase/v2/" -Headers @{ Authorization = "Basic $registryAuth" }
            if ($robotCheckResponse.StatusCode -lt 200 -or $robotCheckResponse.StatusCode -ge 300) {
                throw "Harbor robot check status=$($robotCheckResponse.StatusCode)"
            }
            Write-Host "Harbor robot exists with complete project permissions: $($existingRobot.name)"
        } catch {
            if ($_.Exception.Message -notmatch "status=401") {
                throw
            }
            $refreshRobot = $true
            Write-Host "Harbor robot credential expired; refreshing token: $($existingRobot.name)"
        }
    }
    if ($refreshRobot) {
        $robotBody = @{ permissions = $permissions } | ConvertTo-Json -Compress -Depth 6
        $updateRobotResponse = Invoke-HarborApi -CAPath $registryCAPath -Method "PATCH" -Uri "$apiBase/api/v2.0/robots/$($existingRobot.id)" -Headers $headers -Body $robotBody
        if ($updateRobotResponse.StatusCode -lt 200 -or $updateRobotResponse.StatusCode -ge 300) {
            throw "Harbor robot 更新失败: status=$($updateRobotResponse.StatusCode)"
        }
        $updatedRobot = $updateRobotResponse.Body | ConvertFrom-Json
        if (-not [string]::IsNullOrWhiteSpace($updatedRobot.secret)) {
            $registryPassword = $updatedRobot.secret
            Set-EnvValue -Path $SecretPath -Key "HARBOR_ROBOT_USERNAME" -Value $registryUser
            Set-EnvValue -Path $SecretPath -Key "HARBOR_ROBOT_PASSWORD" -Value $registryPassword
        }
        Write-Host "Updated Harbor robot permissions and credential: $($existingRobot.name)"
    }
} else {
    $robotBody = @{
        name = $robotName
        description = "Chaimir image supply-chain publishing, scanning and signing"
        duration = -1
        disable = $false
        level = "system"
        permissions = $permissions
    } | ConvertTo-Json -Compress -Depth 6
    $createRobotResponse = Invoke-HarborApi -CAPath $registryCAPath -Method "POST" -Uri "$apiBase/api/v2.0/robots" -Headers $headers -Body $robotBody
    if ($createRobotResponse.StatusCode -lt 200 -or $createRobotResponse.StatusCode -ge 300) {
        throw "Harbor robot 创建失败: status=$($createRobotResponse.StatusCode)"
    }
    $createdRobot = $createRobotResponse.Body | ConvertFrom-Json
    $registryUser = $createdRobot.name
    if ([string]::IsNullOrWhiteSpace($registryUser)) {
        $registryUser = $robotFullName
    }
    $registryPassword = $createdRobot.token
    if ([string]::IsNullOrWhiteSpace($registryPassword)) {
        $registryPassword = $createdRobot.secret
    }
    if ([string]::IsNullOrWhiteSpace($registryPassword)) {
        throw "Harbor 已创建 robot,但响应中没有返回 token/secret;请在 Harbor UI 刷新 secret 后写入 HARBOR_ROBOT_PASSWORD"
    }
    Set-EnvValue -Path $SecretPath -Key "HARBOR_ROBOT_NAME" -Value $robotName
    Set-EnvValue -Path $SecretPath -Key "HARBOR_ROBOT_USERNAME" -Value $registryUser
    Set-EnvValue -Path $SecretPath -Key "HARBOR_ROBOT_PASSWORD" -Value $registryPassword
    Write-Host "Created Harbor robot: $registryUser"
}

if ([string]::IsNullOrWhiteSpace($registry)) {
    throw "缺少 SUPPLY_CHAIN_REGISTRY 或 IMAGE_REGISTRY"
}
Write-DockerAuthConfig -Registry $registryEndpoint -Username $registryUser -Password $registryPassword -DockerConfigDir $dockerConfigDir
Write-Host "Wrote Docker registry auth config: $dockerConfigDir\config.json"
