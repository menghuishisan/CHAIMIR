# ImageMetadata 提供镜像 manifest 与 digest 锁的共享读取、校验和写入能力。
Set-StrictMode -Version Latest

# Get-ChaimirYamlValue 从简单 YAML 行集合读取指定键值。
function Get-ChaimirYamlValue {
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

# Get-ChaimirTopLevelYamlValue 只读取 YAML 顶层键值。
function Get-ChaimirTopLevelYamlValue {
    param(
        [string[]]$Lines,
        [string]$Key
    )
    foreach ($line in $Lines) {
        if ($line -match "^$([regex]::Escape($Key)):\s*(.+?)\s*$") {
            return $Matches[1].Trim().Trim('"').Trim("'")
        }
    }
    return $null
}

# Get-ChaimirYamlBlock 读取指定 YAML 顶层块的原始行。
function Get-ChaimirYamlBlock {
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

# Get-ChaimirImageSourceType 读取 manifest 的 source.type。
function Get-ChaimirImageSourceType {
    param([string]$Path)
    $source = Get-ChaimirYamlBlock -Path $Path -BlockName "source"
    return Get-ChaimirYamlValue -Lines $source -Key "type"
}

# Get-ChaimirImageCatalog 读取全部 manifest,统一返回逻辑名、来源类型与部署准入状态。
function Get-ChaimirImageCatalog {
    param([string]$ImagesRoot)
    if (-not (Test-Path -LiteralPath $ImagesRoot)) {
        throw "镜像目录不存在: $ImagesRoot"
    }
    $catalog = @{}
    foreach ($manifest in Get-ChildItem -LiteralPath $ImagesRoot -Recurse -File -Filter "manifest.yaml") {
        $lines = Get-Content -LiteralPath $manifest.FullName
        $image = Get-ChaimirTopLevelYamlValue -Lines $lines -Key "image"
        if ([string]::IsNullOrWhiteSpace($image)) {
            throw "manifest 缺少顶层 image: $($manifest.FullName)"
        }
        if ($catalog.ContainsKey($image)) {
            throw "逻辑镜像名重复: $image"
        }
        $sourceType = Get-ChaimirImageSourceType -Path $manifest.FullName
        if ($sourceType -notin @("platform-built", "thin-wrapper", "build-base", "upstream-pinned")) {
            throw "$($manifest.FullName): source.type 非法或缺失"
        }
        $supplyChain = Get-ChaimirYamlBlock -Path $manifest.FullName -BlockName "supply_chain"
        $deployableValue = Get-ChaimirYamlValue -Lines $supplyChain -Key "deployable"
        $deployable = $deployableValue -ne "false"
        $blockReason = Get-ChaimirYamlValue -Lines $supplyChain -Key "block_reason"
        if (-not $deployable -and [string]::IsNullOrWhiteSpace($blockReason)) {
            throw "$($manifest.FullName): supply_chain.deployable=false 必须声明 block_reason"
        }
        $catalog[$image] = [pscustomobject]@{
            Image       = $image
            Manifest    = $manifest.FullName
            SourceType  = $sourceType
            Deployable  = $deployable
            BlockReason = $blockReason
        }
    }
    return $catalog
}

# Read-ChaimirEnvFile 严格读取项目 KEY=VALUE 配置,拒绝非法行、重复键和必填空值。
function Read-ChaimirEnvFile {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [string[]]$RequiredKeys = @()
    )
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "缺少项目配置文件: $Path"
    }
    $values = @{}
    $lineNumber = 0
    foreach ($line in Get-Content -LiteralPath $Path) {
        $lineNumber++
        $trimmed = ([string]$line).Trim()
        if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) {
            continue
        }
        if ($trimmed -notmatch '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') {
            throw "项目配置包含非法行: ${Path}:$lineNumber"
        }
        $name = $Matches[1]
        $value = $Matches[2]
        if ($values.ContainsKey($name)) {
            throw ("项目配置包含重复键: {0}:{1}:{2}" -f $Path, $lineNumber, $name)
        }
        $values[$name] = $value
    }
    foreach ($name in $RequiredKeys) {
        if (-not $values.ContainsKey($name) -or [string]::IsNullOrWhiteSpace([string]$values[$name])) {
            throw "项目配置缺少必填值: $name"
        }
    }
    return $values
}

