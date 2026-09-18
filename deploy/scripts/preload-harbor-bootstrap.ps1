# preload-harbor-bootstrap 将已验签的 Harbor OCI 引导包预置到 Harbor 目标集群的每个节点。
param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\\..")).Path,
    [string]$ConfigPath = "",
    [string]$EvidencePath = "",
    [int]$DockerTimeoutSeconds = 300
)

$ErrorActionPreference = "Stop"
if ($DockerTimeoutSeconds -lt 1) {
    throw "DockerTimeoutSeconds 必须大于等于 1"
}

# Get-ConfigValue 读取严格键值配置且拒绝缺失或空白项。
function Get-ConfigValue {
    param([hashtable]$Config, [string]$Key)
    $value = [string]$Config[$Key]
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "缺少 $Key"
    }
    return $value
}

# Assert-Sha256 阻止将与已验签证据不一致的 OCI 包导入节点。
function Assert-Sha256 {
    param([string]$Path, [string]$Expected, [string]$Description)
    if ($Expected -notmatch "^sha256:[0-9a-f]{64}$") {
        throw "$Description 的 sha256 非法: $Expected"
    }
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "缺少 ${Description}: $Path"
    }
    $actual = "sha256:" + (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $Expected) {
        throw "$Description 摘要不匹配: expected=$Expected actual=$actual"
    }
}

