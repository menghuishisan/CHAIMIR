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
$allowedCategories = @("service", "runtime", "infra", "tool", "judger", "sim", "sidecar", "init", "base", "middleware", "observability", "ingress", "network")
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

# Test-RuntimeCapabilityDeclaration 防止“能启动节点”被错误登记为“可编排链能力”。
function Test-RuntimeCapabilityDeclaration {
    param(
        [string]$Image,
        [string]$ManifestPath
    )
    $lines = Get-Content -LiteralPath $ManifestPath
    $contents = Get-Content -LiteralPath $ManifestPath -Raw
    $runtimeStart = -1
    for ($index = 0; $index -lt $lines.Count; $index++) {
        if ($lines[$index] -match '^runtime:\s*$') {
            $runtimeStart = $index
            break
        }
    }
    if ($runtimeStart -lt 0) {
        return
    }
    $runtimeLines = [System.Collections.Generic.List[string]]::new()
    for ($index = $runtimeStart + 1; $index -lt $lines.Count; $index++) {
        if ($lines[$index] -match '^[A-Za-z_][A-Za-z0-9_]*:\s*') {
            break
        }
        $runtimeLines.Add($lines[$index])
    }
    $levelMatch = [regex]::Match([string]::Join("`n", $runtimeLines), '(?m)^\s+adapter_level:\s*(\d+)\s*$')
    if (-not $levelMatch.Success) {
        return
    }
    $level = [int]$levelMatch.Groups[1].Value
    if ($level -lt 2) {
        return
    }

    $runtimeText = [string]::Join("`n", $runtimeLines)
    $nativeMatch = [regex]::Match($runtimeText, '(?ms)^\s{2}native:\s*\r?\n(?<body>(?:^\s{4,}.*\r?\n?)*)')
    if (-not $nativeMatch.Success) {
        $errors.Add("标准运行时缺少 runtime.native 声明: $Image")
        return
    }
    $nativeText = $nativeMatch.Groups['body'].Value
    foreach ($field in @('helper', 'profile', 'exec_target', 'reset_strategy')) {
        if ($nativeText -notmatch "(?m)^\s{4}$([regex]::Escape($field)):\s*\S") {
            $errors.Add("标准运行时 native.$field 缺失: $Image")
        }
    }
    $actionsMatch = [regex]::Match($nativeText, '(?ms)^\s{4}actions:\s*\r?\n(?<body>(?:^\s{6,}.*\r?\n?)*)')
    $methodsMatch = [regex]::Match($nativeText, '(?ms)^\s{4}methods:\s*\r?\n(?<body>(?:^\s{6,}.*\r?\n?)*)')
    $actionsText = if ($actionsMatch.Success) { $actionsMatch.Groups['body'].Value } else { '' }
    $methodsText = if ($methodsMatch.Success) { $methodsMatch.Groups['body'].Value } else { '' }
    foreach ($action in @('deploy', 'tx', 'query')) {
        $hasAction = $actionsText -match "(?m)^\s{6}$([regex]::Escape($action)):\s*\S"
        $hasMethod = $methodsText -match "(?m)^\s{6}$([regex]::Escape($action)):\s*\S"
        if (-not $hasAction -and -not $hasMethod) {
            $errors.Add("标准运行时必须声明 native.actions/${action} 或 native.methods/${action}: $Image")
        }
    }
    $selftestMatch = [regex]::Match($contents, '(?ms)^capability_selftest:\s*\r?\n(?<body>(?:^\s{2,}.*\r?\n?)*)')
    if (-not $selftestMatch.Success -or $selftestMatch.Groups['body'].Value -notmatch '(?m)^\s{2}deploy_payload:\s*\S') {
        $errors.Add("标准运行时必须声明 capability_selftest.deploy_payload: $Image")
    }
    if (-not $selftestMatch.Success -or $selftestMatch.Groups['body'].Value -notmatch '(?m)^\s{2}query_target:\s*\S') {
        $errors.Add("标准运行时必须声明 capability_selftest.query_target: $Image")
    }
}

# Test-StructuredCapabilities 保证 runtime/infra/tool/judger 使用同一套能力字段。
function Test-StructuredCapabilities {
    param(
        [string]$Image,
        [string]$Category,
        [string]$ManifestPath
    )
    if ($Category -notin @("runtime", "infra", "tool", "judger")) {
        return
    }
    $lines = Get-Content -LiteralPath $ManifestPath
    $start = -1
    for ($index = 0; $index -lt $lines.Count; $index++) {
        if ($lines[$index] -match '^capabilities:\s*$') {
            $start = $index
            break
        }
    }
    if ($start -lt 0) {
        $errors.Add("$Category 镜像缺少 capabilities 结构: $Image")
        return
    }
    $body = [System.Collections.Generic.List[string]]::new()
    for ($index = $start + 1; $index -lt $lines.Count; $index++) {
        if ($lines[$index] -match '^[A-Za-z_][A-Za-z0-9_]*:\s*') {
            break
        }
        $body.Add($lines[$index])
    }
    foreach ($key in @('provides', 'requires', 'conflicts', 'cardinality', 'placement', 'config_schema', 'student_access')) {
        if (-not ($body -match "^  ${key}:\s*")) {
            $errors.Add("$Category 镜像 capabilities.$key 缺失: $Image")
        }
    }
}

