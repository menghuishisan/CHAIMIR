# 本文件负责在用户登录后自动恢复Docker/WSL、既有Cilium集群及动态HTTPS入口。
param(
    [ValidateSet("Watch", "Install", "Uninstall", "RunOnce", "Suspend", "Resume")]
    [string]$Action = "Watch"
)

$ErrorActionPreference = "Stop"
$ProgressPreference = 'SilentlyContinue'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$deployRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$stateRoot = Join-Path $repoRoot ".tmp\docker-runtime"
$statePath = Join-Path $stateRoot "watcher.state.json"
$logPath = Join-Path $stateRoot "watcher.log"
$suspendPath = Join-Path $stateRoot 'watcher.suspended'
$taskName = "Chaimir.DockerRuntimeWatcher"
$clusterContext = "kind-chaimir-cilium"
$dependencyContext = "docker-desktop"
$edgeNamespace = "chaimir-edge"
$edgeEndpointSlice = "chaimir-cilium-ingress"
$controlPlaneContainer = "chaimir-cilium-control-plane"
$scriptPath = $PSCommandPath
$powerShellPath = Join-Path $env:SystemRoot 'System32\WindowsPowerShell\v1.0\powershell.exe'
New-Item -ItemType Directory -Force -Path $stateRoot | Out-Null

# Stop-ProcessTree 在外部命令超时时终止整个进程树，避免探测子进程长期残留。
function Stop-ProcessTree {
    param([Parameter(Mandatory = $true)][Diagnostics.Process]$Process)

    try {
        & taskkill.exe /PID $Process.Id /T /F | Out-Null
        if ($LASTEXITCODE -ne 0) { Write-WatcherLog "终止超时进程树返回$LASTEXITCODE,继续核对进程是否已退出" }
    } catch {
        Write-WatcherLog "终止进程树失败，进程可能已退出或当前账户无权限: $($_.Exception.Message)"
    }
    try {
        if (-not $Process.HasExited) { $Process.Kill() }
    } catch {
        Write-WatcherLog "终止超时进程失败，进程可能已退出或当前账户无权限: $($_.Exception.Message)"
    }
}

# Write-WatcherLog 保存探测阶段、状态变化和实际恢复结果,不记录凭据或业务正文。
function Write-WatcherLog {
    param([Parameter(Mandatory = $true)][string]$Message)
    $line = "[$(Get-Date -Format o)] $Message"
    Add-Content -LiteralPath $logPath -Value $line -Encoding UTF8
    # 后台计划任务只写文件,不依赖隐藏控制台的输出通道。
    if ($Action -ne 'Watch') { Write-Host $line }
}

# Invoke-ProcessWithTimeout 有界执行基础设施命令并回收超时进程树。
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

# Test-DockerReady 确认本轮 Docker API 可响应,不复用历史状态文件。
function Test-DockerReady {
    $result = Invoke-ProcessWithTimeout -FilePath "docker.exe" -Arguments @("info", "--format", "{{.ServerVersion}}") -TimeoutSeconds 30
    if ($result.TimedOut) { throw 'Docker API 探测超时' }
    if ($result.ExitCode -ne 0) { throw "Docker API 探测失败(exit=$($result.ExitCode)): $($result.Stderr.Trim())" }
    return ($result.Stdout.Trim() -match '^\S+$')
}

