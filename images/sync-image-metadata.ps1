# 本脚本合并已验证镜像 digest 片段,并同步仓库内唯一的静态镜像引用。
param(
    [string]$RepoRoot = "",
    [Parameter(Mandatory = $true)]
    [string]$FragmentsPath,
    [string]$DigestLockPath = "",
    [string]$LocalOverlayPath = "",
    [string[]]$EnvPaths = @(),
    [string]$SimAdapterEnvKey = "SIM_BACKEND_STDIO_ADAPTERS_JSON",
    [string[]]$AttestationEnvPaths = @(),
    [string]$AttestationEnvKey = "SANDBOX_IMAGE_ATTESTATIONS_JSON",
    [string]$RegistryEnvKey = "SUPPLY_CHAIN_REGISTRY"
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}
Import-Module (Join-Path $PSScriptRoot "lib\ImageMetadata.psm1") -Force

if ([string]::IsNullOrWhiteSpace($DigestLockPath)) {
    $DigestLockPath = Join-Path $RepoRoot "images\image-digests.lock"
}
if ([string]::IsNullOrWhiteSpace($LocalOverlayPath)) {
    $LocalOverlayPath = Join-Path $RepoRoot "deploy\overlays\local-dev\kustomization.yaml"
}
if ($EnvPaths.Count -eq 0) {
    $EnvPaths = @(
        (Join-Path $RepoRoot "backend\.env.example"),
        (Join-Path $RepoRoot "deploy\config\chaimir.env")
    )
}
if ($AttestationEnvPaths.Count -eq 0) {
    $AttestationEnvPaths = @((Join-Path $RepoRoot "deploy\config\chaimir.env"))
}

# Read-VerifiedFragments 读取当前流水线产出的锁片段,同一镜像出现不同 digest 时拒绝晋升。
function Read-VerifiedFragments {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "缺少镜像 digest 片段目录: $Path"
    }
    $files = @(Get-ChildItem -LiteralPath $Path -Recurse -File -Filter "*.lock" | Sort-Object FullName)
    if ($files.Count -eq 0) {
        throw "镜像 digest 片段目录为空: $Path"
    }
    $items = @{}
    foreach ($file in $files) {
        $fragment = Read-ChaimirDigestLock -Path $file.FullName -Required
        foreach ($image in $fragment.Keys) {
            if ($items.ContainsKey($image) -and $items[$image] -ne $fragment[$image]) {
                throw "不同流水线为镜像 $image 产出了冲突 digest"
            }
            $items[$image] = $fragment[$image]
        }
    }
    return $items
}

# Write-TextLinesPreservingEncoding 保留目标文件的 UTF-8 BOM、换行符和末尾换行约定。
function Write-TextLinesPreservingEncoding {
    param(
        [string]$Path,
        [System.Collections.Generic.List[string]]$Lines
    )
    $bytes = [System.IO.File]::ReadAllBytes($Path)
    $hasUTF8BOM = $bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF
    $offset = if ($hasUTF8BOM) { 3 } else { 0 }
    $text = [System.Text.Encoding]::UTF8.GetString($bytes, $offset, $bytes.Length - $offset)
    $newline = if ($text.Contains("`r`n")) { "`r`n" } else { "`n" }
    $hasTrailingNewline = $text.EndsWith("`n")
    $contents = [string]::Join($newline, $Lines)
    if ($hasTrailingNewline) {
        $contents += $newline
    }
    [System.IO.File]::WriteAllText($Path, $contents, [System.Text.UTF8Encoding]::new($hasUTF8BOM))
}

# Get-KustomizeLogicalImage 将部署占位名映射到镜像治理目录的逻辑名。
function Get-KustomizeLogicalImage {
    param([string]$Name)
    if ($Name -notmatch "^chaimir/(.+)$") {
        return $null
    }
    $suffix = $Matches[1]
    if ($suffix -in @("backend", "frontend", "migrate", "cron")) {
        return "service/$suffix"
    }
    return $suffix
}

