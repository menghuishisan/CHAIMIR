# image-attestations-generate 执行本地/私有化镜像扫描、签名、验证并生成后端沙箱准入证明。
param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path,
    [string]$ConfigPath = "",
    [string]$SecretPath = "",
    [string]$DigestLock = "",
    [string]$BackendEnvPath = "",
    [string]$DeployEnvPath = "",
    [string]$EvidenceDir = "",
    [string]$Severity = "HIGH,CRITICAL",
    [string]$TrivyStatusPassed = "passed",
    [switch]$GenerateKeyIfMissing,
    [switch]$SkipScan,
    [switch]$SkipSign,
    [switch]$SkipVerify,
    [switch]$NoEnvWrite
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($ConfigPath)) {
    $ConfigPath = Join-Path $RepoRoot "deploy\config\chaimir.env"
}
if ([string]::IsNullOrWhiteSpace($SecretPath)) {
    $SecretPath = Join-Path $RepoRoot "deploy\config\supply-chain.secret.env"
}
if ([string]::IsNullOrWhiteSpace($DigestLock)) {
    $DigestLock = Join-Path $RepoRoot "images\image-digests.lock"
}
if ([string]::IsNullOrWhiteSpace($BackendEnvPath)) {
    $BackendEnvPath = Join-Path $RepoRoot "backend\.env"
}
if ([string]::IsNullOrWhiteSpace($DeployEnvPath)) {
    $DeployEnvPath = $ConfigPath
}
if ([string]::IsNullOrWhiteSpace($EvidenceDir)) {
    $EvidenceDir = Join-Path $RepoRoot ".tmp\backend-functional-test\evidence"
}

# Read-EnvFile 读取简单 KEY=VALUE 文件;只返回键值,不输出密钥内容。
function Read-EnvFile {
    param([string]$Path)
    $items = @{}
    if (-not (Test-Path -LiteralPath $Path)) {
        return $items
    }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq "" -or $trimmed.StartsWith("#")) {
            continue
        }
        $idx = $trimmed.IndexOf("=")
        if ($idx -le 0) {
            continue
        }
        $items[$trimmed.Substring(0, $idx).Trim()] = $trimmed.Substring($idx + 1).Trim()
    }
    return $items
}

# Set-EnvValue 原地更新 env 文件中的单个键;缺失时追加到文件尾部。
function Set-EnvValue {
    param(
        [string]$Path,
        [string]$Key,
        [string]$Value
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "缺少 env 文件: $Path"
    }
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

# Write-TextFile 统一写无 BOM UTF-8,避免 env/JSON 被 PowerShell 5 写入 BOM。
function Write-TextFile {
    param(
        [string]$Path,
        [string[]]$Lines
    )
    $encoding = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllLines($Path, $Lines, $encoding)
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
    return (Join-Path (Join-Path $RepoRoot "deploy") $Value)
}

# Read-DigestLock 读取 category/name sha256:digest 格式的镜像锁。
function Read-DigestLock {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "缺少 digest lock: $Path"
    }
    $items = [System.Collections.Generic.List[object]]::new()
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq "" -or $trimmed.StartsWith("#")) {
            continue
        }
        if ($trimmed -notmatch "^([^:\s]+/[^:\s]+)\s*[:= ]\s*(sha256:[0-9a-f]{64})$") {
            throw "digest lock 行格式非法: $line"
        }
        $items.Add([pscustomobject]@{ Image = $Matches[1]; Digest = $Matches[2] })
    }
    if ($items.Count -eq 0) {
        throw "digest lock 中没有可证明镜像: $Path"
    }
    return $items
}

# Read-YamlBlock 读取顶层 YAML 块,用于解析 manifest 的供应链准入状态。
function Read-YamlBlock {
    param(
        [string]$Path,
        [string]$BlockName
    )
    $lines = Get-Content -LiteralPath $Path
    $block = [System.Collections.Generic.List[string]]::new()
    $inside = $false
    foreach ($line in $lines) {
        if ($line -match "^$([regex]::Escape($BlockName)):\s*$") {
            $inside = $true
            continue
        }
        if ($inside -and $line -match "^[A-Za-z_][A-Za-z0-9_]*:\s*") {
            break
        }
        if ($inside) {
            $block.Add($line)
        }
    }
    return ,$block.ToArray()
}

