# 本脚本从权威镜像锁生成环境部署用的临时 Kustomize digest 覆盖。
param(
    [string]$RepoRoot = "",
    [Parameter(Mandatory = $true)]
    [string]$OverlayPath,
    [Parameter(Mandatory = $true)]
    [string]$Registry,
    [Parameter(Mandatory = $true)]
    [string]$OutputPath,
    [string]$DigestLockPath = ""
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
}
if ([string]::IsNullOrWhiteSpace($DigestLockPath)) {
    $DigestLockPath = Join-Path $RepoRoot "images\image-digests.lock"
}
if ($Registry -notmatch "^[^\s/]+(?::[0-9]+)?(?:/[^\s/]+)*$") {
    throw "Registry 格式非法"
}

Import-Module (Join-Path $RepoRoot "images\lib\ImageMetadata.psm1") -Force
$digests = Read-ChaimirDigestLock -Path $DigestLockPath -Required
$services = [ordered]@{
    "chaimir/backend"  = "service/backend"
    "chaimir/frontend" = "service/frontend"
    "chaimir/migrate"  = "service/migrate"
    "chaimir/cron"     = "service/cron"
}

# Resolve-LockedDigest 保证部署集合中的每个服务都有唯一合法的不可变 digest。
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

# Get-RelativeResourcePath 生成 Kustomize 支持的相对资源路径,避免绝对目录被拒绝。
function Get-RelativeResourcePath {
    param(
        [string]$FromDirectory,
        [string]$TargetPath
    )
    $separator = [System.IO.Path]::DirectorySeparatorChar
    $fromUri = [System.Uri]::new($FromDirectory.TrimEnd("\", "/") + $separator)
    $targetUri = [System.Uri]::new((Resolve-Path -LiteralPath $TargetPath).Path)
    return [System.Uri]::UnescapeDataString($fromUri.MakeRelativeUri($targetUri).ToString())
}

$parent = Split-Path -Parent $OutputPath
if ([string]::IsNullOrWhiteSpace($parent)) {
    $parent = (Get-Location).Path
}
elseif (-not (Test-Path -LiteralPath $parent)) {
    New-Item -ItemType Directory -Path $parent | Out-Null
}
$resolvedParent = (Resolve-Path -LiteralPath $parent).Path
$relativeOverlay = Get-RelativeResourcePath -FromDirectory $resolvedParent -TargetPath $OverlayPath
$lines = [System.Collections.Generic.List[string]]::new()
$lines.Add("apiVersion: kustomize.config.k8s.io/v1beta1")
$lines.Add("kind: Kustomization")
$lines.Add("resources:")
$lines.Add("  - $relativeOverlay")
$lines.Add("images:")
foreach ($placeholder in $services.Keys) {
    $logical = $services[$placeholder]
    $digest = Resolve-LockedDigest -LogicalImage $logical
    $lines.Add("  - name: $placeholder")
    $lines.Add("    newName: $Registry/$logical")
    $lines.Add("    digest: $digest")
}

[System.IO.File]::WriteAllLines($OutputPath, $lines, [System.Text.UTF8Encoding]::new($false))
Write-Output $OutputPath
