# 校验后端 HPA 副本上限、两类连接池和 PostgreSQL 连接预算的一致性。
[CmdletBinding()]
param(
    [string]$HpaPath = "",
    [string]$ConfigPath = ""
)

$ErrorActionPreference = 'Stop'
$scriptDirectory = Split-Path -Parent $MyInvocation.MyCommand.Path
if ([string]::IsNullOrWhiteSpace($HpaPath)) { $HpaPath = Join-Path $scriptDirectory '..\overlays\prod-saas\hpa.yaml' }
if ([string]::IsNullOrWhiteSpace($ConfigPath)) { $ConfigPath = Join-Path $scriptDirectory '..\config\chaimir.env' }
$hpa = Get-Content -Raw -LiteralPath (Resolve-Path $HpaPath)
$config = Get-Content -LiteralPath (Resolve-Path $ConfigPath)

function Read-RequiredInt([string]$Name) {
    $line = $config | Where-Object { $_ -match "^$Name=" } | Select-Object -First 1
    if (-not $line) { throw "缺少配置 $Name" }
    $value = [int]($line -replace "^$Name=", '')
    if ($value -lt 0) { throw "配置 $Name 必须为非负整数" }
    return $value
}

$maxReplicaMatch = [regex]::Match($hpa, '(?m)^\s*maxReplicas:\s*([0-9]+)\s*$')
if (-not $maxReplicaMatch.Success) { throw 'HPA 缺少有效 maxReplicas' }
$maxReplicas = [int]$maxReplicaMatch.Groups[1].Value
if ($maxReplicas -le 0) { throw 'HPA 的 maxReplicas 必须大于零' }
$appMax = Read-RequiredInt 'PG_APP_MAX_CONNS'
$privMax = Read-RequiredInt 'PG_PRIV_MAX_CONNS'
$databaseMax = Read-RequiredInt 'PG_DATABASE_MAX_CONNECTIONS'
$reserved = Read-RequiredInt 'PG_RESERVED_CONNECTIONS'
$safety = Read-RequiredInt 'PG_CONNECTION_SAFETY_MARGIN'
if ($databaseMax -le 0) { throw 'PG_DATABASE_MAX_CONNECTIONS 必须大于零' }
$workload = $maxReplicas * ($appMax + $privMax)
$budget = $workload + $reserved + $safety
if ($budget -gt $databaseMax) {
    throw "PostgreSQL 连接预算超限: maxReplicas=$maxReplicas, app=$appMax, priv=$privMax, reserved=$reserved, safety=$safety, total=$budget, database=$databaseMax"
}
Write-Output "PostgreSQL 连接预算通过: maxReplicas=$maxReplicas, app=$appMax, priv=$privMax, reserved=$reserved, safety=$safety, total=$budget/$databaseMax"
