# 本文件负责在 Docker Desktop/WSL 恢复后自动恢复 Chaimir Cilium 集群。
param(
    [ValidateSet("Watch", "Install", "Uninstall", "RunOnce")]
    [string]$Action = "Watch",
    [int]$PollSeconds = 5
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$deployRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$stateRoot = Join-Path $repoRoot ".tmp\docker-runtime"
$statePath = Join-Path $stateRoot "watcher.state.json"
$logPath = Join-Path $stateRoot "watcher.log"
$taskName = "Chaimir.DockerRuntimeWatcher"
$clusterContext = "kind-chaimir-cilium"
$dependencyContext = "docker-desktop"
$edgeNamespace = "chaimir-edge"
$edgeEndpointSlice = "chaimir-cilium-ingress"
$controlPlaneContainer = "chaimir-cilium-control-plane"
$scriptPath = $PSCommandPath
New-Item -ItemType Directory -Force -Path $stateRoot | Out-Null

# Stop-ProcessTree 在外部命令超时时终止整个进程树，避免探测子进程长期残留。
function Stop-ProcessTree {
    param([Parameter(Mandatory = $true)][Diagnostics.Process]$Process)

    try {
        & taskkill.exe /PID $Process.Id /T /F | Out-Null
    } catch {
        Write-WatcherLog "终止进程树失败，进程可能已退出或当前账户无权限: $($_.Exception.Message)"
    }
    try {
        if (-not $Process.HasExited) { $Process.Kill() }
    } catch {
        Write-WatcherLog "终止超时进程失败，进程可能已退出或当前账户无权限: $($_.Exception.Message)"
    }
}

function Write-WatcherLog {
    param([Parameter(Mandatory = $true)][string]$Message)
    $line = "[$(Get-Date -Format o)] $Message"
    Add-Content -LiteralPath $logPath -Value $line -Encoding UTF8
    Write-Host $line
}

function Invoke-ProcessWithTimeout {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [int]$TimeoutSeconds = 30
    )
    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = New-Object System.Diagnostics.ProcessStartInfo
    $process.StartInfo.FileName = $FilePath
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.CreateNoWindow = $true
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    $process.StartInfo.Arguments = [string]::Join(" ", ($Arguments | ForEach-Object {
        if ($_ -match '[\s"]') { '"' + ($_ -replace '(\\*)"', '$1$1\"') + '"' } else { $_ }
    }))
    try {
        [void]$process.Start()
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
            Stop-ProcessTree -Process $process
            return [pscustomobject]@{ ExitCode = $null; TimedOut = $true; Stdout = ""; Stderr = "timeout" }
        }
        $stdoutTask.Wait()
        $stderrTask.Wait()
        return [pscustomobject]@{
            ExitCode = $process.ExitCode
            TimedOut = $false
            Stdout = $stdoutTask.Result
            Stderr = $stderrTask.Result
        }
    } finally {
        $process.Dispose()
    }
}

function Test-DockerReady {
    $result = Invoke-ProcessWithTimeout -FilePath "docker.exe" -Arguments @("info", "--format", "{{.ServerVersion}}") -TimeoutSeconds 30
    return (-not $result.TimedOut -and $result.ExitCode -eq 0 -and $result.Stdout.Trim() -match '^\S+$')
}

# Test-ExternalEdgeReady 检查外部 Nginx 的 EndpointSlice 是否仍指向当前控制平面地址。
# Docker 重启后 Kind 节点 IP 可能变化;节点和 Cilium Pod 即使已 Ready,旧地址仍会让入口返回 502。
function Test-ExternalEdgeReady {
    $nodeResult = Invoke-ProcessWithTimeout -FilePath "docker.exe" -Arguments @(
        "inspect", $controlPlaneContainer, "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}"
    ) -TimeoutSeconds 30
    if ($nodeResult.TimedOut -or $nodeResult.ExitCode -ne 0) {
        return $false
    }
    $nodeIP = $nodeResult.Stdout.Trim()
    if ($nodeIP -notmatch '^([0-9]{1,3}\.){3}[0-9]{1,3}$') {
        return $false
    }

    $endpointResult = Invoke-ProcessWithTimeout -FilePath "kubectl.exe" -Arguments @(
        "--context", $dependencyContext, "-n", $edgeNamespace, "get", "endpointslice", $edgeEndpointSlice,
        "-o", "jsonpath={.endpoints[0].addresses[0]} {.endpoints[0].conditions.ready} {.ports[0].port}"
    ) -TimeoutSeconds 30
    if ($endpointResult.TimedOut -or $endpointResult.ExitCode -ne 0) {
        return $false
    }
    $parts = @($endpointResult.Stdout.Trim() -split '\s+')
    return ($parts.Count -eq 3 -and $parts[0] -eq $nodeIP -and $parts[1] -eq 'true' -and $parts[2] -eq '30443')
}

