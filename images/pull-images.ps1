# 本脚本按 images manifest 中的不可变 digest 拉取上游固定镜像。
param(
    [string]$Root = (Split-Path -Parent $MyInvocation.MyCommand.Path)
)

$ErrorActionPreference = "Stop"

function Read-YamlValue {
    param(
        [string[]]$Lines,
        [string]$Key
    )
    foreach ($line in $Lines) {
        if ($line -match "^\s*$([regex]::Escape($Key)):\s*(.+?)\s*$") {
            return $Matches[1].Trim().Trim('"')
        }
    }
    return $null
}

function Read-UpstreamBlock {
    param([string]$Path)
    $lines = Get-Content $Path
    $block = New-Object System.Collections.Generic.List[string]
    $inside = $false
    foreach ($line in $lines) {
        if ($line -match "^upstream:\s*$") {
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

function Read-Components {
    param([string[]]$Upstream)
    $components = @()
    $current = $null
    foreach ($line in $Upstream) {
        if ($line -match "^\s*-\s+image:\s*(.+?)\s*$") {
            if ($current) { $components += $current }
            $current = [ordered]@{ image = $Matches[1].Trim().Trim('"') }
            continue
        }
        if ($current -and $line -match "^\s+version:\s*(.+?)\s*$") {
            $current.version = $Matches[1].Trim().Trim('"')
            continue
        }
        if ($current -and $line -match "^\s+digest:\s*(sha256:[0-9a-f]{64})\s*$") {
            $current.digest = $Matches[1]
        }
    }
    if ($current) { $components += $current }
    return $components
}

$manifests = Get-ChildItem -Path $Root -Recurse -Filter manifest.yaml
$images = New-Object System.Collections.Generic.List[string]
$missing = New-Object System.Collections.Generic.List[string]

foreach ($manifest in $manifests) {
    $content = Get-Content -Raw $manifest.FullName
    if ($content -notmatch "source:\s*\r?\n\s*type:\s*upstream-pinned") {
        continue
    }
    $upstream = Read-UpstreamBlock -Path $manifest.FullName
    $registry = Read-YamlValue -Lines $upstream -Key "registry"
    $image = Read-YamlValue -Lines $upstream -Key "image"
    $digest = Read-YamlValue -Lines $upstream -Key "digest"
    $components = Read-Components -Upstream $upstream

    if ($components.Count -gt 0) {
        foreach ($component in $components) {
            if (-not $component.digest) {
                $missing.Add("$($manifest.FullName): component $($component.image):$($component.version) missing digest")
                continue
            }
            $images.Add("docker.io/$($component.image)@$($component.digest)")
        }
        continue
    }

    if (-not $digest) {
        $missing.Add("$($manifest.FullName): upstream digest missing")
        continue
    }
    $images.Add("$registry/$image@$digest")
}

if ($missing.Count -gt 0) {
    Write-Error ("Refusing to pull images because immutable digests are missing:`n" + ($missing -join "`n"))
    exit 1
}

foreach ($imageRef in $images) {
    Write-Host "Pulling $imageRef"
    docker pull $imageRef
}
