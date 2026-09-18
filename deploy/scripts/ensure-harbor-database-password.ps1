# ensure-harbor-database-password 为 Harbor 内部 PostgreSQL 生成并持久化独立的受控密码材料。
param(
    [string]$SecretPath = (Join-Path $PSScriptRoot "..\\config\\supply-chain.secret.env")
)

$ErrorActionPreference = "Stop"
$SecretPath = [System.IO.Path]::GetFullPath($SecretPath)
if (-not (Test-Path -LiteralPath $SecretPath -PathType Leaf)) {
    throw "供应链密钥文件不存在: $SecretPath"
}

# Get-SecretValue 严格拒绝重复配置，防止生成器把值写入不确定的位置。
function Get-SecretValue {
    param([string[]]$Lines, [string]$Key)
    $matches = @($Lines | Where-Object { $_ -match "^\s*$([regex]::Escape($Key))\s*=(.*)$" })
    if ($matches.Count -gt 1) {
        throw "供应链密钥配置重复: $Key"
    }
    if ($matches.Count -eq 0) {
        return $null
    }
    return (($matches[0] -replace "^\s*$([regex]::Escape($Key))\s*=", "").Trim())
}

# New-DatabasePassword 使用加密安全随机源生成仅含 env-safe 字符的独立密码。
function New-DatabasePassword {
    $bytes = New-Object byte[] 32
    [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    return ([Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_'))
}

$lines = [System.Collections.Generic.List[string]]::new()
foreach ($line in Get-Content -LiteralPath $SecretPath) {
    $lines.Add($line)
}
$key = "HARBOR_DATABASE_PASSWORD"
$value = Get-SecretValue -Lines $lines -Key $key
if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace($value)) {
    if ($value.Length -lt 32 -or $value -match "\s" -or $value -match "^(changeit|changeme)") {
        throw "$key 不符合独立高熵密码要求"
    }
    Write-Output "$key 已存在且通过格式校验（值已隐藏）"
    exit 0
}

$password = New-DatabasePassword
$updated = $false
for ($index = 0; $index -lt $lines.Count; $index++) {
    if ($lines[$index] -match "^\s*$([regex]::Escape($key))\s*=") {
        $lines[$index] = "$key=$password"
        $updated = $true
        break
    }
}
if (-not $updated) {
    $lines.Add("$key=$password")
}
$encoding = New-Object System.Text.UTF8Encoding $false
[System.IO.File]::WriteAllLines($SecretPath, $lines, $encoding)
Write-Output "$key 已由项目受控随机源生成并写入供应链密钥文件（值已隐藏）"
