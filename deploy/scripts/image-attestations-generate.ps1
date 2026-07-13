# image-attestations-generate 执行本地/私有化镜像扫描、签名、验证并生成后端沙箱准入证明。
param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path,
    [string]$ConfigPath = "",
    [string]$SecretPath = "",
    [string]$DigestLock = "",
    [string]$BackendEnvPath = "",
    [string]$DeployEnvPath = "",
    [string]$EvidenceDir = "",
    [switch]$GenerateKeyIfMissing,
    [switch]$NoEnvWrite
)

$ErrorActionPreference = "Stop"
Import-Module (Join-Path $RepoRoot "images\lib\ImageMetadata.psm1") -Force

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
$EvidenceDir = if ([System.IO.Path]::IsPathRooted($EvidenceDir)) {
    [System.IO.Path]::GetFullPath($EvidenceDir)
} else {
    [System.IO.Path]::GetFullPath((Join-Path $RepoRoot $EvidenceDir))
}
New-Item -ItemType Directory -Force -Path $EvidenceDir | Out-Null
$script:evidenceHostDir = $EvidenceDir

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

# Read-ManifestAdmissionMap 返回镜像层声明的准入阻断清单;阻断镜像不会进入沙箱证明。
function Read-ManifestAdmissionMap {
    param([string]$ImagesRoot)
    $blocked = @{}
    foreach ($entry in (Get-ChaimirImageCatalog -ImagesRoot $ImagesRoot).Values) {
        if (-not $entry.Deployable) {
            $blocked[$entry.Image] = [pscustomobject]@{ Image = $entry.Image; Reason = $entry.BlockReason; Manifest = $entry.Manifest }
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
        $image = Get-ChaimirTopLevelYamlValue -Lines $lines -Key "image"
        if ([string]::IsNullOrWhiteSpace($image)) {
            continue
        }
        $supplyChain = Get-ChaimirYamlBlock -Path $manifest.FullName -BlockName "supply_chain"
        $reason = Get-ChaimirYamlValue -Lines $supplyChain -Key "trivy_skip_reason"
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
        $previousEvidenceHostDir = $env:SUPPLY_CHAIN_EVIDENCE_HOST_DIR
        $previousErrorActionPreference = $ErrorActionPreference
        try {
            $env:COSIGN_PRIVATE_KEY_PASSWORD = $script:cosignPrivateKeyPassword
            $env:SUPPLY_CHAIN_EVIDENCE_HOST_DIR = $script:evidenceHostDir
            $ErrorActionPreference = "Continue"
            & docker @args
            $exitCode = $LASTEXITCODE
            if ($exitCode -ne 0) {
                throw "$Context 失败, exit=$exitCode"
            }
        } finally {
            $ErrorActionPreference = $previousErrorActionPreference
            $env:COSIGN_PRIVATE_KEY_PASSWORD = $previousCosignPassword
            $env:SUPPLY_CHAIN_EVIDENCE_HOST_DIR = $previousEvidenceHostDir
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
$registryExternalUrl = [string]$config["SUPPLY_CHAIN_HARBOR_EXTERNAL_URL"]
$allowHttpRegistry = $registryExternalUrl.StartsWith("http://", [System.StringComparison]::OrdinalIgnoreCase)
$script:cosignPrivateKeyPassword = $secret["COSIGN_PRIVATE_KEY_PASSWORD"]

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
        throw "缺少 Cosign 私钥或公钥: $cosignDir; 如需首次初始化请加 -GenerateKeyIfMissing"
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

$scanLog = Join-Path $EvidenceDir "image-attestations-trivy.log"
$signLog = Join-Path $EvidenceDir "image-attestations-cosign.log"
$sbomDir = Join-Path $EvidenceDir "sbom"
New-Item -ItemType Directory -Force -Path $sbomDir | Out-Null
$jsonPath = Join-Path $EvidenceDir "sandbox-image-attestations.json"
$summaryPath = Join-Path $EvidenceDir "image-attestations-summary.txt"
Remove-Item -LiteralPath $scanLog, $signLog -ErrorAction SilentlyContinue

$digestItems = Read-ChaimirDigestLock -Path $DigestLock -Required
if ($digestItems.Count -eq 0) {
    throw "digest lock 中没有可证明镜像: $DigestLock"
}
$items = @($digestItems.Keys | Sort-Object | ForEach-Object {
    [pscustomobject]@{ Image = $_; Digest = $digestItems[$_] }
})
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
    try {
        $trivyArgs = @("image", "--config", "/workspace/deploy/ci/trivy.yaml")
        if ($allowHttpRegistry) {
            $trivyArgs += "--insecure"
        }
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
    $sbomName = $item.Image.Replace("/", "-") + ".cdx.json"
    $sbomContainerPath = "/evidence/sbom/$sbomName"
    try {
        $sbomArgs = @("image", "--format", "cyclonedx", "--output", $sbomContainerPath)
        if ($allowHttpRegistry) {
            $sbomArgs += "--insecure"
        }
        $sbomArgs += $ref
        Invoke-ComposeTool -Tool "trivy" -ToolArgs $sbomArgs -Context "生成 SBOM $ref" *>> $scanLog
    } catch {
        $blockedItems.Add([pscustomobject]@{
            image    = $item.Image
            digest   = $item.Digest
            reason   = "SBOM 生成失败: $($_.Exception.Message)"
            manifest = ""
        })
        Write-Warning "Blocking attestation ${ref}: SBOM 生成失败"
        continue
    }
    try {
        $cosignSignArgs = @(
            "sign", "--yes", "--key", "/cosign/cosign.key", "--tlog-upload=false",
            "--use-signing-config=false"
        )
        if ($allowHttpRegistry) {
            $cosignSignArgs += "--allow-http-registry"
        }
        $cosignSignArgs += $ref
        Invoke-ComposeTool -Tool "cosign" -ToolArgs $cosignSignArgs -Context "Cosign 签名 $ref" *>> $signLog

        $cosignAttestArgs = @(
            "attest", "--yes", "--key", "/cosign/cosign.key", "--type", "cyclonedx",
            "--predicate", $sbomContainerPath, "--use-signing-config=false"
        )
        if ($allowHttpRegistry) {
            $cosignAttestArgs += "--allow-http-registry"
        }
        $cosignAttestArgs += $ref
        Invoke-ComposeTool -Tool "cosign" -ToolArgs $cosignAttestArgs -Context "Cosign SBOM 证明 $ref" *>> $signLog
    } catch {
        $blockedItems.Add([pscustomobject]@{
            image    = $item.Image
            digest   = $item.Digest
            reason   = "Cosign 签名或 SBOM 证明失败: $($_.Exception.Message)"
            manifest = ""
        })
        Write-Warning "Blocking attestation ${ref}: Cosign 签名或 SBOM 证明失败"
        continue
    }
    try {
        $cosignVerifyArgs = @("verify", "--key", "/cosign/cosign.pub", "--insecure-ignore-tlog=true")
        if ($allowHttpRegistry) {
            $cosignVerifyArgs += "--allow-http-registry"
        }
        $cosignVerifyArgs += $ref
        Invoke-ComposeTool -Tool "cosign" -ToolArgs $cosignVerifyArgs -Context "Cosign 验签 $ref" *>> $signLog

        $cosignVerifyAttestationArgs = @(
            "verify-attestation", "--key", "/cosign/cosign.pub", "--type", "cyclonedx",
            "--insecure-ignore-tlog=true"
        )
        if ($allowHttpRegistry) {
            $cosignVerifyAttestationArgs += "--allow-http-registry"
        }
        $cosignVerifyAttestationArgs += $ref
        Invoke-ComposeTool -Tool "cosign" -ToolArgs $cosignVerifyAttestationArgs -Context "Cosign SBOM 验证 $ref" *>> $signLog
    } catch {
        $blockedItems.Add([pscustomobject]@{
            image    = $item.Image
            digest   = $item.Digest
            reason   = "Cosign 签名或 SBOM 验证失败: $($_.Exception.Message)"
            manifest = ""
        })
        Write-Warning "Blocking attestation ${ref}: Cosign 签名或 SBOM 验证失败"
        continue
    }
    $attestations.Add([pscustomobject]@{
        image_url       = $ref
        digest          = $item.Digest
        cosign_verified = $true
        trivy_status    = "passed"
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
    "sign_log=$signLog",
    "sbom_dir=$sbomDir"
)
Write-TextFile -Path $summaryPath -Lines $summary
Write-Output ($summary -join "`n")
if ($blockedItems.Count -gt 0) {
    Write-Error "镜像供应链证明存在阻塞项,详见 $blockedPath"
    exit 1
}
