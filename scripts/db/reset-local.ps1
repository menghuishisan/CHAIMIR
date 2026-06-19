# reset-local.ps1 是本地验收测试数据库重置入口。
# 它只负责加载环境变量并调用 backend/cmd/migrate 的统一编排逻辑。

[CmdletBinding()]
param(
    [string]$BackendEnv = "backend/.env",
    [string]$SecretEnv = "deploy/config/secret.env"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Import-ChaimirEnvFile {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "环境变量文件不存在: $Path"
    }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) {
            continue
        }
        $parts = $trimmed.Split("=", 2)
        if ($parts.Count -ne 2) {
            throw "环境变量行格式非法: $Path => $line"
        }
        [Environment]::SetEnvironmentVariable($parts[0].Trim(), $parts[1].Trim(), "Process")
    }
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "../..")
Set-Location $repoRoot

Import-ChaimirEnvFile -Path $BackendEnv
Import-ChaimirEnvFile -Path $SecretEnv

if ($env:APP_ENV -notin @("local", "dev", "development") -and $env:DEPLOY_MODE -notin @("local", "dev")) {
    throw "reset-local 只允许 APP_ENV/DEPLOY_MODE 为 local/dev/development,当前 APP_ENV=$env:APP_ENV DEPLOY_MODE=$env:DEPLOY_MODE"
}
if ($env:PG_HOST -notin @("127.0.0.1", "localhost", "::1")) {
    throw "reset-local 只允许本机数据库,当前 PG_HOST=$env:PG_HOST"
}

Push-Location (Join-Path $repoRoot "backend")
try {
    go run ./cmd/migrate reset-local
}
finally {
    Pop-Location
}