# Read-YamlValue 从顶层或块内读取简单 key:value 字段。
function Read-YamlValue {
    param(
        [string[]]$Lines,
        [string]$Key
    )
    foreach ($line in $Lines) {
        if ($line -match "^\s*$([regex]::Escape($Key)):\s*(.+?)\s*$") {
            return $Matches[1].Trim().Trim('"').Trim("'")
        }
    }
    return $null
}

# Read-ManifestAdmissionMap 返回镜像层声明的准入阻断清单;阻断镜像不会进入沙箱证明。
function Read-ManifestAdmissionMap {
    param([string]$ImagesRoot)
    $blocked = @{}
    foreach ($manifest in Get-ChildItem -Path $ImagesRoot -Recurse -Filter manifest.yaml) {
        $lines = Get-Content -LiteralPath $manifest.FullName
        $image = Read-YamlValue -Lines $lines -Key "image"
        if ([string]::IsNullOrWhiteSpace($image)) {
            continue
        }
        $supplyChain = Read-YamlBlock -Path $manifest.FullName -BlockName "supply_chain"
        $deployable = Read-YamlValue -Lines $supplyChain -Key "deployable"
        if ($deployable -eq "false") {
            $reason = Read-YamlValue -Lines $supplyChain -Key "block_reason"
            if ([string]::IsNullOrWhiteSpace($reason)) {
                $reason = "manifest marks image as not deployable"
            }
            $blocked[$image] = [pscustomobject]@{ Image = $image; Reason = $reason; Manifest = $manifest.FullName }
        }
    }
    return $blocked
}

# Read-ManifestTrivySkipMap 返回经 manifest 明确说明的 Trivy 单文件排除。
function Read-ManifestTrivySkipMap {
    param([string]$ImagesRoot)
    $items = @{}
    foreach ($manifest in Get-ChildItem -Path $ImagesRoot -Recurse -Filter manifest.yaml) {
        $lines = Get-Content -LiteralPath $manifest.FullName
        $image = Read-YamlValue -Lines $lines -Key "image"
        if ([string]::IsNullOrWhiteSpace($image)) {
            continue
        }
        $supplyChain = Read-YamlBlock -Path $manifest.FullName -BlockName "supply_chain"
        $reason = Read-YamlValue -Lines $supplyChain -Key "trivy_skip_reason"
        $skipFiles = [System.Collections.Generic.List[string]]::new()
        $insideSkipFiles = $false
        foreach ($line in $supplyChain) {
            if ($line -match "^\s+trivy_skip_files:\s*$") {
                $insideSkipFiles = $true
                continue
            }
            if ($insideSkipFiles -and $line -match "^\s+-\s*(.+?)\s*$") {
                $skipFiles.Add($Matches[1].Trim().Trim('"').Trim("'"))
                continue
            }
            if ($insideSkipFiles -and $line -match "^\s+[A-Za-z_][A-Za-z0-9_]*:\s*") {
                $insideSkipFiles = $false
            }
        }
        if ($skipFiles.Count -gt 0) {
            if ([string]::IsNullOrWhiteSpace($reason)) {
                throw "$image 声明了 trivy_skip_files 但缺少 trivy_skip_reason"
            }
            $items[$image] = ,$skipFiles.ToArray()
        }
    }
    return $items
}

# Invoke-ComposeTool 通过统一 Docker Compose 入口运行 Trivy/Cosign。
function Invoke-ComposeTool {
    param(
        [string]$Tool,
        [string[]]$ToolArgs,
        [string]$Context
    )
    Push-Location (Join-Path $RepoRoot "deploy")
    try {
        $args = @(
            "compose",
            "--project-name", "chaimir-supply-chain",
            "--env-file", "config/chaimir.env",
            "-f", "image-supply-chain.compose.yaml",
            "run", "--rm", $Tool
        ) + $ToolArgs
        $previousCosignPassword = $env:COSIGN_PRIVATE_KEY_PASSWORD
        $previousHarborAdminPassword = $env:HARBOR_ADMIN_PASSWORD
        $env:COSIGN_PRIVATE_KEY_PASSWORD = $script:cosignPrivateKeyPassword
        $env:HARBOR_ADMIN_PASSWORD = $script:harborAdminPassword
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        & docker @args
        $exitCode = $LASTEXITCODE
        $ErrorActionPreference = $previousErrorActionPreference
        $env:COSIGN_PRIVATE_KEY_PASSWORD = $previousCosignPassword
        $env:HARBOR_ADMIN_PASSWORD = $previousHarborAdminPassword
        if ($exitCode -ne 0) {
            throw "$Context 失败, exit=$exitCode"
        }
    } finally {
        Pop-Location
    }
}

