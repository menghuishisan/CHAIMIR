# 本脚本按 manifest 与构建产物 digest 锁顺序拉取 Chaimir 镜像。
param(
    [string]$Root = (Split-Path -Parent $MyInvocation.MyCommand.Path),
    [ValidateSet("all", "upstream-pinned", "built")]
    [string]$Scope = "all",
    [string]$Registry = $env:CHAIMIR_IMAGE_REGISTRY,
    [string]$DigestLock = "",
    [int]$MaxAttempts = 3,
    [int]$RetryDelaySeconds = 5,
    [switch]$FailFast,
    [switch]$NoCleanupFailedPull
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Registry)) {
    $Registry = "harbor.chaimir.local"
}
if ([string]::IsNullOrWhiteSpace($DigestLock)) {
    $DigestLock = Join-Path $Root "image-digests.lock"
}
if ($MaxAttempts -lt 1) {
    throw "MaxAttempts 必须大于等于 1"
}
if ($RetryDelaySeconds -lt 0) {
    throw "RetryDelaySeconds 不能小于 0"
}

function Read-YamlValue {
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

function Read-TopLevelYamlValue {
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

function Read-YamlBlock {
    param(
        [string]$Path,
        [string]$BlockName
    )
    $lines = Get-Content $Path
    $block = New-Object System.Collections.Generic.List[string]
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

function Read-SourceType {
    param([string]$Path)
    $source = Read-YamlBlock -Path $Path -BlockName "source"
    return Read-YamlValue -Lines $source -Key "type"
}

function Read-Components {
    param([string[]]$Upstream)
    $components = @()
    $current = $null
    foreach ($line in $Upstream) {
        if ($line -match "^\s*-\s+image:\s*(.+?)\s*$") {
            if ($current) {
                $components += $current
            }
            $current = [ordered]@{ image = $Matches[1].Trim().Trim('"').Trim("'") }
            continue
        }
        if ($current -and $line -match "^\s+registry:\s*(.+?)\s*$") {
            $current.registry = $Matches[1].Trim().Trim('"').Trim("'")
            continue
        }
        if ($current -and $line -match "^\s+version:\s*(.+?)\s*$") {
            $current.version = $Matches[1].Trim().Trim('"').Trim("'")
            continue
        }
        if ($current -and $line -match "^\s+digest:\s*(sha256:[0-9a-f]{64})\s*$") {
            $current.digest = $Matches[1]
        }
    }
    if ($current) {
        $components += $current
    }
    return $components
}

function Read-DigestLock {
    param([string]$Path)
    $items = @{}
    if (-not (Test-Path -LiteralPath $Path)) {
        return $items
    }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if ($trimmed -eq "" -or $trimmed.StartsWith("#")) {
            continue
        }
        if ($trimmed -match "^([^:\s]+/[^:\s]+)\s*[:= ]\s*(sha256:[0-9a-f]{64})$") {
            $items[$Matches[1]] = $Matches[2]
            continue
        }
        throw "digest 锁格式非法: $Path -> $line"
    }
    return $items
}

function Add-ImageRef {
    param(
        [System.Collections.Generic.List[object]]$Items,
        [hashtable]$Seen,
        [string]$Ref,
        [string]$Kind,
        [string]$Manifest
    )
    if ($Seen.ContainsKey($Ref)) {
        return
    }
    $Seen[$Ref] = $true
    $Items.Add([pscustomobject]@{ Ref = $Ref; Kind = $Kind; Manifest = $Manifest })
}

function Add-UpstreamRefs {
    param(
        [string]$ManifestPath,
        [System.Collections.Generic.List[object]]$Items,
        [hashtable]$Seen,
        [System.Collections.Generic.List[string]]$Missing
    )
    $upstream = Read-YamlBlock -Path $ManifestPath -BlockName "upstream"
    $registry = Read-YamlValue -Lines $upstream -Key "registry"
    $image = Read-YamlValue -Lines $upstream -Key "image"
    $digest = Read-YamlValue -Lines $upstream -Key "digest"
    $components = Read-Components -Upstream $upstream

    if ($components.Count -gt 0) {
        foreach ($component in $components) {
            if (-not $component.digest) {
                $Missing.Add(("{0}: component {1}:{2} 缺少 digest" -f $ManifestPath, $component.image, $component.version))
                continue
            }
            $componentRegistry = $component.registry
            if ([string]::IsNullOrWhiteSpace($componentRegistry)) {
                $componentRegistry = $registry
            }
            if ([string]::IsNullOrWhiteSpace($componentRegistry)) {
                $componentRegistry = "docker.io"
            }
            Add-ImageRef -Items $Items -Seen $Seen -Ref "$componentRegistry/$($component.image)@$($component.digest)" -Kind "upstream-pinned" -Manifest $ManifestPath
        }
        return
    }

    if ([string]::IsNullOrWhiteSpace($registry) -or [string]::IsNullOrWhiteSpace($image) -or [string]::IsNullOrWhiteSpace($digest)) {
        $Missing.Add(("{0}: upstream registry/image/digest 缺失" -f $ManifestPath))
        return
    }
    Add-ImageRef -Items $Items -Seen $Seen -Ref "$registry/$image@$digest" -Kind "upstream-pinned" -Manifest $ManifestPath
}

function Add-BuiltRef {
    param(
        [string]$ManifestPath,
        [hashtable]$DigestLockItems,
        [System.Collections.Generic.List[object]]$Items,
        [hashtable]$Seen,
        [System.Collections.Generic.List[string]]$Missing
    )
    $lines = Get-Content $ManifestPath
    $image = Read-TopLevelYamlValue -Lines $lines -Key "image"
    if ([string]::IsNullOrWhiteSpace($image)) {
        $Missing.Add(("{0}: image 缺失" -f $ManifestPath))
        return
    }
    $digest = $DigestLockItems[$image]
    if ([string]::IsNullOrWhiteSpace($digest)) {
        $Missing.Add(("{0}: 缺少构建产物 digest,请在 image-digests.lock 中声明 {1} 的不可变 digest" -f $ManifestPath, $image))
        return
    }
    Add-ImageRef -Items $Items -Seen $Seen -Ref "$Registry/$image@$digest" -Kind "built" -Manifest $ManifestPath
}

function Invoke-DockerPullWithRetry {
    param([object]$Item)
    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        Write-Host "Pulling [$attempt/$MaxAttempts] $($Item.Ref)"
        $hadImageBeforePull = Test-LocalImageRef -Ref $Item.Ref
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        try {
            $pullOutput = & docker pull $Item.Ref 2>&1
            $pullExitCode = $LASTEXITCODE
        } finally {
            $ErrorActionPreference = $previousErrorActionPreference
        }
        if ($pullExitCode -eq 0) {
            foreach ($line in $pullOutput) {
                Write-Host $line
            }
            return $true
        }

        foreach ($line in $pullOutput) {
            Write-Warning $line
        }
        Write-Warning "镜像拉取失败: $($Item.Ref), attempt=$attempt, exit=$pullExitCode"
        if (-not $NoCleanupFailedPull -and -not $hadImageBeforePull) {
            $previousErrorActionPreference = $ErrorActionPreference
            $ErrorActionPreference = "Continue"
            try {
                & docker image rm $Item.Ref 1>$null 2>$null
                $cleanupExitCode = $LASTEXITCODE
            } finally {
                $ErrorActionPreference = $previousErrorActionPreference
            }
            if ($cleanupExitCode -ne 0) {
                Write-Warning "清理失败镜像引用未完成: $($Item.Ref), exit=$cleanupExitCode"
            }
        } elseif ($hadImageBeforePull) {
            Write-Warning "保留本地已有 digest 镜像: $($Item.Ref)"
        }
        if ($attempt -lt $MaxAttempts -and $RetryDelaySeconds -gt 0) {
            Start-Sleep -Seconds $RetryDelaySeconds
        }
    }
    return $false
}

function Test-LocalImageRef {
    param([string]$Ref)
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & docker image inspect $Ref 1>$null 2>$null
        return ($LASTEXITCODE -eq 0)
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
}

$rootPath = (Resolve-Path -LiteralPath $Root).Path
$digestLockItems = Read-DigestLock -Path $DigestLock
$manifests = Get-ChildItem -Path $rootPath -Recurse -Filter manifest.yaml | Sort-Object FullName
$items = New-Object System.Collections.Generic.List[object]
$seen = @{}
$missing = New-Object System.Collections.Generic.List[string]

foreach ($manifest in $manifests) {
    $sourceType = Read-SourceType -Path $manifest.FullName
    switch ($sourceType) {
        "upstream-pinned" {
            if ($Scope -in @("all", "upstream-pinned")) {
                Add-UpstreamRefs -ManifestPath $manifest.FullName -Items $items -Seen $seen -Missing $missing
            }
        }
        { $_ -in @("platform-built", "thin-wrapper", "build-base") } {
            if ($Scope -in @("all", "built")) {
                Add-BuiltRef -ManifestPath $manifest.FullName -DigestLockItems $digestLockItems -Items $items -Seen $seen -Missing $missing
            }
        }
        default {
            $missing.Add("$($manifest.FullName): source.type 非法或缺失")
        }
    }
}

if ($missing.Count -gt 0) {
    Write-Error ("Refusing to pull images because immutable digests are missing or invalid:`n" + ($missing -join "`n"))
    exit 1
}

$failures = New-Object System.Collections.Generic.List[string]
foreach ($item in $items) {
    if (-not (Invoke-DockerPullWithRetry -Item $item)) {
        $failures.Add("$($item.Ref) ($($item.Manifest))")
        break
    }
}

if ($failures.Count -gt 0) {
    Write-Error ("Image pull failed after retries:`n" + ($failures -join "`n"))
    exit 1
}

Write-Host "Pulled $($items.Count) image references successfully. scope=$Scope registry=$Registry"
