# 统一创建 Harbor 镜像分类 project 和供应链 robot,避免平台镜像落到默认 library 或使用无权限凭据。
param(
    [string]$ConfigPath = (Join-Path $PSScriptRoot "..\config\chaimir.env"),
    [string]$SecretPath = (Join-Path $PSScriptRoot "..\config\supply-chain.secret.env")
)

$ErrorActionPreference = "Stop"

# Get-EnvValue 读取必填 env 键,缺失时立即终止 Harbor 初始化。
function Get-EnvValue {
    param(
        [string]$Path,
        [string]$Key
    )
    $match = Select-String -LiteralPath $Path -Pattern ("^{0}=(.*)$" -f [regex]::Escape($Key)) | Select-Object -First 1
    if (-not $match) {
        throw "缺少配置项: $Key"
    }
    return $match.Matches[0].Groups[1].Value.Trim()
}

# Get-OptionalEnvValue 读取可选 env 键,缺失时返回默认值。
function Get-OptionalEnvValue {
    param(
        [string]$Path,
        [string]$Key,
        [string]$DefaultValue
    )
    $match = Select-String -LiteralPath $Path -Pattern ("^{0}=(.*)$" -f [regex]::Escape($Key)) | Select-Object -First 1
    if (-not $match) {
        return $DefaultValue
    }
    $value = $match.Matches[0].Groups[1].Value.Trim()
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

$externalUrl = (Get-EnvValue -Path $ConfigPath -Key "SUPPLY_CHAIN_HARBOR_EXTERNAL_URL").TrimEnd("/")
$registry = Get-OptionalEnvValue -Path $ConfigPath -Key "SUPPLY_CHAIN_REGISTRY" -DefaultValue (Get-OptionalEnvValue -Path $ConfigPath -Key "IMAGE_REGISTRY" -DefaultValue "")
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
    "ingress"
)

foreach ($project in $projects) {
    $projectUri = "$externalUrl/api/v2.0/projects/$project"
    try {
        Invoke-WebRequest -UseBasicParsing -Uri $projectUri -Headers $headers -TimeoutSec 20 | Out-Null
        Write-Host "Harbor project exists: $project"
        continue
    } catch {
        $response = $_.Exception.Response
        if ($null -eq $response -or [int]$response.StatusCode -ne 404) {
            throw
        }
    }

    $body = @{
        project_name = $project
        metadata = @{
            public = "false"
        }
    } | ConvertTo-Json -Compress
    Invoke-WebRequest -UseBasicParsing -Uri "$externalUrl/api/v2.0/projects" -Method Post -Headers ($headers + @{ "Content-Type" = "application/json" }) -Body $body -TimeoutSec 20 | Out-Null
    Write-Host "Created Harbor project: $project"
}

$robotName = Get-OptionalEnvValue -Path $SecretPath -Key "HARBOR_ROBOT_NAME" -DefaultValue "chaimir-supply-chain"
$registryUser = Get-OptionalEnvValue -Path $SecretPath -Key "HARBOR_ROBOT_USERNAME" -DefaultValue ""
$registryPassword = Get-OptionalEnvValue -Path $SecretPath -Key "HARBOR_ROBOT_PASSWORD" -DefaultValue ""
$robots = Invoke-RestMethod -UseBasicParsing -Uri "$externalUrl/api/v2.0/robots" -Headers $headers -TimeoutSec 20
$robotFullName = "robot`$$robotName"
$existingRobot = @($robots | Where-Object { $_.name -eq $robotFullName -or $_.name -eq $robotName }) | Select-Object -First 1
if ($existingRobot) {
    if ([string]::IsNullOrWhiteSpace($registryUser) -or [string]::IsNullOrWhiteSpace($registryPassword)) {
        throw "Harbor robot 已存在但本地缺少 HARBOR_ROBOT_USERNAME/HARBOR_ROBOT_PASSWORD;请在 Harbor UI 刷新 secret 后写入 $SecretPath"
    }
    Write-Host "Harbor robot exists: $($existingRobot.name)"
} else {
    $access = @(
        @{ resource = "repository"; action = "pull" },
        @{ resource = "repository"; action = "push" },
        @{ resource = "repository"; action = "delete" },
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
    $robotBody = @{
        name = $robotName
        description = "Chaimir image supply-chain publishing, scanning and signing"
        duration = -1
        disable = $false
        level = "system"
        permissions = $permissions
    } | ConvertTo-Json -Compress -Depth 6
    $createdRobot = Invoke-RestMethod -UseBasicParsing -Uri "$externalUrl/api/v2.0/robots" -Method Post -Headers ($headers + @{ "Content-Type" = "application/json" }) -Body $robotBody -TimeoutSec 20
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
Write-DockerAuthConfig -Registry $registry -Username $registryUser -Password $registryPassword -DockerConfigDir $dockerConfigDir
Write-Host "Wrote Docker registry auth config: $dockerConfigDir\config.json"
