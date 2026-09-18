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

# Invoke-ChaimirDocker 以明确超时执行 Docker CLI,并保留可定位的标准输出与错误。
function Invoke-ChaimirDocker {
    param(
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [int]$TimeoutSeconds = 300
    )
    if ($TimeoutSeconds -lt 1) { throw "Docker CLI 超时必须大于等于 1 秒" }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = [Diagnostics.ProcessStartInfo]::new()
    $process.StartInfo.FileName = "docker.exe"
    $process.StartInfo.Arguments = (($Arguments | ForEach-Object { ConvertTo-ChaimirProcessArgument ([string]$_) }) -join " ")
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.CreateNoWindow = $true
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    $process.StartInfo.StandardOutputEncoding = [Text.Encoding]::UTF8
    $process.StartInfo.StandardErrorEncoding = [Text.Encoding]::UTF8
    try {
        if (-not $process.Start()) { throw "Docker CLI 启动失败" }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
            try {
                & taskkill.exe /PID $process.Id /T /F | Out-Null
            } catch {
                Write-Warning ("Docker CLI 超时后的进程树终止失败: {0}" -f $_.Exception.Message)
            }
            try {
                $process.Kill($true)
            } catch {
                Write-Warning ("Docker CLI 超时后的进程终止失败: {0}" -f $_.Exception.Message)
            }
            $stdout = $stdoutTask.GetAwaiter().GetResult()
            $stderr = $stderrTask.GetAwaiter().GetResult()
            return [pscustomobject]@{
                Code = 124
                Text = ((@(("Docker CLI 超时（{0} 秒）" -f $TimeoutSeconds), $stdout, $stderr) | Where-Object { $_ }) -join "`n").Trim()
            }
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        return [pscustomobject]@{
            Code = $process.ExitCode
            Text = ((@($stdout, $stderr) | Where-Object { $_ }) -join "`n").Trim()
        }
    } finally {
        $process.Dispose()
    }
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
        PG_CLIENT_IMAGE    = "middleware/postgres"
        CILIUM_RUNTIME_IMAGE = "network/cilium-runtime"
        CILIUM_BUILDER_IMAGE = "network/cilium-builder"
        CILIUM_ENVOY_IMAGE   = "network/cilium-envoy"
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

Export-ModuleMember -Function Get-ChaimirYamlValue, Get-ChaimirTopLevelYamlValue, Get-ChaimirYamlBlock, Get-ChaimirImageSourceType, Get-ChaimirImageCatalog, Read-ChaimirEnvFile, Get-ChaimirCanonicalRegistry, Invoke-ChaimirDocker, Get-ChaimirInternalImageBuildArguments, Resolve-ChaimirImageBuildPaths, Read-ChaimirDigestLock, Write-ChaimirDigestLock