# Get-ChaimirCanonicalRegistry 校验镜像、供应链端点和 Harbor 外部 URL 为同一 HTTPS 入口。
function Get-ChaimirCanonicalRegistry {
    param([Parameter(Mandatory = $true)][hashtable]$Config)

    foreach ($name in @("SUPPLY_CHAIN_REGISTRY", "SUPPLY_CHAIN_REGISTRY_ENDPOINT", "SUPPLY_CHAIN_HARBOR_EXTERNAL_URL", "IMAGE_REGISTRY")) {
        if (-not $Config.ContainsKey($name) -or [string]::IsNullOrWhiteSpace([string]$Config[$name])) {
            throw "统一配置缺少 $name"
        }
    }
    $registry = ([string]$Config["SUPPLY_CHAIN_REGISTRY"]).TrimEnd("/").ToLowerInvariant()
    $endpoint = ([string]$Config["SUPPLY_CHAIN_REGISTRY_ENDPOINT"]).TrimEnd("/").ToLowerInvariant()
    $imageRegistry = ([string]$Config["IMAGE_REGISTRY"]).TrimEnd("/").ToLowerInvariant()
    if ($registry -notmatch '^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$' -or
        $endpoint -notmatch '^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$' -or
        $imageRegistry -notmatch '^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$') {
        throw "统一 Registry 必须是无协议、无端口的 DNS 主机名"
    }
    $externalUrl = [Uri]([string]$Config["SUPPLY_CHAIN_HARBOR_EXTERNAL_URL"])
    if ($externalUrl.Scheme -ne "https" -or -not $externalUrl.IsDefaultPort -or
        $externalUrl.AbsolutePath -ne "/" -or $externalUrl.Query.Length -ne 0 -or $externalUrl.Fragment.Length -ne 0) {
        throw "SUPPLY_CHAIN_HARBOR_EXTERNAL_URL 必须是标准 HTTPS 443 根地址"
    }
    $externalHost = $externalUrl.Host.TrimEnd(".").ToLowerInvariant()
    if ($registry -ne $endpoint -or $registry -ne $imageRegistry -or $registry -ne $externalHost) {
        throw "镜像 Registry、供应链端点、Harbor 外部 URL 必须统一为同一主机"
    }
    if ($Config.ContainsKey("SUPPLY_CHAIN_REGISTRY_HOST_ALIAS") -and
        ([string]$Config["SUPPLY_CHAIN_REGISTRY_HOST_ALIAS"]).TrimEnd("/").ToLowerInvariant() -ne $registry) {
        throw "SUPPLY_CHAIN_REGISTRY_HOST_ALIAS 必须与统一 Registry 一致"
    }
    return $registry
}

