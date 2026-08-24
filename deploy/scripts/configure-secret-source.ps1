# 将受控运维输入写入唯一 Kubernetes 密钥源,不在终端输出任何敏感值。
[CmdletBinding()]
param(
    [string]$Context = "kind-chaimir-cilium",
    [string]$SecretEnvPath = "",
    [string]$SecretContractPath = "",
    [string]$PostgresCertificatePath = "",
    [string]$PostgresPrivateKeyPath = ""
)

$ErrorActionPreference = "Stop"
$scriptDirectory = Split-Path -Parent $MyInvocation.MyCommand.Path
if ([string]::IsNullOrWhiteSpace($SecretEnvPath)) { $SecretEnvPath = Join-Path $scriptDirectory "..\config\secret.env" }
if ([string]::IsNullOrWhiteSpace($SecretContractPath)) { $SecretContractPath = Join-Path $scriptDirectory "..\config\secret.env.example" }
if ([string]::IsNullOrWhiteSpace($PostgresCertificatePath)) { $PostgresCertificatePath = Join-Path $scriptDirectory "..\config\chaimir-tls\postgres.crt" }
if ([string]::IsNullOrWhiteSpace($PostgresPrivateKeyPath)) { $PostgresPrivateKeyPath = Join-Path $scriptDirectory "..\config\chaimir-tls\postgres.key" }

function Read-EnvValues([string]$Path) {
    $values = @{}
    foreach ($line in Get-Content -LiteralPath (Resolve-Path $Path)) {
        $trimmed = $line.Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) { continue }
        $separator = $trimmed.IndexOf("=")
        if ($separator -le 0) { throw "密钥文件包含无效行" }
        $name = $trimmed.Substring(0, $separator).Trim()
        $value = $trimmed.Substring($separator + 1)
        if ($values.ContainsKey($name)) { throw "密钥文件包含重复键: $name" }
        $values[$name] = $value
    }
    return $values
}

$contract = Read-EnvValues $SecretContractPath
$values = Read-EnvValues $SecretEnvPath
foreach ($name in $contract.Keys) {
    if (-not $values.ContainsKey($name) -or [string]::IsNullOrWhiteSpace($values[$name])) {
        throw "密钥文件缺少必填值: $name"
    }
    if ($values[$name] -match "^changeme") {
        throw "密钥文件仍包含模板占位值: $name"
    }
}
if (-not (Test-Path -LiteralPath $PostgresCertificatePath -PathType Leaf)) { throw "缺少 PostgreSQL TLS 证书" }
if (-not (Test-Path -LiteralPath $PostgresPrivateKeyPath -PathType Leaf)) { throw "缺少 PostgreSQL TLS 私钥" }

& kubectl --context $Context create namespace chaimir-secrets --dry-run=client -o yaml | & kubectl --context $Context apply -f - | Out-Null
if ($LASTEXITCODE -ne 0) { throw "创建密钥源命名空间失败" }
& kubectl --context $Context -n chaimir-secrets create secret generic chaimir `
    --from-env-file=$SecretEnvPath `
    --from-file="POSTGRES_TLS_CERTIFICATE=$PostgresCertificatePath" `
    --from-file="POSTGRES_TLS_PRIVATE_KEY=$PostgresPrivateKeyPath" `
    --dry-run=client -o yaml | & kubectl --context $Context apply -f - | Out-Null
if ($LASTEXITCODE -ne 0) { throw "更新密钥源失败" }
Write-Output "密钥源已更新;敏感值未输出"
