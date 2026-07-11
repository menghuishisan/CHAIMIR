# 本脚本按 manifest 与构建产物 digest 锁顺序拉取 Chaimir 镜像。
param(
    [string]$Root = (Split-Path -Parent $MyInvocation.MyCommand.Path),
    [ValidateSet("all", "upstream-pinned", "built")]
    [string]$Scope = "all",
    [string]$Registry = $env:CHAIMIR_IMAGE_REGISTRY,
    [string]$DockerConfig = $env:DOCKER_CONFIG,
    [string]$DigestLock = "",
    [switch]$PrintBuildArgs,
    [int]$MaxAttempts = 3,
    [int]$RetryDelaySeconds = 5,
    [switch]$FailFast,
    [switch]$NoCleanupFailedPull
)

$ErrorActionPreference = "Stop"
Import-Module (Join-Path $PSScriptRoot "lib\ImageMetadata.psm1") -Force

if ([string]::IsNullOrWhiteSpace($Registry)) {
    $Registry = $env:SUPPLY_CHAIN_REGISTRY
}
if ([string]::IsNullOrWhiteSpace($Registry)) {
    $Registry = $env:IMAGE_REGISTRY
}
if ([string]::IsNullOrWhiteSpace($Registry)) {
    $Registry = "harbor.chaimir:30080"
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
if ($PrintBuildArgs -and $Scope -eq "upstream-pinned") {
    throw "PrintBuildArgs 只能与 Scope=all 或 Scope=built 一起使用"
}

# Test-DeployableManifest 判断 manifest 是否允许进入拉取、扫描和准入流程。
function Test-DeployableManifest {
    param([string]$Path)
    $supplyChain = Get-ChaimirYamlBlock -Path $Path -BlockName "supply_chain"
    $deployable = Get-ChaimirYamlValue -Lines $supplyChain -Key "deployable"
    if ($deployable -ne "false") {
        return $true
    }
    $reason = Get-ChaimirYamlValue -Lines $supplyChain -Key "block_reason"
    if ([string]::IsNullOrWhiteSpace($reason)) {
        throw "${Path}: supply_chain.deployable=false 必须声明 block_reason"
    }
    Write-Host "Skipping non-deployable image manifest: $Path ($reason)"
    return $false
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

function Get-BuildArgRef {
    param(
        [hashtable]$DigestLockItems,
        [string]$ImageName,
        [string]$ArgName
    )
    $digest = $DigestLockItems[$ImageName]
    if ([string]::IsNullOrWhiteSpace($digest)) {
        throw "无法生成 $ArgName, digest 锁缺少 $ImageName"
    }
    return "$ArgName=$Registry/$ImageName@$digest"
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
    $upstream = Get-ChaimirYamlBlock -Path $ManifestPath -BlockName "upstream"
    $registry = Get-ChaimirYamlValue -Lines $upstream -Key "registry"
    $image = Get-ChaimirYamlValue -Lines $upstream -Key "image"
    $digest = Get-ChaimirYamlValue -Lines $upstream -Key "digest"
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
    $image = Get-ChaimirTopLevelYamlValue -Lines $lines -Key "image"
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

function Invoke-CheckedDocker {
    param(
        [string[]]$Arguments,
        [string]$Context
    )
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $output = Invoke-DockerCli -Arguments $Arguments 2>&1
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    if ($exitCode -ne 0) {
        foreach ($line in $output) {
            Write-Warning $line
        }
        throw "$Context 失败, exit=$exitCode"
    }
    foreach ($line in $output) {
        Write-Host $line
    }
    return ,$output
}

function Invoke-DockerCli {
    param([string[]]$Arguments)
    $baseArguments = @()
    if (-not [string]::IsNullOrWhiteSpace($DockerConfig)) {
        $baseArguments += @("--config", $DockerConfig)
    }
    & docker @baseArguments @Arguments
}

function Invoke-DockerPullWithRetry {
    param([object]$Item)
    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        Write-Host "Pulling [$attempt/$MaxAttempts] $($Item.Ref)"
        $hadImageBeforePull = Test-LocalImageRef -Ref $Item.Ref
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        try {
            $pullOutput = Invoke-DockerCli -Arguments @("pull", $Item.Ref) 2>&1
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
                Invoke-DockerCli -Arguments @("image", "rm", $Item.Ref) 1>$null 2>$null
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
        Invoke-DockerCli -Arguments @("image", "inspect", $Ref) 1>$null 2>$null
        return ($LASTEXITCODE -eq 0)
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
}

$rootPath = (Resolve-Path -LiteralPath $Root).Path
$manifests = Get-ChildItem -Path $rootPath -Recurse -Filter manifest.yaml | Sort-Object FullName
$digestLockItems = Read-ChaimirDigestLock -Path $DigestLock
if ($PrintBuildArgs) {
    Write-Output (Get-BuildArgRef -DigestLockItems $digestLockItems -ImageName "base/go-builder" -ArgName "GO_BUILDER_IMAGE")
    Write-Output (Get-BuildArgRef -DigestLockItems $digestLockItems -ImageName "base/judge-min" -ArgName "JUDGE_MIN_IMAGE")
    exit 0
}
$items = New-Object System.Collections.Generic.List[object]
$seen = @{}
$missing = New-Object System.Collections.Generic.List[string]

foreach ($manifest in $manifests) {
    if (-not (Test-DeployableManifest -Path $manifest.FullName)) {
        continue
    }
    $sourceType = Get-ChaimirImageSourceType -Path $manifest.FullName
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
    if (Test-LocalImageRef -Ref $item.Ref) {
        Write-Host "Skipping existing image $($item.Ref)"
        continue
    }
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