# Test-StructuredBindings 校验依赖组件只使用角色化 bindings,拒绝旧的 required_bindings。
function Test-StructuredBindings {
    param(
        [string]$Image,
        [string]$Category,
        [string]$ManifestPath
    )
    $lines = Get-Content -LiteralPath $ManifestPath
    $contents = Get-Content -LiteralPath $ManifestPath -Raw
    if ($contents -match '(?m)^\s+required_bindings:\s*$') {
        $errors.Add("$Category 镜像仍使用已删除的 required_bindings 字段: $Image")
    }
    if ($Category -notin @("infra", "tool", "judger")) {
        return
    }

    $capabilities = Get-ChaimirYamlBlock -Path $ManifestPath -BlockName "capabilities"
    $requiresNonEmpty = $false
    $insideRequires = $false
    foreach ($line in $capabilities) {
        if ($line -match '^  requires:\s*$') {
            $insideRequires = $true
            continue
        }
        if ($insideRequires -and $line -match '^  [A-Za-z_][A-Za-z0-9_]*:\s*') {
            break
        }
        if ($insideRequires -and $line -match '^    -\s+\S') {
            $requiresNonEmpty = $true
            break
        }
    }

    $configEnvKeys = @{}
    $insideConfigKeys = $false
    foreach ($line in $lines) {
        if ($line -match '^env_keys:\s*$') {
            $insideConfigKeys = $false
            continue
        }
        if ($line -match '^  config:\s*$') {
            $insideConfigKeys = $true
            continue
        }
        if ($insideConfigKeys -and $line -match '^  [A-Za-z_][A-Za-z0-9_]*:\s*') {
            break
        }
        if ($insideConfigKeys -and $line -match '^    -\s+([A-Za-z_][A-Za-z0-9_]*)\s*$') {
            $configEnvKeys[$Matches[1]] = $true
        }
    }

    $bindingCount = 0
    for ($index = 0; $index -lt $lines.Count; $index++) {
        if ($lines[$index] -notmatch '^  bindings:\s*$') {
            continue
        }
        $items = [System.Collections.Generic.List[string]]::new()
        $current = [System.Collections.Generic.List[string]]::new()
        for ($cursor = $index + 1; $cursor -lt $lines.Count; $cursor++) {
            $line = $lines[$cursor]
            if ($line -match '^[A-Za-z_][A-Za-z0-9_]*:\s*') {
                break
            }
            if ($line -match '^    - name:\s*\S+') {
                if ($current.Count -gt 0) {
                    $items.Add([string]::Join("`n", $current))
                    $current = [System.Collections.Generic.List[string]]::new()
                }
            }
            if ($line.Trim() -ne '') {
                $current.Add($line)
            }
        }
        if ($current.Count -gt 0) {
            $items.Add([string]::Join("`n", $current))
        }
        if ($items.Count -eq 0) {
            $errors.Add("$Category bindings 不能为空: $Image")
            continue
        }
        $bindingCount += $items.Count
        $names = @{}
        foreach ($item in $items) {
            $nameMatch = [regex]::Match($item, '(?m)^\s{4}- name:\s*([A-Za-z][A-Za-z0-9_-]*)\s*$')
            if (-not $nameMatch.Success) {
                $errors.Add("$Category bindings 缺少合法 name: $Image")
                continue
            }
            $name = $nameMatch.Groups[1].Value
            if ($names.ContainsKey($name)) {
                $errors.Add("$Category bindings.name 重复: $Image -> $name")
            }
            $names[$name] = $true
            foreach ($field in @('capability', 'endpoint', 'protocol', 'required_at_start', 'config_binding')) {
                if ($item -notmatch "(?m)^\s{6}$([regex]::Escape($field)):\s*\S") {
                    $errors.Add("$Category binding $name 缺少 ${field}: $Image")
                }
            }
            $protocolMatch = [regex]::Match($item, '(?m)^\s{6}protocol:\s*(\S+)\s*$')
            if ($protocolMatch.Success -and $protocolMatch.Groups[1].Value -notin @('TCP', 'HTTP', 'HTTPS', 'WS', 'WSS', 'GRPC')) {
                $errors.Add("$Category binding $name 的 protocol 非法: $Image -> $($protocolMatch.Groups[1].Value)")
            }
            $configMatch = [regex]::Match($item, '(?m)^\s{6}config_binding:\s*env:([A-Za-z_][A-Za-z0-9_]*)\s*$')
            if (-not $configMatch.Success) {
                $errors.Add("$Category binding $name 的 config_binding 必须是 env:ENV_NAME: $Image")
            } elseif (-not $configEnvKeys.ContainsKey($configMatch.Groups[1].Value)) {
                $errors.Add("$Category binding $name 的环境键未在 manifest 声明: $Image -> $($configMatch.Groups[1].Value)")
            }
        }
    }
    if ($requiresNonEmpty -and $bindingCount -eq 0) {
        $errors.Add("$Category capabilities.requires 非空但没有 bindings: $Image")
    }
}