# Get-NodeRuntimeNetworkState 只读节点线程状态,识别Docker API可用时仍会阻断沙箱创建的内核网络等待。
function Get-NodeRuntimeNetworkState {
    param(
        [Parameter(Mandatory = $true)][string[]]$NodeNames,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )

    if ($NodeNames.Count -eq 0) { throw '节点列表为空,无法检查运行时网络控制' }
    foreach ($nodeName in $NodeNames) {
        if ($nodeName -notmatch '^[a-z0-9][a-z0-9.-]*$') { throw '节点容器名称无效' }
        $probe = Invoke-ProcessWithTimeout -FilePath 'docker.exe' -Arguments @(
            'exec', '--user', '0', $nodeName, 'ps', '-eLo', 'stat=,wchan:40=,comm='
        ) -TimeoutSeconds $TimeoutSeconds
        if ($probe.TimedOut -or $probe.ExitCode -ne 0) {
            return [pscustomobject]@{ Ready = $false; RecoverRuntime = [bool]$probe.TimedOut; Samples = @($nodeName); Error = "节点 $nodeName 的运行时状态读取失败(exit=$($probe.ExitCode),timeout=$($probe.TimedOut))" }
        }
        $threads = @($probe.Stdout -split '\r?\n' | ForEach-Object {
            if ($_ -match '^\s*(\S+)\s+(\S+)\s+(\S+)\s*$') {
                [pscustomobject]@{ State = $Matches[1]; Wait = $Matches[2]; Name = $Matches[3] }
            }
        })
        $missing = @(@('containerd', 'kubelet') | Where-Object { $_ -notin $threads.Name })
        if ($missing.Count -gt 0) {
            return [pscustomobject]@{ Ready = $false; RecoverRuntime = $false; Samples = @($nodeName); Error = "节点 $nodeName 缺少运行时进程状态: $($missing -join ',')" }
        }
        $blocked = @($threads | Where-Object {
            $_.State.StartsWith('D') -and $_.Name -in @('containerd', 'kubelet', 'cilium-agent') -and
                $_.Wait -match '^(rtnl_|rtnetlink_|register_nexthop_notifier)'
        })
        if ($blocked.Count -gt 0) {
            $samples = @($blocked | Select-Object -First 5 | ForEach-Object { "$nodeName $($_.Name) $($_.State) $($_.Wait)" })
            return [pscustomobject]@{ Ready = $false; RecoverRuntime = $true; Samples = $samples; Error = "节点 $nodeName 的核心进程正在等待内核网络锁" }
        }
    }
    return [pscustomobject]@{ Ready = $true; RecoverRuntime = $false; Samples = @(); Error = $null }
}

# Test-ExternalEdgeReady 检查外部 Nginx 的 EndpointSlice 是否仍指向当前控制平面地址。
# Docker 重启后 Kind 节点 IP 可能变化;节点和 Cilium Pod 即使已 Ready,旧地址仍会让入口返回 502。
function Test-ExternalEdgeReady {
    $nodeResult = Invoke-ProcessWithTimeout -FilePath "docker.exe" -Arguments @(
        "inspect", $controlPlaneContainer, "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}"
    ) -TimeoutSeconds 30
    if ($nodeResult.TimedOut) { throw '外部入口控制平面地址探测超时' }
    if ($nodeResult.ExitCode -ne 0) { throw "外部入口控制平面地址探测失败: $($nodeResult.Stderr.Trim())" }
    $nodeIP = $nodeResult.Stdout.Trim()
    if ($nodeIP -notmatch '^([0-9]{1,3}\.){3}[0-9]{1,3}$') {
        return $false
    }

    $endpointResult = Invoke-ProcessWithTimeout -FilePath "kubectl.exe" -Arguments @(
        "--context", $dependencyContext, "-n", $edgeNamespace, "get", "endpointslice", $edgeEndpointSlice,
        "-o", "jsonpath={.endpoints[0].addresses[0]} {.endpoints[0].conditions.ready} {.ports[0].port}"
    ) -TimeoutSeconds 30
    if ($endpointResult.TimedOut) { throw '外部入口 EndpointSlice 探测超时' }
    if ($endpointResult.ExitCode -ne 0) { throw "外部入口 EndpointSlice 探测失败: $($endpointResult.Stderr.Trim())" }
    $parts = @($endpointResult.Stdout.Trim() -split '\s+')
    return ($parts.Count -eq 3 -and $parts[0] -eq $nodeIP -and $parts[1] -eq 'true' -and $parts[2] -eq '30443')
}