$config = Read-EnvFile -Path $ConfigPath
$secret = Read-EnvFile -Path $SecretPath
$registry = $config["SUPPLY_CHAIN_REGISTRY"]
if ([string]::IsNullOrWhiteSpace($registry)) {
    $registry = $config["IMAGE_REGISTRY"]
}
if ([string]::IsNullOrWhiteSpace($registry)) {
    throw "缺少 SUPPLY_CHAIN_REGISTRY 或 IMAGE_REGISTRY"
}
$registryUser = $secret["HARBOR_ROBOT_USERNAME"]
$registryPassword = $secret["HARBOR_ROBOT_PASSWORD"]
if ([string]::IsNullOrWhiteSpace($registryUser) -or [string]::IsNullOrWhiteSpace($registryPassword)) {
    throw "缺少 HARBOR_ROBOT_USERNAME/HARBOR_ROBOT_PASSWORD"
}
$script:cosignPrivateKeyPassword = $secret["COSIGN_PRIVATE_KEY_PASSWORD"]
$script:harborAdminPassword = $secret["HARBOR_ADMIN_PASSWORD"]

$cosignDirValue = $config["SUPPLY_CHAIN_COSIGN_KEY_HOST_DIR"]
if ([string]::IsNullOrWhiteSpace($cosignDirValue)) {
    $cosignDirValue = "config/cosign"
}
$cosignDir = Resolve-DeployPath -Value $cosignDirValue
if (-not (Test-Path -LiteralPath $cosignDir)) {
    New-Item -ItemType Directory -Force -Path $cosignDir | Out-Null
}
$cosignKey = Join-Path $cosignDir "cosign.key"
$cosignPub = Join-Path $cosignDir "cosign.pub"
if ((-not (Test-Path -LiteralPath $cosignKey)) -or (-not (Test-Path -LiteralPath $cosignPub))) {
    if (-not $GenerateKeyIfMissing) {
        throw "缺少 Cosign 私钥或公钥: $cosignDir; 如需本地生成请加 -GenerateKeyIfMissing"
    }
    Invoke-ComposeTool -Tool "cosign" -ToolArgs @("generate-key-pair", "--output-key-prefix", "/cosign/cosign") -Context "生成 Cosign 密钥"
}

$dockerConfigDir = $config["SUPPLY_CHAIN_DOCKER_CONFIG_HOST_DIR"]
if (-not [string]::IsNullOrWhiteSpace($dockerConfigDir)) {
    $resolvedDockerConfig = Resolve-DeployPath -Value $dockerConfigDir
    if (-not (Test-Path -LiteralPath (Join-Path $resolvedDockerConfig "config.json"))) {
        throw "缺少 Docker registry 认证配置: $resolvedDockerConfig\config.json"
    }
}

New-Item -ItemType Directory -Force -Path $EvidenceDir | Out-Null
$scanLog = Join-Path $EvidenceDir "image-attestations-trivy.log"
$signLog = Join-Path $EvidenceDir "image-attestations-cosign.log"
$jsonPath = Join-Path $EvidenceDir "sandbox-image-attestations.json"
$summaryPath = Join-Path $EvidenceDir "image-attestations-summary.txt"
Remove-Item -LiteralPath $scanLog, $signLog -ErrorAction SilentlyContinue

