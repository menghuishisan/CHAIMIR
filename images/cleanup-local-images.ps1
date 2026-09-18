# 清理 Chaimir 本机 Docker 临时镜像,只删除未被正式锁、候选锁或容器使用的项目对象。
param(
    [string]$RepoRoot = "",
    [string]$Registry = $env:IMAGE_REGISTRY,
    [string]$DigestLockPath = "",
    [string[]]$CandidateDigestLockPath = @(),
    [string[]]$KeepTags = @(),
    [switch]$Apply
)

$ErrorActionPreference = "Stop"
# Docker 的“目标不存在”通过退出码表达；不能让 PowerShell 将原生 stderr 提前升级为脚本异常。
if ($null -ne (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue)) {
    $PSNativeCommandUseErrorActionPreference = $false
}
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
# KeepTags 既支持 PowerShell 数组,也接受脚本边界传入的逗号串;统一展开后再比较,避免子进程参数序列化丢失保护标签。
$KeepTags = @($KeepTags | ForEach-Object {
    ([string]$_ -split ',') | ForEach-Object { $_.Trim() } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
}) | Select-Object -Unique
$candidateLockPaths = @($CandidateDigestLockPath | ForEach-Object {
    ([string]$_ -split ',') | ForEach-Object { $_.Trim() } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
}) | Select-Object -Unique
$candidateRefs = [System.Collections.Generic.List[string]]::new()
foreach ($candidatePath in $candidateLockPaths) {
    if ([string]::IsNullOrWhiteSpace($candidatePath)) {
        continue
    }
    $candidateItems = Read-ChaimirDigestLock -Path $candidatePath -Required
    foreach ($entry in $candidateItems.GetEnumerator()) {
        $candidateRefs.Add("$registryHost/$($entry.Key)@$($entry.Value)")
    }
}

$protectedRefs = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
foreach ($entry in $formal.GetEnumerator()) {
    [void]$protectedRefs.Add("$registryHost/$($entry.Key)@$($entry.Value)")
}
foreach ($candidateRef in $candidateRefs) {
    [void]$protectedRefs.Add($candidateRef)
}

function Test-ChaimirDockerImageReference([string]$Reference) {
    $previousErrorActionPreference = $ErrorActionPreference
    try {
        # Docker 将“本机未缓存摘要”写到 stderr;这只是清理筛选结果,不是脚本异常。
        $ErrorActionPreference = "Continue"
        docker image inspect $Reference 2>$null | Out-Null
        return $LASTEXITCODE -eq 0
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
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

# 这些标签只代表本地构建/回归过程中的临时引用,不属于发布契约。
# refactor/prepull 也必须纳入清理,否则旧重构产物会长期占用本机空间。
$temporaryTagPattern = '^(e2e|candidate|refresh|local|refactor|prepull|[0-9]{8}-(regression-fix|private-))'
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
    if (Test-ChaimirDockerImageReference $immutableRef) {
        [void]$availableProtectedRefs.Add($immutableRef)
    } else {
        $unavailableProtectedCount++
    }
}
# 选择性构建/重建时,未受影响的正式摘要可能只存在于远端;清理只作用于本机对象,
# 缺失的远端引用没有可删除的本机对象,因此不能把它误判为清理失败或触发隐式下载。
$missingProtectedRefs = @($protectedRefs | ForEach-Object { [string]$_ } | Where-Object {
    -not $availableProtectedRefs.Contains($_)
})
if ($missingProtectedRefs.Count -gt 0) {
    Write-Warning ("本机未缓存正式/候选不可变 digest,跳过这些远端引用的本机保护检查: " + ($missingProtectedRefs -join ', '))
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
    # Docker Desktop 对按 digest 导入的镜像可能同时把不可变引用暴露在 RepoTags 和 RepoDigests;
    # 两者都必须参与保护判断,否则清理会把锁定镜像误判成 dangling 对象。
    $repoDigests = @($inspect[0].RepoDigests)
    $imageReferences = @($inspect[0].RepoTags) + $repoDigests
    $protectedImageRefs = [System.Collections.Generic.List[string]]::new()
    foreach ($imageReference in $imageReferences) {
        if ($protectedRefs.Contains([string]$imageReference)) {
            $protectedImageRefs.Add([string]$imageReference)
        }
    }
    if ($protectedImageRefs.Count -gt 0) {
        # 用户未授权下载;删除最后一个本地引用后不得自动 pull 恢复 digest。
        # 这类对象保留到下一次显式下载/重建流程,避免清理脚本产生网络副作用。
        $skipped.Add("$ref (formal/candidate digest protected; no download)")
        continue
    }

    if ($Apply) {
        # A dangling project reference can share the same image ID with an
        # upstream digest reference. Removing the short ID asks Docker to
        # delete the whole image and fails with a multi-repository conflict.
        # Remove only the exact project digest references instead.
        $removableRefs = @($repoDigests | Where-Object {
            $candidateRef = [string]$_
            $candidateRef -match "^$([regex]::Escape($registryHost))/" -and
                -not $protectedRefs.Contains($candidateRef)
        })
        if ($removableRefs.Count -eq 0) {
            $removableRefs = @($ref)
        } elseif ($ref -notin $removableRefs) {
            # 临时 tag 与 RepoDigest 是两个独立引用,必须同时删除,否则旧候选 tag 会继续占用本机镜像空间。
            $removableRefs += $ref
        }
        foreach ($removableRef in $removableRefs) {
            # 删除前重新确认精确引用;并发清理已经删除的目标视为幂等成功。
            if (-not (Test-ChaimirDockerImageReference $removableRef)) {
                $skipped.Add("$removableRef (already absent)")
                continue
            }
            docker image rm $removableRef | Out-Null
            if ($LASTEXITCODE -ne 0) {
                if (-not (Test-ChaimirDockerImageReference $removableRef)) {
                    $skipped.Add("$removableRef (removed concurrently)")
                    continue
                }
                throw "删除临时镜像失败: $removableRef"
            }
            $removed.Add([string]$removableRef)
        }
    } else {
        $removed.Add("$ref (dry-run)")
    }
}

# 清理前后都验证正式和候选不可变引用,防止误删最后一个 digest 引用。
foreach ($immutableRef in $protectedRefSnapshot) {
    if (-not (Test-ChaimirDockerImageReference $immutableRef)) {
        throw "清理后不可变镜像引用不可访问: $immutableRef"
    }
}

Write-Host ("Image cleanup {0}. removed={1} skipped={2} unavailable_protected={3}" -f ($(if ($Apply) { 'applied' } else { 'dry-run' }), $removed.Count, $skipped.Count, $unavailableProtectedCount))
foreach ($entry in $removed) { Write-Host "  removed: $entry" }
foreach ($entry in $skipped) { Write-Host "  kept: $entry" }