# Read-ClusterJson 显式保留只读 kubectl 探测的超时、退出失败和 JSON 解析错误。
function Read-ClusterJson {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)
    $result = Invoke-ProcessWithTimeout -FilePath "kubectl.exe" -Arguments (
        @("--context", $clusterContext, "--request-timeout=30s") + $Arguments + @("-o", "json")
    ) -TimeoutSeconds 30
    if ($result.TimedOut) { throw "kubectl 探测超时: $($Arguments -join ' ')" }
    if ($result.ExitCode -ne 0) { throw "kubectl 探测失败(exit=$($result.ExitCode)): $($result.Stderr.Trim())" }
    return ($result.Stdout | ConvertFrom-Json)
}

# New-WatcherHealth 为启动和每轮探测创建未知计数,不把尚未读取的数据报告为零。
function New-WatcherHealth {
    return [pscustomobject]@{
        docker_ready = $null; cluster_ready = $false; edge_ready = $null
        runtime_network_ready = $null; runtime_network_samples = @()
        runtime_recovery_required = $false
        application_ready = $null; https_ready = $null
        recovery_suspended = (Test-Path -LiteralPath $suspendPath)
        cep_total = $null; cep_expected = $null; cep_stale = $null
        cep_missing = $null; cep_not_ready = $null; cep_unknown = $null
        cep_samples = @(); failure_stage = 'initializing'; repair_reason = 'probe_pending'; error = $null
        unhealthy_since = $null
    }
}