$items = Read-DigestLock -Path $DigestLock
$blockedByManifest = Read-ManifestAdmissionMap -ImagesRoot (Join-Path $RepoRoot "images")
$trivySkipByManifest = Read-ManifestTrivySkipMap -ImagesRoot (Join-Path $RepoRoot "images")
$blockedItems = [System.Collections.Generic.List[object]]::new()
$attestations = [System.Collections.Generic.List[object]]::new()
foreach ($item in $items) {
    if ($blockedByManifest.ContainsKey($item.Image)) {
        $blocked = $blockedByManifest[$item.Image]
        $blockedItems.Add([pscustomobject]@{
            image    = $item.Image
            digest   = $item.Digest
            reason   = $blocked.Reason
            manifest = $blocked.Manifest
        })
        Write-Host "Blocking attestation $($item.Image)@$($item.Digest): $($blocked.Reason)"
        continue
    }
    $ref = "$registry/$($item.Image)@$($item.Digest)"
    Write-Host "Attesting $ref"
    if (-not $SkipScan) {
        try {
            $trivyArgs = @(
                "image", "--exit-code", "1", "--severity", $Severity,
                "--scanners", "vuln,secret", "--ignore-unfixed=false",
                "--username", $registryUser, "--password", $registryPassword, "--insecure"
            )
            if ($trivySkipByManifest.ContainsKey($item.Image)) {
                foreach ($skipFile in @($trivySkipByManifest[$item.Image])) {
                    $trivyArgs += @("--skip-files", $skipFile)
                }
            }
            $trivyArgs += $ref
            Invoke-ComposeTool -Tool "trivy" -ToolArgs $trivyArgs -Context "Trivy 扫描 $ref" *>> $scanLog
        } catch {
            $blockedItems.Add([pscustomobject]@{
                image    = $item.Image
                digest   = $item.Digest
                reason   = "Trivy 扫描未通过: $($_.Exception.Message)"
                manifest = ""
            })
            Write-Warning "Blocking attestation ${ref}: Trivy 扫描未通过"
            continue
        }
    }
    if (-not $SkipSign) {
        try {
            Invoke-ComposeTool -Tool "cosign" -ToolArgs @(
                "sign", "--yes", "--key", "/cosign/cosign.key", "--tlog-upload=false",
                "--use-signing-config=false", "--allow-http-registry", "--registry-username", $registryUser,
                "--registry-password", $registryPassword, $ref
            ) -Context "Cosign 签名 $ref" *>> $signLog
        } catch {
            $blockedItems.Add([pscustomobject]@{
                image    = $item.Image
                digest   = $item.Digest
                reason   = "Cosign 签名失败: $($_.Exception.Message)"
                manifest = ""
            })
            Write-Warning "Blocking attestation ${ref}: Cosign 签名失败"
            continue
        }
    }
    if (-not $SkipVerify) {
        try {
            Invoke-ComposeTool -Tool "cosign" -ToolArgs @(
                "verify", "--key", "/cosign/cosign.pub", "--insecure-ignore-tlog=true",
                "--allow-http-registry", "--registry-username", $registryUser, "--registry-password", $registryPassword, $ref
            ) -Context "Cosign 验签 $ref" *>> $signLog
        } catch {
            $blockedItems.Add([pscustomobject]@{
                image    = $item.Image
                digest   = $item.Digest
                reason   = "Cosign 验签失败: $($_.Exception.Message)"
                manifest = ""
            })
            Write-Warning "Blocking attestation ${ref}: Cosign 验签失败"
            continue
        }
    }
    $attestations.Add([pscustomobject]@{
        image_url       = $ref
        digest          = $item.Digest
        cosign_verified = $true
        trivy_status    = $TrivyStatusPassed
    })
}

$json = $attestations | ConvertTo-Json -Compress -Depth 5
Write-TextFile -Path $jsonPath -Lines @($json)
$blockedPath = Join-Path $EvidenceDir "image-attestations-blocked.json"
$blockedJson = $blockedItems | ConvertTo-Json -Compress -Depth 5
Write-TextFile -Path $blockedPath -Lines @($blockedJson)
if (-not $NoEnvWrite) {
    Set-EnvValue -Path $DeployEnvPath -Key "SANDBOX_IMAGE_ATTESTATIONS_JSON" -Value $json
    Set-EnvValue -Path $BackendEnvPath -Key "SANDBOX_IMAGE_ATTESTATIONS_JSON" -Value $json
}

$summary = @(
    "attested_count=$($attestations.Count)",
    "blocked_count=$($blockedItems.Count)",
    "registry=$registry",
    "digest_lock=$DigestLock",
    "json=$jsonPath",
    "blocked_json=$blockedPath",
    "deploy_env=$DeployEnvPath",
    "backend_env=$BackendEnvPath",
    "scan_log=$scanLog",
    "sign_log=$signLog"
)
Write-TextFile -Path $summaryPath -Lines $summary
Write-Output ($summary -join "`n")
if ($blockedItems.Count -gt 0) {
    Write-Error "镜像供应链证明存在阻塞项,详见 $blockedPath"
    exit 1
}