# Update-KustomizeDigests 更新 local-dev images 条目,首次晋升时将旧 tag 原位替换为 digest。
function Update-KustomizeDigests {
    param(
        [string]$Path,
        [hashtable]$Digests
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "缺少 local-dev Kustomize 文件: $Path"
    }
    $source = @(Get-Content -LiteralPath $Path)
    $output = [System.Collections.Generic.List[string]]::new()
    $index = 0
    while ($index -lt $source.Count) {
        if ($source[$index] -notmatch "^\s+-\s+name:\s+([^#\s]+)\s*$") {
            $output.Add($source[$index])
            $index++
            continue
        }

        $logical = Get-KustomizeLogicalImage -Name $Matches[1]
        $end = $index + 1
        while ($end -lt $source.Count -and $source[$end] -notmatch "^\s+-\s+name:\s+") {
            $end++
        }
        $block = [System.Collections.Generic.List[string]]::new()
        for ($blockIndex = $index; $blockIndex -lt $end; $blockIndex++) {
            $block.Add($source[$blockIndex])
        }

        if (-not [string]::IsNullOrWhiteSpace($logical) -and $Digests.ContainsKey($logical)) {
            $digestIndex = -1
            $tagIndex = -1
            $newNameIndex = -1
            for ($blockIndex = 1; $blockIndex -lt $block.Count; $blockIndex++) {
                if ($block[$blockIndex] -match "^\s+newName:\s+") {
                    $newNameIndex = $blockIndex
                }
                elseif ($block[$blockIndex] -match "^\s+digest:\s+") {
                    $digestIndex = $blockIndex
                }
                elseif ($block[$blockIndex] -match "^\s+newTag:\s+") {
                    $tagIndex = $blockIndex
                }
            }
            $digestLine = "    digest: $($Digests[$logical])"
            if ($digestIndex -ge 0) {
                $block[$digestIndex] = $digestLine
            }
            elseif ($tagIndex -ge 0) {
                $block[$tagIndex] = $digestLine
            }
            elseif ($newNameIndex -ge 0) {
                $block.Insert($newNameIndex + 1, $digestLine)
            }
            else {
                throw "$Path 中镜像 $logical 缺少 newName"
            }
        }

        foreach ($line in $block) {
            $output.Add($line)
        }
        $index = $end
    }
    Write-TextLinesPreservingEncoding -Path $Path -Lines $output
}

# Update-SimAdapterDigests 解析 env 中的 JSON 能力目录并替换已晋升镜像的 digest。
function Update-SimAdapterDigests {
    param(
        [string]$Path,
        [string]$Key,
        [hashtable]$Digests
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "缺少配置文件: $Path"
    }
    $lines = [System.Collections.Generic.List[string]]::new()
    foreach ($line in Get-Content -LiteralPath $Path) {
        $lines.Add($line)
    }
    $matched = $false
    for ($index = 0; $index -lt $lines.Count; $index++) {
        if ($lines[$index] -notmatch "^$([regex]::Escape($Key))=(.*)$") {
            continue
        }
        if ($matched) {
            throw "$Path 中 $Key 重复"
        }
        $matched = $true
        $jsonValue = $Matches[1]
        $parsedProfiles = ConvertFrom-Json -InputObject $jsonValue
        $profiles = [System.Collections.Generic.List[object]]::new()
        foreach ($parsedProfile in $parsedProfiles) {
            $profiles.Add($parsedProfile)
        }
        if ($profiles.Count -eq 0) {
            throw "$Path 中 $Key 不能为空数组"
        }
        foreach ($profile in $profiles) {
            $imageRef = [string]$profile.image
            if ($imageRef -notmatch "^(.+/)([^/]+/[^/@]+)@sha256:[0-9a-f]{64}$") {
                throw "$Path 中 $Key 包含非法镜像引用"
            }
            $prefix = $Matches[1]
            $logical = $Matches[2]
            if ($Digests.ContainsKey($logical)) {
                $profile.image = "$prefix$logical@$($Digests[$logical])"
            }
        }
        $json = ConvertTo-Json -InputObject $profiles.ToArray() -Compress -Depth 32
        $lines[$index] = "$Key=$json"
    }
    if (-not $matched) {
        throw "$Path 缺少 $Key"
    }
    Write-TextLinesPreservingEncoding -Path $Path -Lines $lines
}

