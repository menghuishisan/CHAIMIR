# 本脚本校验静态 Kustomize overlay 的平台服务镜像与权威 digest 锁完全一致。
param(
    [string]$RepoRoot = "",
    [Parameter(Mandatory = $true)]
    [string]$OverlayPath,
    [string]$DigestLockPath = ""
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
}
if ([string]::IsNullOrWhiteSpace($DigestLockPath)) {
    $DigestLockPath = Join-Path $RepoRoot "images\image-digests.lock"
}
$resolvedOverlayPath = (Resolve-Path -LiteralPath $OverlayPath).Path
$overlayKustomization = Join-Path $resolvedOverlayPath "kustomization.yaml"
if (-not (Test-Path -LiteralPath $overlayKustomization)) {
    throw "缺少 overlay kustomization.yaml: $resolvedOverlayPath"
}
if (Get-Content -LiteralPath $overlayKustomization | Where-Object { $_ -match "^images:\s*$" }) {
    throw "overlay 不得声明 images,镜像引用必须归 base 或资源所属 component"
}

Import-Module (Join-Path $RepoRoot "images\lib\ImageMetadata.psm1") -Force
$digests = Read-ChaimirDigestLock -Path $DigestLockPath -Required

# Read-ImageRegistry 从统一非密配置源读取规范镜像仓库地址。
function Read-ImageRegistry {
    param([string]$Path)
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -match "^IMAGE_REGISTRY=(.+)$") {
            $value = $Matches[1].Trim()
            if ($value -notmatch "^[^\s/:]+(?::[0-9]+)?(?:/[^\s/]+)*$") {
                throw "IMAGE_REGISTRY 格式非法"
            }
            return $value.TrimEnd("/")
        }
    }
    throw "统一配置缺少 IMAGE_REGISTRY"
}

# Resolve-LockedDigest 保证部署集合中的每个服务都有合法不可变 digest。
function Resolve-LockedDigest {
    param([string]$LogicalImage)
    if (-not $digests.ContainsKey($LogicalImage)) {
        throw "权威镜像锁缺少 $LogicalImage"
    }
    $digest = [string]$digests[$LogicalImage]
    if ($digest -notmatch "^sha256:[0-9a-f]{64}$") {
        throw "权威镜像锁中的 $LogicalImage digest 非法"
    }
    return $digest
}

$registry = Read-ImageRegistry -Path (Join-Path $RepoRoot "deploy\config\chaimir.env")
$services = @("service/backend", "service/frontend", "service/migrate", "service/cron")
$expected = @{}
foreach ($logicalImage in $services) {
    $expected[$logicalImage] = "$registry/$logicalImage@$(Resolve-LockedDigest -LogicalImage $logicalImage)"
}

# 直接渲染仓库静态 overlay,禁止用临时清单或宽松加载限制改变资源来源。
$rendered = @(& kubectl kustomize $resolvedOverlayPath 2>&1)
if ($LASTEXITCODE -ne 0) {
    throw "Kustomize overlay 渲染失败: $resolvedOverlayPath"
}
$renderedImages = [System.Collections.Generic.List[string]]::new()
foreach ($line in $rendered) {
    if ([string]$line -match '^\s*image:\s*[''"]?([^''"#\s]+)') {
        $renderedImages.Add($Matches[1])
    }
}
if ($renderedImages.Count -eq 0) {
    throw "overlay 未渲染出任何镜像"
}

foreach ($logicalImage in $services) {
    $expectedReference = $expected[$logicalImage]
    if ($expectedReference -notin $renderedImages) {
        throw "overlay 镜像与权威锁不一致: $logicalImage"
    }
}
foreach ($reference in $renderedImages) {
    if ($reference -match "^$([regex]::Escape($registry))/service/([^@:\s]+)(?:[@:].*)?$") {
        $logicalImage = "service/$($Matches[1])"
        if (-not $expected.ContainsKey($logicalImage) -or $reference -ne $expected[$logicalImage]) {
            throw "overlay 存在未锁定或不一致的平台服务镜像: $logicalImage"
        }
    }
    if ($reference -match "(?:^|:)placeholder$") {
        throw "overlay 仍包含 placeholder 镜像"
    }
}

Write-Output "Overlay image lock verified: $resolvedOverlayPath"
