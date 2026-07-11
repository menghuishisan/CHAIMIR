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

Export-ModuleMember -Function Get-ChaimirYamlValue, Get-ChaimirTopLevelYamlValue, Get-ChaimirYamlBlock, Get-ChaimirImageSourceType, Read-ChaimirDigestLock, Write-ChaimirDigestLock