# Get-WatcherHealth 读取本轮全部健康门禁,失败时保留阶段和已取得的真实计数。
function Get-WatcherHealth {
    $health = New-WatcherHealth
    if ($health.recovery_suspended) {
        $health.failure_stage = 'suspended'
        $health.repair_reason = 'operator_suspended'
        return $health
    }
    try {
        $health.failure_stage = 'docker'
        $health.docker_ready = Test-DockerReady
        if (-not $health.docker_ready) {
            $health.repair_reason = 'docker_unavailable'
            return $health
        }

        $health.failure_stage = 'nodes'
        $nodes = @( (Read-ClusterJson -Arguments @('get', 'nodes')).items )
        if ($nodes.Count -lt 3) {
            $health.repair_reason = 'nodes_unready'
            return $health
        }

        $health.failure_stage = 'node_runtime_network'
        $runtimeNetwork = Get-NodeRuntimeNetworkState -NodeNames @($nodes | ForEach-Object { [string]$_.metadata.name }) -TimeoutSeconds $commandSeconds
        $health.runtime_network_ready = $runtimeNetwork.Ready
        $health.runtime_network_samples = $runtimeNetwork.Samples
        $health.runtime_recovery_required = $runtimeNetwork.RecoverRuntime
        if (-not $runtimeNetwork.Ready) {
            $health.repair_reason = 'node_runtime_network_unavailable'
            $health.error = $runtimeNetwork.Error
            return $health
        }

        $health.failure_stage = 'nodes'
        if (@($nodes | Where-Object {
            @($_.status.conditions | Where-Object { $_.type -eq 'Ready' -and $_.status -eq 'True' }).Count -ne 1
        }).Count -gt 0) {
            $health.repair_reason = 'nodes_unready'
            return $health
        }

        $health.failure_stage = 'system_pods'
        $pods = @( (Read-ClusterJson -Arguments @('get', 'pods', '--all-namespaces')).items )
        $criticalPods = @($pods | Where-Object {
            $_.metadata.namespace -eq 'kube-system' -and -not $_.metadata.deletionTimestamp -and (
                $_.metadata.labels.'k8s-app' -in @('cilium', 'kube-dns', 'cilium-envoy') -or
                $_.metadata.labels.'io.cilium/app' -eq 'operator' -or $_.metadata.name -like 'cilium-operator-*'
            )
        })
        $missingComponents = @(@('cilium', 'kube-dns', 'cilium-envoy') | Where-Object {
            $component = $_
            @($criticalPods | Where-Object { $_.metadata.labels.'k8s-app' -eq $component }).Count -eq 0
        })
        $operators = @($criticalPods | Where-Object {
            $_.metadata.labels.'io.cilium/app' -eq 'operator' -or $_.metadata.name -like 'cilium-operator-*'
        })
        if ($missingComponents.Count -gt 0 -or $operators.Count -eq 0 -or @($criticalPods | Where-Object {
            $_.status.phase -ne 'Running' -or @($_.status.containerStatuses).Count -eq 0 -or
            @($_.status.containerStatuses | Where-Object { -not $_.ready }).Count -gt 0
        }).Count -gt 0) {
            $health.repair_reason = 'system_pods_unready'
            return $health
        }

        $health.failure_stage = 'cilium_endpoints'
        $endpoints = @( (Read-ClusterJson -Arguments @('get', 'ciliumendpoints', '--all-namespaces')).items )
        $cep = Get-CiliumEndpointState -Nodes $nodes -Pods $pods -Endpoints $endpoints
        $health.cep_total = $cep.Total
        $health.cep_expected = $cep.ExpectedPodKeys.Count
        $health.cep_stale = $cep.StaleEndpoints.Count
        $health.cep_missing = $cep.MissingKeys.Count
        $health.cep_not_ready = $cep.NotReadyKeys.Count
        $health.cep_unknown = $cep.UnknownKeys.Count
        $health.cep_samples = @((@($cep.StaleEndpoints | ForEach-Object {
            "$($_.metadata.namespace)/$($_.metadata.name)"
        }) + $cep.MissingKeys + $cep.NotReadyKeys + $cep.UnknownKeys) | Select-Object -Unique -First 5)
        if (-not $cep.Ready) {
            $health.repair_reason = 'cilium_endpoints_inconsistent'
            return $health
        }

        $health.failure_stage = 'external_edge'
        $health.edge_ready = Test-ExternalEdgeReady
        if (-not $health.edge_ready) {
            $health.repair_reason = 'external_edge_unready'
            return $health
        }
        $health.cluster_ready = $true
        $health.failure_stage = 'application'
        $deployments = @((Read-ClusterJson -Arguments @('-n', 'chaimir-system', 'get', 'deployments')).items)
        $unready = @(@('chaimir-backend', 'chaimir-frontend', 'chaimir-sandbox-rbac-provisioner') | Where-Object {
            $name = $_
            $deployment = @($deployments | Where-Object { $_.metadata.name -eq $name })
            $deployment.Count -ne 1 -or $deployment[0].spec.replicas -lt 1 -or
                $deployment[0].status.observedGeneration -lt $deployment[0].metadata.generation -or
                $deployment[0].status.updatedReplicas -ne $deployment[0].spec.replicas -or
                $deployment[0].status.availableReplicas -ne $deployment[0].spec.replicas
        })
        $health.application_ready = $unready.Count -eq 0
        if (-not $health.application_ready) {
            $health.repair_reason = 'application_unready'
            $health.error = '应用尚未就绪: ' + ($unready -join ',')
            return $health
        }
        $health.failure_stage = 'https'
        $https = Invoke-ProcessWithTimeout -FilePath 'curl.exe' -Arguments @(
            '--fail', '--silent', '--show-error', '--connect-timeout', [string]$commandSeconds, '--max-time', [string]$commandSeconds,
            '--cacert', (Join-Path $deployRoot 'config\chaimir-tls\tls.crt'), ($appOrigin.TrimEnd('/') + '/healthz')
        ) -TimeoutSeconds ($commandSeconds * 2)
        $health.https_ready = -not $https.TimedOut -and $https.ExitCode -eq 0
        if (-not $health.https_ready) {
            $health.repair_reason = 'https_unavailable'
            $health.error = $https.Stderr.Trim()
            return $health
        }
        $health.failure_stage = $null
        $health.repair_reason = $null
    } catch {
        if ($health.failure_stage -eq 'docker') { $health.docker_ready = $false }
        if ($health.failure_stage -eq 'node_runtime_network') { $health.runtime_network_ready = $false }
        $health.repair_reason = 'probe_failed'
        $health.error = $_.Exception.Message
    }
    return $health
}