# Test-CiliumReady 检查项目 Kind API 与全部节点是否真实 Ready。
# 不能只依据 Docker API、节点状态或上一次状态文件,还必须确认外部 HTTPS 入口已同步。
function Test-CiliumReady {
    $result = Invoke-ProcessWithTimeout -FilePath "kubectl.exe" -Arguments @(
        "--context", $clusterContext, "get", "nodes", "--no-headers"
    ) -TimeoutSeconds 30
    if ($result.TimedOut -or $result.ExitCode -ne 0) {
        return $false
    }
    $lines = @($result.Stdout -split "`r?`n" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    if ($lines.Count -lt 3) {
        return $false
    }
    foreach ($line in $lines) {
        $columns = @($line -split "\s+")
        if ($columns.Count -lt 2 -or $columns[1] -ne "Ready") {
            return $false
        }
    }

    # 节点 Ready 不代表数据面已恢复；Docker 重启后 kubelet 可能先报告节点，
    # 但 Cilium/CoreDNS 仍处于 Unknown。必须检查关键 kube-system Pod 的真实容器状态。
    $podResult = Invoke-ProcessWithTimeout -FilePath "kubectl.exe" -Arguments @(
        "--context", $clusterContext, "-n", "kube-system", "get", "pods", "-o", "json"
    ) -TimeoutSeconds 30
    if ($podResult.TimedOut -or $podResult.ExitCode -ne 0) {
        return $false
    }
    try {
        $podList = $podResult.Stdout | ConvertFrom-Json
    } catch {
        Write-WatcherLog "kube-system Pod 状态解析失败，将重新探测: $($_.Exception.Message)" | Out-Null
        return $false
    }
    $criticalPods = @($podList.items | Where-Object {
        $_.metadata.labels.'k8s-app' -in @('cilium', 'kube-dns') -or
        $_.metadata.labels.'k8s-app' -eq 'cilium-envoy' -or
        $_.metadata.labels.'io.cilium/app' -eq 'operator' -or
        $_.metadata.name -like 'cilium-operator-*'
    })
    if ($criticalPods.Count -lt 3) {
        return $false
    }
    foreach ($pod in $criticalPods) {
        if ($pod.status.phase -ne 'Running') {
            return $false
        }
        $statuses = @($pod.status.containerStatuses)
        if ($statuses.Count -eq 0 -or @($statuses | Where-Object { -not $_.ready }).Count -gt 0) {
            return $false
        }
    }
    return (Test-ExternalEdgeReady)
}

function Read-WatcherState {
    if (-not (Test-Path -LiteralPath $statePath)) { return $false }
    try {
        $state = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
        return [bool]$state.docker_ready
    } catch {
        Write-WatcherLog "状态文件解析失败，将重新探测: $($_.Exception.Message)"
        return $false
    }
}

function Write-WatcherState {
    param([bool]$DockerReady)
    $payload = [pscustomobject]@{
        docker_ready = $DockerReady
        checked_at = (Get-Date).ToUniversalTime().ToString("o")
    } | ConvertTo-Json -Compress
    [System.IO.File]::WriteAllText($statePath, $payload, (New-Object System.Text.UTF8Encoding($false)))
}

function Restore-Cilium {
    Write-WatcherLog "Docker API 已恢复，开始执行 Cilium 集群恢复" | Out-Null
    $result = Invoke-ProcessWithTimeout -FilePath "powershell.exe" -Arguments @(
        "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", (Join-Path $deployRoot "scripts\cilium-cluster.ps1"), "-Action", "Start"
    ) -TimeoutSeconds 900
    if ($result.TimedOut) {
        Write-WatcherLog "Cilium 恢复超时(900s)，下次 Docker 状态转换时重试" | Out-Null
        return $false
    }
    if ($result.ExitCode -ne 0) {
        $detail = (($result.Stderr + " " + $result.Stdout).Trim() -replace "\s+", " ")
        Write-WatcherLog "Cilium 恢复失败(exit=$($result.ExitCode)): $detail" | Out-Null
        return $false
    }
    Write-WatcherLog "Cilium 集群恢复完成" | Out-Null
    return $true
}

function Install-Watcher {
    $userId = "$env:USERDOMAIN\$env:USERNAME"
    $action = New-ScheduledTaskAction -Execute "$PSHOME\powershell.exe" -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$scriptPath`" -Action Watch"
    $trigger = New-ScheduledTaskTrigger -AtLogOn -User $userId
    $principal = New-ScheduledTaskPrincipal -UserId $userId -LogonType Interactive -RunLevel Limited
    $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew
    Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
    Write-WatcherLog "已安装用户级计划任务 $taskName"
}

function Uninstall-Watcher {
    $task = $null
    try {
        $task = Get-ScheduledTask -TaskName $taskName -ErrorAction Stop
    } catch {
        # 仅把 Windows 明确的 ERROR_FILE_NOT_FOUND 视为幂等卸载；权限和 CIM 故障必须上抛。
        if ($_.Exception.HResult -ne -2147024894 -and $_.FullyQualifiedErrorId -notmatch "TaskNotFound|NoMatching") {
            throw
        }
    }
    if ($null -ne $task) {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    }
    Write-WatcherLog "已卸载用户级计划任务 $taskName"
}

if ($Action -eq "Install") { Install-Watcher; return }
if ($Action -eq "Uninstall") { Uninstall-Watcher; return }

$mutex = New-Object System.Threading.Mutex($false, "Chaimir.DockerRuntimeWatcher")
if (-not $mutex.WaitOne(0)) { Write-WatcherLog "已有 watcher 实例运行，当前实例退出"; return }
try {
    do {
        $ready = Test-DockerReady
        $effectiveReady = $false
        if ($ready) {
            $clusterReady = Test-CiliumReady
            if (-not $clusterReady) {
                # 以实际集群状态为准；即使状态文件仍为 ready，也必须修复重启后未恢复的节点。
                $clusterReady = Restore-Cilium
            }
            $effectiveReady = $clusterReady
        }
        Write-WatcherState -DockerReady $effectiveReady
        if ($Action -eq "RunOnce") { break }
        Start-Sleep -Seconds ([Math]::Max(2, $PollSeconds))
    } while ($true)
} finally {
    $mutex.ReleaseMutex()
    $mutex.Dispose()
}
