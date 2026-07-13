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

# Get-ChaimirInternalImageBuildArguments 返回内部构建镜像参数到逻辑镜像名的唯一映射。
function Get-ChaimirInternalImageBuildArguments {
    return [ordered]@{
        GO_BUILDER_IMAGE   = "base/go-builder"
        CHAIN_TOOLS_IMAGE  = "base/chain-tools"
        NODE_BUILDER_IMAGE = "base/node-builder"
        JUDGE_MIN_IMAGE    = "base/judge-min"
        FABRIC_TOOLS_IMAGE = "base/fabric-tools"
        RUNTIME_IMAGE      = "base/runtime-min"
        PG_CLIENT_IMAGE    = "middleware/postgres"
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

Export-ModuleMember -Function Get-ChaimirYamlValue, Get-ChaimirTopLevelYamlValue, Get-ChaimirYamlBlock, Get-ChaimirImageSourceType, Get-ChaimirImageCatalog, Get-ChaimirInternalImageBuildArguments, Resolve-ChaimirImageBuildPaths, Read-ChaimirDigestLock, Write-ChaimirDigestLock