# Write-WatcherState 原子保存独立门禁和进程心跳,防止读者把半写入或旧布尔值当作健康。
function Write-WatcherState {
    param([object]$Health, [bool]$RepairInProgress = $false, [object]$LastRepair = $null)
    $payload = [ordered]@{
        checked_at = (Get-Date).ToUniversalTime().ToString('o')
        process_id = $PID; script_sha256 = $scriptHash
        administrator = $isAdministrator
        repair_in_progress = $RepairInProgress; last_repair = $LastRepair
    }
    foreach ($property in $Health.PSObject.Properties) { $payload[$property.Name] = $property.Value }
    $temporaryPath = Join-Path $stateRoot ("watcher.state." + [guid]::NewGuid().ToString('N') + '.tmp')
    try {
        [System.IO.File]::WriteAllText($temporaryPath, ($payload | ConvertTo-Json -Depth 5 -Compress), (New-Object System.Text.UTF8Encoding($false)))
        if (Test-Path -LiteralPath $statePath) {
            [System.IO.File]::Replace($temporaryPath, $statePath, [System.Management.Automation.Language.NullString]::Value)
        } else {
            [System.IO.File]::Move($temporaryPath, $statePath)
        }
    } finally {
        if (Test-Path -LiteralPath $temporaryPath) { Remove-Item -LiteralPath $temporaryPath }
    }
}

# Restore-Environment 对持续节点执行/网络阻塞调用原Recover入口,其余故障先Ensure,再恢复既有集群。
function Restore-Environment {
    param([bool]$RecoverRuntime = $false)

    $runtimeAction = 'Ensure'
    if ($RecoverRuntime) { $runtimeAction = 'Recover' }
    Write-WatcherLog "健康门禁未通过,执行原Docker/WSL入口 action=$runtimeAction" | Out-Null
    $runtimeBudget = $startSeconds * 3 + $shutdownSeconds * 2 + $commandSeconds * 6
    $runtime = Invoke-ProcessWithTimeout -FilePath $powerShellPath -Arguments @(
        '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $PSScriptRoot 'docker-runtime.ps1'), '-Action', $runtimeAction
    ) -TimeoutSeconds $runtimeBudget
    if ($runtime.TimedOut -or $runtime.ExitCode -ne 0) {
        Write-WatcherLog "Docker/WSL恢复未完成: $($runtime.Stderr.Trim()) $($runtime.Stdout.Trim())" | Out-Null
        return [pscustomobject]@{ stage = 'docker'; exit_code = $runtime.ExitCode; timed_out = $runtime.TimedOut; completed_at = (Get-Date).ToUniversalTime().ToString('o') }
    }
    if (Test-Path -LiteralPath $suspendPath) {
        Write-WatcherLog '操作员已暂停自动恢复,本次不启动集群' | Out-Null
        return [pscustomobject]@{ stage = 'suspended'; exit_code = $null; timed_out = $false; completed_at = (Get-Date).ToUniversalTime().ToString('o') }
    }
    Write-WatcherLog "健康门禁未通过,开始执行正式 Cilium 集群恢复" | Out-Null
    $result = Invoke-ProcessWithTimeout -FilePath $powerShellPath -Arguments @(
        "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", (Join-Path $deployRoot "scripts\cilium-cluster.ps1"), "-Action", "Start"
    ) -TimeoutSeconds 900
    if ($result.TimedOut) {
        Write-WatcherLog "Cilium 恢复超时(900s),冷却后重新探测并按需重试" | Out-Null
    } elseif ($result.ExitCode -ne 0) {
        $detail = (($result.Stderr + " " + $result.Stdout).Trim() -replace "\s+", " ")
        Write-WatcherLog "Cilium 恢复失败(exit=$($result.ExitCode)): $detail" | Out-Null
    } else {
        Write-WatcherLog "Cilium 恢复进程完成,继续核对全部健康门禁" | Out-Null
    }
    return [pscustomobject]@{ stage = 'cluster'; exit_code = $result.ExitCode; timed_out = $result.TimedOut; completed_at = (Get-Date).ToUniversalTime().ToString('o') }
}