# ConvertTo-ChaimirProcessArgument 将参数安全编码为 Windows 原生进程参数。
function ConvertTo-ChaimirProcessArgument {
    param([AllowNull()][string]$Value)
    if ($null -eq $Value -or $Value.Length -eq 0) { return '""' }
    if ($Value -notmatch '[\s"]') { return $Value }
    $builder = [Text.StringBuilder]::new()
    [void]$builder.Append('"')
    $slashes = 0
    foreach ($character in $Value.ToCharArray()) {
        if ($character -eq '\') { $slashes++; continue }
        if ($character -eq '"') {
            if ($slashes -gt 0) { [void]$builder.Append(('\' * ($slashes * 2)) -join '') }
            [void]$builder.Append('\').Append('"')
            $slashes = 0
            continue
        }
        if ($slashes -gt 0) { [void]$builder.Append(('\' * $slashes) -join '') }
        [void]$builder.Append($character)
        $slashes = 0
    }
    if ($slashes -gt 0) { [void]$builder.Append(('\' * ($slashes * 2)) -join '') }
    [void]$builder.Append('"')
    return $builder.ToString()
}

# Invoke-ChaimirProcess 按退出码执行原生工具,区分正常stderr和失败,并限制等待时间。
function Invoke-ChaimirProcess {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][AllowEmptyString()][string[]]$Arguments,
        [int]$TimeoutSeconds = 300,
        [string]$WorkingDirectory,
        [Text.Encoding]$OutputEncoding = [Text.Encoding]::UTF8
    )
    if ($TimeoutSeconds -lt 1 -or $TimeoutSeconds -gt 2147483) { throw "原生工具超时必须为1..2147483秒" }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = [Diagnostics.ProcessStartInfo]::new()
    $process.StartInfo.FileName = $FilePath
    if ($WorkingDirectory) { $process.StartInfo.WorkingDirectory = $WorkingDirectory }
    $process.StartInfo.Arguments = (($Arguments | ForEach-Object { ConvertTo-ChaimirProcessArgument ([string]$_) }) -join " ")
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.CreateNoWindow = $true
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    $process.StartInfo.StandardOutputEncoding = $OutputEncoding
    $process.StartInfo.StandardErrorEncoding = $OutputEncoding
    try {
        if (-not $process.Start()) { throw "原生工具启动失败: $FilePath" }
        $clock = [Diagnostics.Stopwatch]::StartNew()
        $budget = $TimeoutSeconds * 1000
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        $streams = [Threading.Tasks.Task[]]@($stdoutTask, $stderrTask)
        $processExited = $process.WaitForExit($budget)
        # 子进程可能继承输出管道；父进程退出后仍须遵守原期限。
        $remaining = [int][Math]::Max(0, $budget - $clock.ElapsedMilliseconds)
        $timedOut = -not $processExited -or -not [Threading.Tasks.Task]::WaitAll($streams, $remaining)
        if ($timedOut -and -not $process.HasExited) {
            try {
                $termination = [Diagnostics.ProcessStartInfo]::new()
                $termination.FileName = 'taskkill.exe'
                $termination.Arguments = "/PID $($process.Id) /T /F"
                $termination.UseShellExecute = $false
                $termination.CreateNoWindow = $true
                $terminator = [Diagnostics.Process]::Start($termination)
                try {
                    if (-not $terminator.WaitForExit(5000)) { throw 'taskkill未在5秒内结束' }
                    if ($terminator.ExitCode -ne 0 -and -not $process.HasExited) { throw "taskkill退出码$($terminator.ExitCode)" }
                } finally { $terminator.Dispose() }
            } catch {
                Write-Warning ("原生工具超时后的进程树终止失败: {0}" -f $_.Exception.Message)
            }
            if (-not $process.HasExited) { $process.Kill() }
            if (-not $process.WaitForExit(5000)) { throw "超时进程未能终止: $FilePath" }
            [void][Threading.Tasks.Task]::WaitAll($streams, 5000)
        }
        $stdout = if ($stdoutTask.IsCompleted) { $stdoutTask.GetAwaiter().GetResult() } else { '' }
        $stderr = if ($stderrTask.IsCompleted) { $stderrTask.GetAwaiter().GetResult() } else { '' }
        $timeoutMessage = if ($timedOut) { "原生工具超时（${TimeoutSeconds} 秒）: $FilePath" } else { '' }
        return [pscustomobject]@{
            Code = if ($timedOut) { 124 } else { $process.ExitCode }
            Stdout = $stdout
            Stderr = $stderr
            TimedOut = $timedOut
            Text = ((@($timeoutMessage, $stdout, $stderr) | Where-Object { $_ }) -join [Environment]::NewLine).Trim()
        }
    } finally {
        $process.Dispose()
    }
}

# Invoke-ChaimirDocker 使用共享进程边界执行Docker,保持镜像脚本的统一结果契约。
function Invoke-ChaimirDocker {
    param(
        [Parameter(Mandatory = $true)][AllowEmptyString()][string[]]$Arguments,
        [int]$TimeoutSeconds = 300
    )
    return Invoke-ChaimirProcess -FilePath 'docker.exe' -Arguments $Arguments -TimeoutSeconds $TimeoutSeconds
}

# Get-ChaimirInternalImageBuildArguments 返回内部构建镜像参数到逻辑镜像名的唯一映射。
function Get-ChaimirInternalImageBuildArguments {
    return [ordered]@{
        GO_BUILDER_IMAGE   = "base/go-builder"
        NODE_BUILDER_IMAGE = "base/node-builder"
        JUDGE_MIN_IMAGE    = "base/judge-min"
        FABRIC_TOOLS_IMAGE = "base/fabric-tools"
        RUNTIME_IMAGE      = "base/runtime-min"
        EVM_HARDHAT_IMAGE  = "runtime/evm-hardhat"
        SLITHER_IMAGE      = "tool/slither"
        PG_CLIENT_IMAGE      = "infra/postgres"
        CILIUM_RUNTIME_IMAGE = "infra/cilium-runtime"
        CILIUM_BUILDER_IMAGE = "infra/cilium-builder"
        CILIUM_ENVOY_IMAGE   = "infra/cilium-envoy"
    }
}

# Get-ChaimirImageBuildSources 只解析 Dockerfile 声明的镜像参数；内部依赖以逻辑名称返回。
function Get-ChaimirImageBuildSources {
    param([string]$ManifestPath, [string]$DockerfileText)
    $internal = Get-ChaimirInternalImageBuildArguments
    $upstream = Get-ChaimirYamlBlock -Path $ManifestPath -BlockName 'upstream'
    $externalArg = Get-ChaimirYamlValue -Lines $upstream -Key 'build_base_arg'
    $sources = [ordered]@{}
    foreach ($declaration in [regex]::Matches($DockerfileText, '(?im)^[ \t]*ARG[ \t]+([A-Z0-9_]+_IMAGE)\b')) {
        $argument = $declaration.Groups[1].Value
        if ($internal.Contains($argument)) {
            $sources[$argument] = $internal[$argument]
            continue
        }
        if ($argument -eq 'NGINX_RUNTIME_IMAGE') {
            $imageKey = 'runtime'
            $digestKey = 'runtime_digest'
        } elseif ($argument -eq $externalArg) {
            $imageKey = 'build_base_image'
            $digestKey = 'build_base_digest'
        } else {
            throw "${ManifestPath}: 未登记的构建镜像参数 $argument"
        }
        $image = Get-ChaimirYamlValue -Lines $upstream -Key $imageKey
        $digest = Get-ChaimirYamlValue -Lines $upstream -Key $digestKey
        if ($image -notmatch '^[^@\s]+$' -or $digest -cnotmatch '^sha256:[0-9a-f]{64}$') {
            throw "${ManifestPath}: upstream.$imageKey 和 upstream.$digestKey 必须组成不可变引用"
        }
        $sources[$argument] = "$image@$digest"
    }
    if ($externalArg -and ($internal.Contains($externalArg) -or -not $sources.Contains($externalArg))) {
        throw "${ManifestPath}: upstream.build_base_arg 必须指向已声明的外部镜像参数"
    }
    return $sources
}

function Resolve-ChaimirImageBuildArguments {
    param([System.Collections.IDictionary]$Sources, [string]$Registry, [hashtable]$DigestLock)
    $arguments = [ordered]@{}
    foreach ($argument in $Sources.Keys) {
        $source = $Sources[$argument]
        if ($source.Contains('@')) {
            $arguments[$argument] = $source
            continue
        }
        $digest = $DigestLock[$source]
        if ($digest -cnotmatch '^sha256:[0-9a-f]{64}$') {
            throw "digest 锁缺少合法的 $source，无法注入 $argument"
        }
        $arguments[$argument] = "$Registry/$source@$digest"
    }
    return $arguments
}

function Get-ChaimirFrontendBuildArguments {
    param([string]$Path, [string]$DockerfileText)
    $names = @([regex]::Matches($DockerfileText, '(?m)^ARG (VITE_[A-Z0-9_]+)\s*$') | ForEach-Object { $_.Groups[1].Value })
    if (-not $names.Count) { throw '前端 Dockerfile 没有声明构建配置参数' }
    $required = @($names | Where-Object { $_ -ne 'VITE_WS_BASE_URL' })
    $values = Read-ChaimirEnvFile -Path $Path -RequiredKeys $required
    $arguments = [ordered]@{}
    foreach ($name in $names) { $arguments[$name] = [string]$values[$name] }
    return $arguments
}

# Get-ChaimirImageBuildWaves 为选中的镜像分批。未选中的依赖从 digest 锁消费。
function Get-ChaimirImageBuildWaves {
    param([hashtable]$Dependencies)
    $remaining = @{}
    foreach ($image in $Dependencies.Keys) { $remaining[$image] = $true }
    while ($remaining.Count) {
        $ready = @($remaining.Keys | Sort-Object | Where-Object {
            -not ($Dependencies[$_] | Where-Object { $remaining.ContainsKey($_) })
        })
        if (-not $ready.Count) {
            throw "镜像内部构建依赖存在循环: $(($remaining.Keys | Sort-Object) -join ', ')"
        }
        [pscustomobject]@{ Images = $ready }
        foreach ($image in $ready) { $remaining.Remove($image) }
    }
}

# Resolve-ChaimirImageBuildPaths 按 manifest 规则统一解析构建上下文与 Dockerfile 绝对路径。
function Resolve-ChaimirImageBuildPaths {
    param(
        [string]$RepoRoot,
        [string]$ManifestPath,
        [string]$ContextValue,
        [string]$DockerfileValue
    )
    $manifestDir = Split-Path -Parent $ManifestPath
    if ([string]::IsNullOrWhiteSpace($ContextValue)) {
        $ContextValue = "."
    }
    if ([string]::IsNullOrWhiteSpace($DockerfileValue)) {
        $DockerfileValue = "Dockerfile"
    }
    $contextPath = if ($ContextValue -match "^images[\\/]") {
        [System.IO.Path]::GetFullPath((Join-Path $RepoRoot $ContextValue))
    } else {
        [System.IO.Path]::GetFullPath((Join-Path $manifestDir $ContextValue))
    }
    $dockerfilePath = if ($DockerfileValue -match "^images[\\/]") {
        [System.IO.Path]::GetFullPath((Join-Path $RepoRoot $DockerfileValue))
    } else {
        [System.IO.Path]::GetFullPath((Join-Path $contextPath $DockerfileValue))
    }
    return [pscustomobject]@{
        Context    = $contextPath
        Dockerfile = $dockerfilePath
    }
}

# Assert-ChaimirImageSelection enforces the single image-selection contract for build, pull, and admission scripts.
function Assert-ChaimirImageSelection {
    param([string[]]$Images)
    foreach ($image in $Images) {
        if ([string]::IsNullOrWhiteSpace($image)) {
            throw "-Images 不能包含空值"
        }
        if ($image.Contains(',')) {
            throw "-Images 必须传入 PowerShell 数组元素,不能把多个镜像拼成逗号字符串: $image"
        }
    }
}
function Read-ChaimirDigestFragments {
    param([string]$Directory)
    $files = @(Get-ChildItem -LiteralPath $Directory -Filter '*.lock' -File)
    if (-not $files.Count) { throw "digest 目录没有锁片段: $Directory" }
    $items = @{}
    foreach ($file in $files) {
        $fragment = Read-ChaimirDigestLock -Path $file.FullName -Required
        if ($fragment.Count -ne 1) { throw "每个 digest 片段须且只能声明一个镜像: $($file.Name)" }
        foreach ($image in $fragment.Keys) {
            if ($items.ContainsKey($image)) { throw "digest 锁片段重复声明 $image" }
            $items[$image] = $fragment[$image]
        }
    }
    return $items
}

# Read-ChaimirDigestLock 读取并严格校验 logical-image sha256:digest 锁。
function Read-ChaimirDigestLock {
    param(
        [string]$Path,
        [switch]$Required
    )
    $items = @{}
    if (-not (Test-Path -LiteralPath $Path)) {
        if ($Required) {
            throw "缺少 digest 锁: $Path"
        }
        return $items
    }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq "" -or $trimmed.StartsWith("#")) {
            continue
        }
        if ($trimmed -notmatch "^([^:\s]+/[^:\s]+)\s*[:= ]\s*(sha256:[0-9a-f]{64})$") {
            throw "digest 锁格式非法: $Path -> $line"
        }
        $image = $Matches[1]
        $digest = $Matches[2]
        if ($items.ContainsKey($image) -and $items[$image] -ne $digest) {
            throw "digest 锁中镜像 $image 存在冲突值"
        }
        $items[$image] = $digest
    }
    return $items
}

# Write-ChaimirDigestLock 按镜像逻辑名排序写入无 BOM、LF 结尾的 ASCII 锁文件。
function Write-ChaimirDigestLock {
    param(
        [string]$Path,
        [hashtable]$Items
    )
    $parent = Split-Path -Parent $Path
    if (-not [string]::IsNullOrWhiteSpace($parent) -and -not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent | Out-Null
    }
    $lines = [System.Collections.Generic.List[string]]::new()
    foreach ($key in ($Items.Keys | Sort-Object)) {
        $digest = $Items[$key]
        if ($key -notmatch "^[^:\s]+/[^:\s]+$" -or $digest -notmatch "^sha256:[0-9a-f]{64}$") {
            throw "拒绝写入非法 digest 锁项: $key $digest"
        }
        $lines.Add("$key $digest")
    }
    $contents = [string]::Join("`n", $lines) + "`n"
    [System.IO.File]::WriteAllText($Path, $contents, [System.Text.ASCIIEncoding]::new())
}

Export-ModuleMember -Function Get-ChaimirYamlValue, Get-ChaimirTopLevelYamlValue, Get-ChaimirYamlBlock, Get-ChaimirImageSourceType, Get-ChaimirImageCatalog, Read-ChaimirEnvFile, Get-ChaimirCanonicalRegistry, Invoke-ChaimirProcess, Invoke-ChaimirDocker, Get-ChaimirImageBuildSources, Resolve-ChaimirImageBuildArguments, Get-ChaimirFrontendBuildArguments, Get-ChaimirImageBuildWaves, Resolve-ChaimirImageBuildPaths, Assert-ChaimirImageSelection, Read-ChaimirDigestFragments, Read-ChaimirDigestLock, Write-ChaimirDigestLock
