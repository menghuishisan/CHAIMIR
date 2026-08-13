# 清理 Chaimir 本机 Docker 临时镜像,只删除未被正式锁、候选锁或容器使用的项目对象。
param(
    [string]$RepoRoot = "",
    [string]$Registry = $env:IMAGE_REGISTRY,
    [string]$DigestLockPath = "",
    [string]$CandidateDigestLockPath = "",
    [string[]]$KeepTags = @(),
    [switch]$Apply
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}
if ([string]::IsNullOrWhiteSpace($Registry)) {
    throw "Registry 不能为空;清理必须显式绑定项目的 canonical registry"
}
if ([string]::IsNullOrWhiteSpace($DigestLockPath)) {
    $DigestLockPath = Join-Path $RepoRoot "images\image-digests.lock"
}
Import-Module (Join-Path $PSScriptRoot "lib\ImageMetadata.psm1") -Force

$registryHost = ($Registry -replace "^https?://", "").TrimEnd('/')
$catalog = Get-ChaimirImageCatalog -ImagesRoot (Join-Path $RepoRoot "images")
$formal = Read-ChaimirDigestLock -Path $DigestLockPath -Required
$candidate = @{}
if (-not [string]::IsNullOrWhiteSpace($CandidateDigestLockPath)) {
    $candidate = Read-ChaimirDigestLock -Path $CandidateDigestLockPath -Required
}

$protectedRefs = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
foreach ($entry in @($formal.GetEnumerator()) + @($candidate.GetEnumerator())) {
    [void]$protectedRefs.Add("$registryHost/$($entry.Key)@$($entry.Value)")
}

# 运行中或已停止但仍保留的容器都可能是验收证据,其镜像不得由本机清理删除。
$protectedIds = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
$containerIds = @(docker ps -aq)
foreach ($containerId in $containerIds) {
    if ([string]::IsNullOrWhiteSpace($containerId)) { continue }
    $inspect = docker inspect $containerId | ConvertFrom-Json
    if ($inspect.Count -gt 0) {
        [void]$protectedIds.Add([string]$inspect[0].Image)
    }
}

$temporaryTagPattern = '^(e2e|candidate|refresh|local)-'
$images = @(docker image ls --format '{{json .}}' | ForEach-Object {
    if (-not [string]::IsNullOrWhiteSpace($_)) { $_ | ConvertFrom-Json }
})
$removed = [System.Collections.Generic.List[string]]::new()
$skipped = [System.Collections.Generic.List[string]]::new()

# 选择性构建时本机可能没有未受影响的正式镜像;只保护本机确实存在的不可变引用。
$availableProtectedRefs = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
$unavailableProtectedCount = 0
foreach ($candidateRef in @($protectedRefs | ForEach-Object { [string]$_ })) {
    $immutableRef = [string]$candidateRef
    docker image inspect $immutableRef | Out-Null
    if ($LASTEXITCODE -eq 0) {
        [void]$availableProtectedRefs.Add($immutableRef)
    } else {
        $unavailableProtectedCount++
    }
}
$protectedRefs = $availableProtectedRefs
$protectedRefSnapshot = @($protectedRefs | ForEach-Object { [string]$_ })

foreach ($image in $images) {
    $repository = [string]$image.Repository
    $tag = [string]$image.Tag
    if ([string]::IsNullOrWhiteSpace($repository) -or $repository -eq '<none>') { continue }
    if ($repository -notmatch "^$([regex]::Escape($registryHost))/") { continue }
    $logical = $repository.Substring($registryHost.Length + 1)
    if (-not $catalog.ContainsKey($logical)) { continue }
    $ref = if ($tag -eq '<none>') { [string]$image.ID } else { "$repository`:$tag" }
    $isDangling = $tag -eq '<none>'
    $isTemporary = $tag -match $temporaryTagPattern
    if (-not $isDangling -and -not $isTemporary) { continue }
    if ($KeepTags -contains $tag) {
        $skipped.Add("$ref (kept current tag)")
        continue
    }

    $inspect = @(docker image inspect $ref | ConvertFrom-Json)
    if ($inspect.Count -eq 0) { continue }
    $imageId = [string]$inspect[0].Id
    if ($protectedIds.Contains($imageId)) {
        $skipped.Add("$ref (referenced by container)")
        continue
    }
    $repoDigests = @($inspect[0].RepoDigests)
    $protectedImageRefs = [System.Collections.Generic.List[string]]::new()
    foreach ($repoDigest in $repoDigests) {
        if ($protectedRefs.Contains([string]$repoDigest)) {
            $protectedImageRefs.Add([string]$repoDigest)
        }
    }
    if ($protectedImageRefs.Count -gt 0 -and $isDangling) {
        $skipped.Add("$ref (formal/candidate digest protected)")
        continue
    }

    if ($Apply) {
        docker image rm $ref | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "删除临时镜像失败: $ref"
        }
        # Docker 删除最后一个 tag 时会一并删除本地 RepoDigest;按不可变引用回拉恢复唯一引用。
        foreach ($protectedImageRef in $protectedImageRefs) {
            docker pull $protectedImageRef | Out-Null
            if ($LASTEXITCODE -ne 0) {
                throw "恢复正式/候选 digest 引用失败: $protectedImageRef"
            }
        }
        $removed.Add($ref)
    } else {
        $removed.Add("$ref (dry-run)")
    }
}

# 清理前后都验证正式和候选不可变引用,防止误删最后一个 digest 引用。
foreach ($immutableRef in $protectedRefSnapshot) {
    docker image inspect $immutableRef | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "清理后不可变镜像引用不可访问: $immutableRef"
    }
}

Write-Host ("Image cleanup {0}. removed={1} skipped={2} unavailable_protected={3}" -f ($(if ($Apply) { 'applied' } else { 'dry-run' }), $removed.Count, $skipped.Count, $unavailableProtectedCount))
foreach ($entry in $removed) { Write-Host "  removed: $entry" }
foreach ($entry in $skipped) { Write-Host "  kept: $entry" }
