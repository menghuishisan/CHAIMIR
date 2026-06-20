# 统一创建 Harbor 镜像分类 project,避免平台镜像落到默认 library。
param(
    [string]$ConfigPath = (Join-Path $PSScriptRoot "..\config\chaimir.env"),
    [string]$SecretPath = (Join-Path $PSScriptRoot "..\config\supply-chain.secret.env")
)

$ErrorActionPreference = "Stop"

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

function Get-BasicAuthHeader {
    param(
        [string]$User,
        [string]$Password
    )
    $bytes = [Text.Encoding]::ASCII.GetBytes(("{0}:{1}" -f $User, $Password))
    return [Convert]::ToBase64String($bytes)
}

if (-not (Test-Path -LiteralPath $ConfigPath)) {
    throw "配置文件不存在: $ConfigPath"
}
if (-not (Test-Path -LiteralPath $SecretPath)) {
    throw "密钥文件不存在: $SecretPath"
}

$externalUrl = (Get-EnvValue -Path $ConfigPath -Key "SUPPLY_CHAIN_HARBOR_EXTERNAL_URL").TrimEnd("/")
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
