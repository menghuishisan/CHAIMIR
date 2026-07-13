# 本脚本在无任何私钥的环境中校验镜像 manifest、目录、Dockerfile 与 digest lock 一致性。
param(
    [string]$RepoRoot = "",
    [string]$DigestLockPath = "",
    [string]$DeployConfigPath = ""
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}
if ([string]::IsNullOrWhiteSpace($DigestLockPath)) {
    $DigestLockPath = Join-Path $RepoRoot "images\image-digests.lock"
}
if ([string]::IsNullOrWhiteSpace($DeployConfigPath)) {
    $DeployConfigPath = Join-Path $RepoRoot "deploy\config\chaimir.env"
}

Import-Module (Join-Path $PSScriptRoot "lib\ImageMetadata.psm1") -Force
$imagesRoot = (Resolve-Path (Join-Path $RepoRoot "images")).Path
$catalog = Get-ChaimirImageCatalog -ImagesRoot $imagesRoot
$lock = Read-ChaimirDigestLock -Path $DigestLockPath -Required
$allowedCategories = @("service", "runtime", "infra", "tool", "judger", "sim", "sidecar", "init", "base", "middleware", "observability", "ingress")
$buildSourceTypes = @("platform-built", "thin-wrapper", "build-base")
$internalBuildArguments = Get-ChaimirInternalImageBuildArguments
$errors = [System.Collections.Generic.List[string]]::new()

# Test-DockerfileImmutableSources 拒绝可变字面量基础镜像和可静默生效的镜像参数默认值。
function Test-DockerfileImmutableSources {
    param(
        [string]$Image,
        [string]$DockerfilePath,
        [string]$ManifestPath
    )
    $manifestContents = Get-Content -LiteralPath $ManifestPath -Raw
    $lineNumber = 0
    foreach ($line in Get-Content -LiteralPath $DockerfilePath) {
        $lineNumber++
        if ($line -match "^\s*FROM\s+([^\s]+)") {
            $source = $Matches[1]
            if ($source -ne "scratch" -and $source -notmatch "^\$\{" -and $source -notmatch "@sha256:[0-9a-f]{64}$") {
                $errors.Add("Dockerfile 字面量基础镜像必须锁定 digest: $Image`:$lineNumber -> $source")
            }
            elseif ($source -match "@(sha256:[0-9a-f]{64})$" -and $manifestContents -notmatch [regex]::Escape($Matches[1])) {
                $errors.Add("Dockerfile 基础镜像 digest 未同步到 manifest: $Image`:$lineNumber -> $source")
            }
        }
        if ($line -match "^\s*ARG\s+([A-Z0-9_]+_IMAGE)(?:=(.*))?\s*$") {
            $argument = $Matches[1]
            $default = $Matches[2]
            if ([string]::IsNullOrWhiteSpace($default) -or $default -notmatch "^invalid\.invalid/.+:required$") {
                $errors.Add("Dockerfile 镜像参数必须使用显式失败默认值: $Image`:$lineNumber -> $argument")
            }
            if (-not $internalBuildArguments.Contains($argument) -and $argument -ne "NGINX_RUNTIME_IMAGE") {
                $errors.Add("Dockerfile 镜像参数没有统一注入映射: $Image`:$lineNumber -> $argument")
            }
        }
    }
}