# Test-DeclaredChecksumFile 校验镜像 manifest 声明的源码/补丁摘要清单未被静默替换。
function Test-DeclaredChecksumFile {
    param(
        [string]$Image,
        [string]$ManifestPath
    )
    $upstream = Get-ChaimirYamlBlock -Path $ManifestPath -BlockName "upstream"
    $checksumFile = Get-ChaimirYamlValue -Lines $upstream -Key "patch_artifact_checksum_file"
    $declaredDigest = Get-ChaimirYamlValue -Lines $upstream -Key "patch_artifact_checksum_file_sha256"
    if ([string]::IsNullOrWhiteSpace($checksumFile) -and [string]::IsNullOrWhiteSpace($declaredDigest)) {
        return
    }
    if ([string]::IsNullOrWhiteSpace($checksumFile) -or $declaredDigest -notmatch '^sha256:[0-9a-f]{64}$') {
        $errors.Add("patch_artifact_checksum_file 与 patch_artifact_checksum_file_sha256 必须同时声明且使用 sha256: 摘要: $Image")
        return
    }
    $manifestDirectory = (Resolve-Path (Split-Path -Parent $ManifestPath)).Path
    $checksumPath = [System.IO.Path]::GetFullPath((Join-Path $manifestDirectory $checksumFile))
    if (-not $checksumPath.StartsWith($manifestDirectory + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
        $errors.Add("patch_artifact_checksum_file 不能越出镜像目录: $Image")
        return
    }
    if (-not (Test-Path -LiteralPath $checksumPath -PathType Leaf)) {
        $errors.Add("patch_artifact_checksum_file 不存在: $Image -> $checksumFile")
        return
    }
    $actualDigest = (Get-FileHash -LiteralPath $checksumPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ("sha256:$actualDigest" -ne $declaredDigest) {
        $errors.Add("patch_artifact_checksum_file 摘要不匹配: $Image")
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

    if ($category -eq "runtime") {
        Test-RuntimeCapabilityDeclaration -Image $entry.Image -ManifestPath $entry.Manifest
    }
    Test-DeclaredChecksumFile -Image $entry.Image -ManifestPath $entry.Manifest
    Test-StructuredCapabilities -Image $entry.Image -Category $category -ManifestPath $entry.Manifest
    Test-StructuredBindings -Image $entry.Image -Category $category -ManifestPath $entry.Manifest

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
$attestationLine = Get-Content -LiteralPath $DeployConfigPath | Where-Object { $_ -match "^PLATFORM_IMAGE_ATTESTATIONS_JSON=" }
if (@($attestationLine).Count -ne 1) {
    $errors.Add("$DeployConfigPath 必须且只能声明一次 PLATFORM_IMAGE_ATTESTATIONS_JSON")
} else {
    $attestationJSON = $attestationLine.Substring($attestationLine.IndexOf("=") + 1)
    $seenAttestations = @{}
    $parsedAttestations = ConvertFrom-Json -InputObject $attestationJSON
    foreach ($item in $parsedAttestations) {
        $imageURL = [string]$item.image_url
        if ($imageURL -notmatch "^.+/([^/]+/[^/@]+)@(sha256:[0-9a-f]{64})$") {
            $errors.Add("PLATFORM_IMAGE_ATTESTATIONS_JSON 包含非法镜像引用")
            continue
        }
        $logical = $Matches[1]
        $digest = $Matches[2]
        if ($seenAttestations.ContainsKey($logical)) {
            $errors.Add("PLATFORM_IMAGE_ATTESTATIONS_JSON 重复登记: $logical")
            continue
        }
        $seenAttestations[$logical] = $true
        if (-not ($lock.ContainsKey($logical)) -or $lock[$logical] -ne $digest) {
            $errors.Add("PLATFORM_IMAGE_ATTESTATIONS_JSON 与正式 digest lock 不一致: $logical")
        }
        if (-not ([bool]$item.cosign_verified) -or -not [string]::Equals(([string]$item.trivy_status), "passed", [System.StringComparison]::OrdinalIgnoreCase)) {
            $errors.Add("PLATFORM_IMAGE_ATTESTATIONS_JSON 包含未通过门禁的条目: $logical")
        }
    }
}

if ($errors.Count -gt 0) {
    Write-Error ("镜像元数据校验失败:`n" + ($errors -join "`n"))
    exit 1
}

Write-Output "Validated $($catalog.Count) manifests and $($lock.Count) immutable digest entries."
