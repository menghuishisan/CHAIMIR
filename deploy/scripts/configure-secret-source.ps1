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

$namespaceYaml = & kubectl --context $Context --request-timeout=30s create namespace chaimir-secrets --dry-run=client -o yaml
if ($LASTEXITCODE -ne 0) { throw "生成密钥源命名空间清单失败" }
$namespacePath = Join-Path ([IO.Path]::GetTempPath()) ("chaimir-secrets-namespace-" + [Guid]::NewGuid().ToString("N") + ".yaml")
[IO.File]::WriteAllText($namespacePath, ($namespaceYaml -join "`n"), [Text.UTF8Encoding]::new($false))
try {
    & kubectl --context $Context --request-timeout=30s apply -f $namespacePath | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "创建密钥源命名空间失败" }
} finally {
    if (Test-Path -LiteralPath $namespacePath) { Remove-Item -LiteralPath $namespacePath -Force }
}
# Kubernetes CLI 不允许 --from-env-file 与 --from-file 混用;统一构造一个
# Secret data 映射,只发布契约文件声明的键,避免旧的契约外值进入集群。
$secretData = @{}
foreach ($name in $contract.Keys) {
    $secretData[$name] = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([string]$values[$name]))
}
$secretData["POSTGRES_TLS_CERTIFICATE"] = [Convert]::ToBase64String([IO.File]::ReadAllBytes((Resolve-Path $PostgresCertificatePath)))
$secretData["POSTGRES_TLS_PRIVATE_KEY"] = [Convert]::ToBase64String([IO.File]::ReadAllBytes((Resolve-Path $PostgresPrivateKeyPath)))
$secretManifest = @{
    apiVersion = "v1"
    kind = "Secret"
    metadata = @{ name = "chaimir"; namespace = "chaimir-secrets" }
    type = "Opaque"
    data = $secretData
} | ConvertTo-Json -Depth 10 -Compress
$secretManifestPath = Join-Path ([IO.Path]::GetTempPath()) ("chaimir-secrets-" + [Guid]::NewGuid().ToString("N") + ".json")
[IO.File]::WriteAllText($secretManifestPath, $secretManifest, [Text.UTF8Encoding]::new($false))
try {
    & kubectl --context $Context --request-timeout=30s apply -f $secretManifestPath | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "更新密钥源失败" }
} finally {
    if (Test-Path -LiteralPath $secretManifestPath) { Remove-Item -LiteralPath $secretManifestPath -Force }
}
Write-Output "密钥源已更新;敏感值未输出"