foreach ($entry in $catalog.Values) {
    $relative = $entry.Manifest.Substring($imagesRoot.Length).TrimStart("\", "/").Replace("\", "/")
    $parts = $relative.Split("/")
    if ($parts.Count -ne 3 -or $parts[2] -ne "manifest.yaml") {
        $errors.Add("manifest 目录必须是 images/<category>/<name>/manifest.yaml: $relative")
        continue
    }
    $category = $parts[0]
    $name = $parts[1]
    if ($category -notin $allowedCategories) {
        $errors.Add("镜像分类非法: $relative -> $category")
    }
    if ($entry.Image -ne "$category/$name") {
        $errors.Add("目录与逻辑镜像名不一致: $relative -> $($entry.Image)")
    }
    $lines = Get-Content -LiteralPath $entry.Manifest
    $manifestCategory = Get-ChaimirTopLevelYamlValue -Lines $lines -Key "category"
    $manifestName = Get-ChaimirTopLevelYamlValue -Lines $lines -Key "name"
    if ($manifestCategory -ne $category -or $manifestName -ne $name) {
        $errors.Add("manifest category/name 与目录不一致: $relative")
    }
    $readmePath = Join-Path (Split-Path -Parent $entry.Manifest) "README.md"
    if (-not (Test-Path -LiteralPath $readmePath -PathType Leaf)) {
        $errors.Add("镜像目录缺少 README.md: $relative")
    }

    if ($entry.SourceType -in $buildSourceTypes) {
        $build = Get-ChaimirYamlBlock -Path $entry.Manifest -BlockName "build"
        $contextValue = Get-ChaimirYamlValue -Lines $build -Key "context"
        $dockerfileValue = Get-ChaimirYamlValue -Lines $build -Key "dockerfile"
        $buildPaths = Resolve-ChaimirImageBuildPaths -RepoRoot $RepoRoot -ManifestPath $entry.Manifest -ContextValue $contextValue -DockerfileValue $dockerfileValue
        $contextPath = $buildPaths.Context
        $dockerfilePath = $buildPaths.Dockerfile
        if (-not (Test-Path -LiteralPath $contextPath -PathType Container)) {
            $errors.Add("构建上下文不存在: $($entry.Image) -> $contextPath")
        }
        if (-not (Test-Path -LiteralPath $dockerfilePath -PathType Leaf)) {
            $errors.Add("Dockerfile 不存在: $($entry.Image) -> $dockerfilePath")
        } else {
            Test-DockerfileImmutableSources -Image $entry.Image -DockerfilePath $dockerfilePath -ManifestPath $entry.Manifest
            if ((Get-Content -LiteralPath $dockerfilePath -Raw) -match "(?m)^\s*ARG\s+NGINX_RUNTIME_IMAGE\b") {
                $upstream = Get-ChaimirYamlBlock -Path $entry.Manifest -BlockName "upstream"
                $runtimeImage = Get-ChaimirYamlValue -Lines $upstream -Key "runtime"
                $runtimeDigest = Get-ChaimirYamlValue -Lines $upstream -Key "runtime_digest"
                if ([string]::IsNullOrWhiteSpace($runtimeImage) -or $runtimeImage.Contains("@") -or $runtimeDigest -notmatch "^sha256:[0-9a-f]{64}$") {
                    $errors.Add("NGINX_RUNTIME_IMAGE 必须由 upstream.runtime 与 upstream.runtime_digest 组成: $($entry.Image)")
                }
            }
        }
    } else {
        $unexpectedDockerfile = Join-Path (Split-Path -Parent $entry.Manifest) "Dockerfile"
        if (Test-Path -LiteralPath $unexpectedDockerfile -PathType Leaf) {
            $errors.Add("upstream-pinned 镜像不得维护 Dockerfile: $relative")
        }
    }

    if (-not $entry.Deployable -and $lock.ContainsKey($entry.Image)) {
        $errors.Add("已阻断镜像不得保留在 digest lock: $($entry.Image)")
    }
}

foreach ($image in $lock.Keys) {
    if (-not $catalog.ContainsKey($image)) {
        $errors.Add("digest lock 包含未登记镜像: $image")
    }
    elseif (-not $catalog[$image].Deployable) {
        $errors.Add("digest lock 包含不可部署镜像: $image")
    }
}

# 本地/私有化静态证明不得引用已阻断、已删除或与正式锁不一致的旧 digest。
$attestationLine = Get-Content -LiteralPath $DeployConfigPath | Where-Object { $_ -match "^SANDBOX_IMAGE_ATTESTATIONS_JSON=" }
if (@($attestationLine).Count -ne 1) {
    $errors.Add("$DeployConfigPath 必须且只能声明一次 SANDBOX_IMAGE_ATTESTATIONS_JSON")
} else {
    $attestationJSON = $attestationLine.Substring($attestationLine.IndexOf("=") + 1)
    $seenAttestations = @{}
    $parsedAttestations = ConvertFrom-Json -InputObject $attestationJSON
    foreach ($item in $parsedAttestations) {
        $imageURL = [string]$item.image_url
        if ($imageURL -notmatch "^.+/([^/]+/[^/@]+)@(sha256:[0-9a-f]{64})$") {
            $errors.Add("SANDBOX_IMAGE_ATTESTATIONS_JSON 包含非法镜像引用")
            continue
        }
        $logical = $Matches[1]
        $digest = $Matches[2]
        if ($seenAttestations.ContainsKey($logical)) {
            $errors.Add("SANDBOX_IMAGE_ATTESTATIONS_JSON 重复登记: $logical")
            continue
        }
        $seenAttestations[$logical] = $true
        if (-not ($lock.ContainsKey($logical)) -or $lock[$logical] -ne $digest) {
            $errors.Add("SANDBOX_IMAGE_ATTESTATIONS_JSON 与正式 digest lock 不一致: $logical")
        }
        if (-not ([bool]$item.cosign_verified) -or -not [string]::Equals(([string]$item.trivy_status), "passed", [System.StringComparison]::OrdinalIgnoreCase)) {
            $errors.Add("SANDBOX_IMAGE_ATTESTATIONS_JSON 包含未通过门禁的条目: $logical")
        }
    }
}

if ($errors.Count -gt 0) {
    Write-Error ("镜像元数据校验失败:`n" + ($errors -join "`n"))
    exit 1
}

Write-Output "Validated $($catalog.Count) manifests and $($lock.Count) immutable digest entries."