# Install-Watcher 受控更新当前用户单实例任务,补齐周期唤醒/失败重启并立即启动。
function Install-Watcher {
    $existing = Get-WatcherTask
    if ($existing -and $existing.State -eq 'Running') {
        $state = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
        if ($state.repair_in_progress -isnot [bool] -or $state.repair_in_progress -or
            ([DateTimeOffset]::UtcNow - [DateTimeOffset]::Parse([string]$state.checked_at)).TotalSeconds -gt $restartSeconds) {
            throw '现有监护正在恢复或状态未知,拒绝中断;请先核实当前恢复进程'
        }
        Stop-ScheduledTask -TaskName $taskName
        $deadline = (Get-Date).AddSeconds($restartSeconds)
        while ((Get-WatcherTask).State -eq 'Running') {
            if ((Get-Date) -ge $deadline) { throw '监护任务未在受控重载期限内停止' }
            Start-Sleep -Milliseconds 200
        }
    }
    $userId = "$env:USERDOMAIN\$env:USERNAME"
    $action = New-ScheduledTaskAction -Execute $powerShellPath -WorkingDirectory $deployRoot -Argument "-NoLogo -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$scriptPath`" -Action Watch"
    $interval = New-TimeSpan -Seconds $restartSeconds
    $triggers = @(
        (New-ScheduledTaskTrigger -AtLogOn -User $userId),
        (New-ScheduledTaskTrigger -Once -At (Get-Date) -RepetitionInterval $interval)
    )
    $principal = New-ScheduledTaskPrincipal -UserId $userId -LogonType Interactive -RunLevel Highest
    # 调度器级别4使用普通CPU/I/O优先级;默认级别7的低I/O会让模块加载和心跳长期滞后。
    $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew -RestartCount $restartCount -RestartInterval $interval -Priority 4
    Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $triggers -Principal $principal -Settings $settings -Force | Out-Null
    Start-ScheduledTask -TaskName $taskName
    Write-WatcherLog "已安装并启动当前用户最高可用权限任务 $taskName,周期/失败重启间隔${restartSeconds}s"
}

# Get-WatcherTask 只把明确的任务不存在视为缺失,其余 CIM/权限错误保留。
function Get-WatcherTask {
    $task = $null
    try {
        $task = Get-ScheduledTask -TaskName $taskName -ErrorAction Stop
    } catch {
        if ($_.Exception.HResult -ne -2147024894 -and $_.FullyQualifiedErrorId -notmatch "TaskNotFound|NoMatching") {
            throw
        }
    }
    return $task
}