# Update-ImageAttestations 只用已验证片段更新证明,并清除已离开权威锁的旧项。
function Update-ImageAttestations {
    param(
        [string]$Path,
        [string]$Key,
        [string]$RegistryKey,
        [hashtable]$Digests,
        [hashtable]$VerifiedDigests
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "缺少配置文件: $Path"
    }
    $lines = [System.Collections.Generic.List[string]]::new()
    foreach ($line in Get-Content -LiteralPath $Path) {
        $lines.Add($line)
    }
    $keyIndex = -1
    $registry = ""
    for ($index = 0; $index -lt $lines.Count; $index++) {
        if ($lines[$index] -match "^$([regex]::Escape($Key))=(.*)$") {
            if ($keyIndex -ge 0) {
                throw "$Path 中 $Key 重复"
            }
            $keyIndex = $index
            $jsonValue = $Matches[1]
        }
        if ($lines[$index] -match "^$([regex]::Escape($RegistryKey))=(.+)$") {
            $registry = $Matches[1].Trim().TrimEnd("/")
        }
    }
    if ($keyIndex -lt 0) {
        throw "$Path 缺少 $Key"
    }
    if ([string]::IsNullOrWhiteSpace($registry) -or $registry.Contains("://") -or $registry.Contains("/")) {
        throw "$Path 中 $RegistryKey 必须是 registry host 或 host:port"
    }

    $byLogical = @{}
    $parsedAttestations = ConvertFrom-Json -InputObject $jsonValue
    foreach ($item in $parsedAttestations) {
        $imageURL = [string]$item.image_url
        if ($imageURL -notmatch "^(.+/)([^/]+/[^/@]+)@(sha256:[0-9a-f]{64})$") {
            throw "$Path 中 $Key 包含非法镜像引用"
        }
        $prefix = $Matches[1]
        $logical = $Matches[2]
        $digest = $Matches[3]
        if ($byLogical.ContainsKey($logical)) {
            throw "$Path 中 $Key 重复登记 $logical"
        }
        if (-not $Digests.ContainsKey($logical)) {
            continue
        }
        if ($VerifiedDigests.ContainsKey($logical)) {
            $digest = $VerifiedDigests[$logical]
            $item.image_url = "$prefix$logical@$digest"
            $item.digest = $digest
            $item.cosign_verified = $true
            $item.trivy_status = "passed"
        }
        elseif ($digest -ne $Digests[$logical] -or -not ([bool]$item.cosign_verified) -or -not [string]::Equals(([string]$item.trivy_status), "passed", [System.StringComparison]::OrdinalIgnoreCase)) {
            continue
        }
        $byLogical[$logical] = $item
    }

    foreach ($logical in $VerifiedDigests.Keys) {
        if ($byLogical.ContainsKey($logical)) {
            continue
        }
        $digest = $VerifiedDigests[$logical]
        $byLogical[$logical] = [pscustomobject][ordered]@{
            image_url       = "$registry/$logical@$digest"
            digest          = $digest
            cosign_verified = $true
            trivy_status    = "passed"
        }
    }
    $attestations = @($byLogical.Values | Sort-Object image_url)
    $lines[$keyIndex] = "$Key=$(ConvertTo-Json -InputObject $attestations -Compress -Depth 8)"
    Write-TextLinesPreservingEncoding -Path $Path -Lines $lines
}

$fragments = Read-VerifiedFragments -Path $FragmentsPath
$catalog = Get-ChaimirImageCatalog -ImagesRoot (Join-Path $RepoRoot "images")
foreach ($image in $fragments.Keys) {
    if (-not $catalog.ContainsKey($image)) {
        throw "digest 片段引用未登记镜像: $image"
    }
    if ($catalog[$image].SourceType -notin @("platform-built", "thin-wrapper", "build-base")) {
        throw "digest 片段不能覆盖非构建镜像: $image"
    }
    if (-not $catalog[$image].Deployable) {
        throw "digest 片段不能晋升 manifest 已阻断的镜像: $image"
    }
}

$merged = Read-ChaimirDigestLock -Path $DigestLockPath -Required
# 正式锁只保留当前 catalog 中仍允许部署的镜像,避免旧 manifest 状态与部署引用并存。
foreach ($image in @($merged.Keys)) {
    if (-not $catalog.ContainsKey($image) -or -not $catalog[$image].Deployable) {
        $merged.Remove($image)
    }
}
foreach ($image in $fragments.Keys) {
    $merged[$image] = $fragments[$image]
}

Write-ChaimirDigestLock -Path $DigestLockPath -Items $merged
Update-KustomizeDigests -Path $LocalOverlayPath -Digests $merged
foreach ($envPath in $EnvPaths) {
    Update-SimAdapterDigests -Path $envPath -Key $SimAdapterEnvKey -Digests $merged
}
foreach ($envPath in $AttestationEnvPaths) {
    Update-ImageAttestations -Path $envPath -Key $AttestationEnvKey -RegistryKey $RegistryEnvKey -Digests $merged -VerifiedDigests $fragments
}

Write-Host "Promoted $($fragments.Count) verified image digests; authoritative lock contains $($merged.Count) entries."