# ConvertTo-ProcessArgument 将参数安全编码为 Windows 原生进程参数。
function ConvertTo-ProcessArgument([string]$Value) {
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

# Invoke-DockerBounded 对节点导入和校验设置明确时限，避免 Docker Desktop 异常时无限等待。
function Invoke-DockerBounded {
    param([string[]]$Arguments, [int]$TimeoutSeconds)
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = [Diagnostics.ProcessStartInfo]::new()
    $process.StartInfo.FileName = "docker.exe"
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.CreateNoWindow = $true
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    $process.StartInfo.Arguments = (($Arguments | ForEach-Object { ConvertTo-ProcessArgument ([string]$_) }) -join " ")
    try {
        if (-not $process.Start()) { throw "Docker CLI 启动失败" }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
            try { $process.Kill($true) } catch { }
            throw "Docker CLI 超时（$TimeoutSeconds 秒）: $($Arguments -join ' ')"
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        $text = (@($stdout, $stderr) | Where-Object { $_ }) -join "`n"
        if ($process.ExitCode -ne 0) {
            throw "Docker CLI 失败（exit=$($process.ExitCode)）: $($Arguments -join ' '); $text"
        }
        return $text.Trim()
    }
    finally {
        $process.Dispose()
    }
}

# Restart-ContainerdAndAssertCanonicalIdentity 刷新 CRI 对 OCI 导入记录的缓存，拒绝旧 import 别名作为运行时身份。
function Restart-ContainerdAndAssertCanonicalIdentity {
    param(
        [string]$Node,
        [string[]]$ExpectedRefs,
        [int]$TimeoutSeconds
    )
    [void](Invoke-DockerBounded -Arguments @("exec", $Node, "systemctl", "restart", "containerd") -TimeoutSeconds $TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastReason = "containerd 尚未返回 canonical 镜像身份"
    do {
        try {
            $active = Invoke-DockerBounded -Arguments @("exec", $Node, "systemctl", "is-active", "containerd") -TimeoutSeconds 30
            if ($active.Trim() -ne "active") {
                $lastReason = "containerd 状态不是 active: $active"
            }
            else {
                $valid = $true
                foreach ($expectedRef in $ExpectedRefs) {
                    $image = (Invoke-DockerBounded -Arguments @("exec", $Node, "crictl", "inspecti", $expectedRef) -TimeoutSeconds 30 | ConvertFrom-Json).status
                    $repoDigests = @($image.repoDigests)
                    $repoTags = @($image.repoTags)
                    if ($repoDigests -notcontains $expectedRef -or $repoTags.Count -ne 0) {
                        $valid = $false
                        $lastReason = "CRI 镜像身份未闭合: node=$Node image=$expectedRef repoDigests=$($repoDigests -join ',') repoTags=$($repoTags -join ',')"
                        break
                    }
                }
                if ($valid) {
                    return
                }
            }
        }
        catch {
            $lastReason = $_.Exception.Message
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    throw "Harbor 节点 CRI canonical 身份刷新超时: node=$Node reason=$lastReason"
}

# Resolve-EvidenceFile 限制证据引用只能位于已验证证据目录。
function Resolve-EvidenceFile {
    param([string]$EvidenceDirectory, [string]$RelativePath, [string]$Description)
    if ([string]::IsNullOrWhiteSpace($RelativePath) -or [System.IO.Path]::IsPathRooted($RelativePath)) {
        throw "$Description 必须是证据目录内的相对路径"
    }
    $path = [System.IO.Path]::GetFullPath((Join-Path $EvidenceDirectory $RelativePath))
    if (-not $path.StartsWith($EvidenceDirectory + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "$Description 越出证据目录: $RelativePath"
    }
    return $path
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
Import-Module (Join-Path $RepoRoot "images\\lib\\ImageMetadata.psm1") -Force
if ([string]::IsNullOrWhiteSpace($ConfigPath)) {
    $ConfigPath = Join-Path $RepoRoot "deploy\\config\\chaimir.env"
}
$config = Read-ChaimirEnvFile -Path ([System.IO.Path]::GetFullPath($ConfigPath)) -RequiredKeys @(
    "SUPPLY_CHAIN_HARBOR_KUBECONFIG_HOST_PATH",
    "SUPPLY_CHAIN_HARBOR_BOOTSTRAP_EVIDENCE_PATH"
)
if ([string]::IsNullOrWhiteSpace($EvidencePath)) {
    $EvidencePath = Join-Path $RepoRoot (Get-ConfigValue -Config $config -Key "SUPPLY_CHAIN_HARBOR_BOOTSTRAP_EVIDENCE_PATH")
}
$EvidencePath = [System.IO.Path]::GetFullPath($EvidencePath)
$evidenceDirectory = Split-Path -Parent $EvidencePath
if (-not (Test-Path -LiteralPath $EvidencePath -PathType Leaf)) {
    throw "缺少已经由 prepare-harbor-chart 验签的 Harbor bootstrap 证据: $EvidencePath"
}
$evidence = Get-Content -LiteralPath $EvidencePath -Raw | ConvertFrom-Json
$bundlePath = Resolve-EvidenceFile -EvidenceDirectory $evidenceDirectory -RelativePath ([string]$evidence.bundle.file) -Description "Harbor OCI 引导包"
Assert-Sha256 -Path $bundlePath -Expected ([string]$evidence.bundle.sha256) -Description "Harbor OCI 引导包"

$kubeconfigPath = Join-Path (Join-Path $RepoRoot "deploy") (Get-ConfigValue -Config $config -Key "SUPPLY_CHAIN_HARBOR_KUBECONFIG_HOST_PATH")
$kubeconfigPath = [System.IO.Path]::GetFullPath($kubeconfigPath)
if (-not (Test-Path -LiteralPath $kubeconfigPath -PathType Leaf)) {
    throw "缺少 Harbor kubeconfig: $kubeconfigPath"
}
$nodesRaw = & kubectl.exe --kubeconfig $kubeconfigPath get nodes -o json
if ($LASTEXITCODE -ne 0) {
    throw "无法读取 Harbor 目标节点，拒绝跳过节点预置"
}
$nodes = @((($nodesRaw -join "`n") | ConvertFrom-Json).items | ForEach-Object { [string]$_.metadata.name } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
if ($nodes.Count -eq 0) {
    throw "Harbor 目标节点为空，拒绝跳过节点预置"
}

$expectedComponents = @($evidence.components)
$expectedRefs = @($expectedComponents | ForEach-Object { "$($_.repository)@$($_.digest)" })
if ($expectedRefs.Count -eq 0) {
    throw "Harbor bootstrap 证据未声明组件镜像"
}
$nodeBundlePath = "/var/lib/chaimir/harbor-bootstrap.oci.tar"
foreach ($node in $nodes) {
    $nodeInfo = Invoke-DockerBounded -Arguments @("inspect", "--format", "{{ index .Config.Labels `"io.x-k8s.kind.cluster`" }}|{{ .State.Running }}", $node) -TimeoutSeconds $DockerTimeoutSeconds
    if ($nodeInfo.Trim() -ne "desktop|true") {
        throw "Harbor 节点容器身份或状态非法: node=$node actual=$nodeInfo"
    }
    try {
        [void](Invoke-DockerBounded -Arguments @("exec", $node, "mkdir", "-p", "/var/lib/chaimir") -TimeoutSeconds 60)
        [void](Invoke-DockerBounded -Arguments @("cp", $bundlePath, "${node}:$nodeBundlePath") -TimeoutSeconds $DockerTimeoutSeconds)
        [void](Invoke-DockerBounded -Arguments @("exec", $node, "ctr", "-n", "k8s.io", "images", "import", "--digests", "--all-platforms", $nodeBundlePath) -TimeoutSeconds $DockerTimeoutSeconds)
        $availableRefs = @((Invoke-DockerBounded -Arguments @("exec", $node, "ctr", "-n", "k8s.io", "images", "list", "-q") -TimeoutSeconds $DockerTimeoutSeconds) -split "`r?`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ })
        $temporaryPrefix = $null
        foreach ($component in $expectedComponents) {
            $componentID = [string]$component.id
            if ($componentID -notmatch "^[a-z0-9-]+$") {
                throw "Harbor bootstrap 组件 id 非法: $componentID"
            }
            $sourceRefs = @($availableRefs | Where-Object { $_ -match "^(import-[0-9]{4}-[0-9]{2}-[0-9]{2}):$([regex]::Escape($componentID))$" })
            if ($sourceRefs.Count -ne 1) {
                throw "Harbor OCI 引导包组件来源不唯一: node=$node component=$componentID actual=$($sourceRefs -join ',')"
            }
            $sourceRef = $sourceRefs[0]
            $prefix = ($sourceRef -split ":", 2)[0]
            if ($null -eq $temporaryPrefix) {
                $temporaryPrefix = $prefix
            }
            elseif ($temporaryPrefix -ne $prefix) {
                throw "Harbor OCI 引导包组件未使用同一受控导入前缀: node=$node expected=$temporaryPrefix actual=$prefix"
            }
            $canonicalRef = "$($component.repository)@$($component.digest)"
            [void](Invoke-DockerBounded -Arguments @("exec", $node, "ctr", "-n", "k8s.io", "images", "tag", $sourceRef, $canonicalRef) -TimeoutSeconds $DockerTimeoutSeconds)
        }
        if ([string]::IsNullOrWhiteSpace($temporaryPrefix)) {
            throw "Harbor OCI 引导包未提供可清理的受控导入前缀: node=$node"
        }
        $availableRefs = @((Invoke-DockerBounded -Arguments @("exec", $node, "ctr", "-n", "k8s.io", "images", "list", "-q") -TimeoutSeconds $DockerTimeoutSeconds) -split "`r?`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ })
        foreach ($expectedRef in $expectedRefs) {
            if ($availableRefs -notcontains $expectedRef) {
                throw "Harbor 节点缺少已预置组件: node=$node image=$expectedRef"
            }
        }
        $temporaryRefs = @($availableRefs | Where-Object { $_ -match "^$([regex]::Escape($temporaryPrefix))(:|@)" })
        if ($temporaryRefs.Count -eq 0) {
            throw "Harbor OCI 引导临时引用缺失，拒绝跳过清理: node=$node prefix=$temporaryPrefix"
        }
        foreach ($temporaryRef in $temporaryRefs) {
            [void](Invoke-DockerBounded -Arguments @("exec", $node, "ctr", "-n", "k8s.io", "images", "rm", $temporaryRef) -TimeoutSeconds $DockerTimeoutSeconds)
        }
        $availableRefs = @((Invoke-DockerBounded -Arguments @("exec", $node, "ctr", "-n", "k8s.io", "images", "list", "-q") -TimeoutSeconds $DockerTimeoutSeconds) -split "`r?`n" | ForEach-Object { $_.Trim() } | Where-Object { $_ })
        foreach ($expectedRef in $expectedRefs) {
            if ($availableRefs -notcontains $expectedRef) {
                throw "清理引导临时引用后丢失 Harbor 组件: node=$node image=$expectedRef"
            }
        }
        Restart-ContainerdAndAssertCanonicalIdentity -Node $node -ExpectedRefs $expectedRefs -TimeoutSeconds $DockerTimeoutSeconds
        Write-Output "preloaded_node=$node components=$($expectedRefs.Count)"
    }
    finally {
        try { [void](Invoke-DockerBounded -Arguments @("exec", $node, "rm", "-f", $nodeBundlePath) -TimeoutSeconds 60) } catch { Write-Warning "清理节点引导包失败: node=$node error=$($_.Exception.Message)" }
        try { [void](Invoke-DockerBounded -Arguments @("exec", $node, "rmdir", "/var/lib/chaimir") -TimeoutSeconds 60) } catch { Write-Warning "清理节点引导目录失败: node=$node error=$($_.Exception.Message)" }
    }
}