# Uninstall-Watcher 仅卸载本项目当前用户任务,保留历史状态和日志。
function Uninstall-Watcher {
    $task = Get-WatcherTask
    if ($null -ne $task) {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
    }
    Write-WatcherLog "已卸载用户级计划任务 $taskName"
}

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$isAdministrator = ([Security.Principal.WindowsPrincipal]$identity).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if ($Action -in @('Install', 'Uninstall') -and -not $isAdministrator) {
    $elevated = Start-Process -FilePath $powerShellPath -Verb RunAs -WindowStyle Hidden -ArgumentList @(
        '-NoLogo', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', ('"' + $scriptPath + '"'), '-Action', $Action
    ) -PassThru -Wait
    if ($elevated.ExitCode -ne 0) { throw "管理员任务操作失败(exit=$($elevated.ExitCode))" }
    return
}
if ($Action -eq 'Suspend') {
    [IO.File]::WriteAllText($suspendPath, (Get-Date).ToUniversalTime().ToString('o'), (New-Object Text.UTF8Encoding($false)))
    Write-WatcherLog '已按操作员要求暂停自动恢复'
    return
}
if ($Action -eq 'Resume') {
    if (Test-Path -LiteralPath $suspendPath) { Remove-Item -LiteralPath $suspendPath }
    Write-WatcherLog '已按操作员要求恢复自动恢复'
    return
}
if ($Action -eq 'Watch' -and -not $isAdministrator) { throw '后台监护需要最高可用权限,请重新安装本项目任务' }
Write-WatcherLog "监护入口已加载,动作=$Action 进程=$PID"
Import-Module (Join-Path $repoRoot "images\lib\ImageMetadata.psm1") -Force
Import-Module (Join-Path $PSScriptRoot "lib\CiliumEndpointState.psm1") -Force
Write-WatcherLog "监护共享模块已加载,动作=$Action 进程=$PID"
$scriptHash = (Get-FileHash -LiteralPath $scriptPath -Algorithm SHA256).Hash.ToLowerInvariant()
if ($Action -eq "Uninstall") { Uninstall-Watcher; return }

$config = Read-ChaimirEnvFile -Path (Join-Path $deployRoot 'config\chaimir.env') -RequiredKeys @(
    'DOCKER_RUNTIME_WATCHER_POLL_SECONDS', 'DOCKER_RUNTIME_WATCHER_RESTART_INTERVAL_SECONDS', 'DOCKER_RUNTIME_WATCHER_RESTART_COUNT',
    'DOCKER_RUNTIME_COMMAND_TIMEOUT_SECONDS', 'DOCKER_RUNTIME_SHUTDOWN_TIMEOUT_SECONDS', 'DOCKER_RUNTIME_START_TIMEOUT_SECONDS', 'DOCKER_RUNTIME_APP_ORIGIN'
)
$pollSeconds = 0; $restartSeconds = 0; $restartCount = 0
if (-not [int]::TryParse($config.DOCKER_RUNTIME_WATCHER_POLL_SECONDS, [ref]$pollSeconds) -or $pollSeconds -lt 1 -or
    -not [int]::TryParse($config.DOCKER_RUNTIME_WATCHER_RESTART_INTERVAL_SECONDS, [ref]$restartSeconds) -or $restartSeconds -lt 60 -or
    -not [int]::TryParse($config.DOCKER_RUNTIME_WATCHER_RESTART_COUNT, [ref]$restartCount) -or $restartCount -lt 1 -or $restartCount -gt 999) {
    throw '监护配置无效:轮询须为正整数秒,重启间隔至少60秒,重启次数为1..999'
}
$appOrigin = [string]$config.DOCKER_RUNTIME_APP_ORIGIN
$appUri = $null
if (-not [Uri]::TryCreate($appOrigin, [UriKind]::Absolute, [ref]$appUri) -or $appUri.Scheme -ne 'https' -or
    $appUri.AbsolutePath -ne '/' -or $appUri.Query -or $appUri.Fragment -or $appUri.UserInfo) {
    throw '监护应用入口必须是无路径/凭据的HTTPS origin'
}
$commandSeconds = 0; $shutdownSeconds = 0; $startSeconds = 0
if (-not [int]::TryParse($config.DOCKER_RUNTIME_COMMAND_TIMEOUT_SECONDS, [ref]$commandSeconds) -or $commandSeconds -lt 10 -or
    -not [int]::TryParse($config.DOCKER_RUNTIME_SHUTDOWN_TIMEOUT_SECONDS, [ref]$shutdownSeconds) -or $shutdownSeconds -lt $commandSeconds -or
    -not [int]::TryParse($config.DOCKER_RUNTIME_START_TIMEOUT_SECONDS, [ref]$startSeconds) -or $startSeconds -lt $shutdownSeconds) {
    throw 'Docker恢复预算配置无效'
}
Write-WatcherLog "监护配置已读取,动作=$Action 轮询${pollSeconds}s"
if ($Action -eq "Install") {
    try { Install-Watcher } catch {
        Write-WatcherLog "安装失败: $($_.Exception.Message)"
        throw
    }
    return
}

$mutex = New-Object System.Threading.Mutex($false, "Global\Chaimir.DockerRuntimeWatcher")
$locked = $false
$exitCode = 0
try {
    try { $locked = $mutex.WaitOne(0) } catch [System.Threading.AbandonedMutexException] {
        $locked = $true
        Write-WatcherLog '上次监护异常退出,已接管互斥并重新探测'
    }
    if (-not $locked) {
        Write-WatcherLog '已有监护实例运行,本次不重复探测或恢复'
        if ($Action -eq 'RunOnce') { $exitCode = 1 }
    } else {
        Write-WatcherLog "监护已启动,进程=$PID 脚本=$scriptHash"
        Write-WatcherState -Health (New-WatcherHealth)
        $nextRecoveryAt = [datetime]::MinValue
        $unhealthySince = $null
        $lastRepair = $null
        $previousSignature = $null
        do {
            $health = Get-WatcherHealth
            if (-not $health.recovery_suspended -and -not $health.cluster_ready) {
                if ($null -eq $unhealthySince) { $unhealthySince = Get-Date }
                $health.unhealthy_since = $unhealthySince.ToUniversalTime().ToString('o')
            } else {
                $unhealthySince = $null
            }
            # 地址明确陈旧时立即恢复;其它失败先等待配置间隔,避免正常 Pod 网络收敛触发全局操作。
            $requiresRecovery = $health.cep_stale -gt 0 -or ($null -ne $unhealthySince -and ((Get-Date) - $unhealthySince).TotalSeconds -ge $restartSeconds)
            if (-not $health.recovery_suspended -and -not $health.cluster_ready -and $requiresRecovery -and
                (Get-Date) -ge $nextRecoveryAt -and -not (Test-Path -LiteralPath $suspendPath)) {
                Write-WatcherLog "恢复原因=$($health.repair_reason) 阶段=$($health.failure_stage) CEP样例=$($health.cep_samples -join ',') 错误=$($health.error)"
                Write-WatcherState -Health $health -RepairInProgress $true -LastRepair $lastRepair
                try {
                    $lastRepair = Restore-Environment -RecoverRuntime $health.runtime_recovery_required
                } catch {
                    Write-WatcherLog "恢复调用失败: $($_.Exception.Message)"
                    $lastRepair = [pscustomobject]@{ exit_code = $null; timed_out = $false; error = $_.Exception.Message; completed_at = (Get-Date).ToUniversalTime().ToString('o') }
                } finally {
                    $nextRecoveryAt = (Get-Date).AddSeconds($restartSeconds)
                    $health = Get-WatcherHealth
                }
            }
            Write-WatcherState -Health $health -LastRepair $lastRepair
            $signature = $health | ConvertTo-Json -Depth 3 -Compress
            if ($signature -ne $previousSignature) {
                Write-WatcherLog "探测结果: $signature"
                $previousSignature = $signature
            }
            if ($Action -eq "RunOnce") {
                if (-not $health.cluster_ready -or -not $health.application_ready -or -not $health.https_ready) { $exitCode = 1 }
                break
            }
            Start-Sleep -Seconds $pollSeconds
        } while ($true)
    }
} catch {
    Write-WatcherLog "监护异常退出,由任务调度器按配置重启: $($_.Exception.Message)"
    $exitCode = 1
} finally {
    if ($locked) { $mutex.ReleaseMutex() }
    $mutex.Dispose()
}
if ($exitCode -ne 0) { exit $exitCode }
