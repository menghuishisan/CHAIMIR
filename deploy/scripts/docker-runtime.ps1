# 本文件负责在项目部署入口前确认 Docker Desktop/WSL Linux engine 可响应，并在发行版无响应时执行受控恢复。
param(
    [ValidateSet("Ensure", "Recover")]
    [string]$Action = "Ensure"
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$evidenceRoot = Join-Path $repoRoot ".tmp\docker-runtime"
$logPath = Join-Path $evidenceRoot "recovery.log"
New-Item -ItemType Directory -Force -Path $evidenceRoot | Out-Null
Import-Module (Join-Path $repoRoot 'images\lib\ImageMetadata.psm1') -Force
$runtimeConfig = Read-ChaimirEnvFile -Path (Join-Path $repoRoot 'deploy\config\chaimir.env') -RequiredKeys @(
    'DOCKER_RUNTIME_COMMAND_TIMEOUT_SECONDS', 'DOCKER_RUNTIME_SHUTDOWN_TIMEOUT_SECONDS', 'DOCKER_RUNTIME_START_TIMEOUT_SECONDS'
)
$commandSeconds = 0; $shutdownSeconds = 0; $startSeconds = 0
if (-not [int]::TryParse($runtimeConfig.DOCKER_RUNTIME_COMMAND_TIMEOUT_SECONDS, [ref]$commandSeconds) -or $commandSeconds -lt 10 -or
    -not [int]::TryParse($runtimeConfig.DOCKER_RUNTIME_SHUTDOWN_TIMEOUT_SECONDS, [ref]$shutdownSeconds) -or $shutdownSeconds -lt $commandSeconds -or
    -not [int]::TryParse($runtimeConfig.DOCKER_RUNTIME_START_TIMEOUT_SECONDS, [ref]$startSeconds) -or $startSeconds -lt $shutdownSeconds) {
    throw 'Docker恢复配置无效:命令超时至少10秒,关闭预算不得小于命令预算,启动预算不得小于关闭预算'
}

# Stop-ProcessTree 在外部命令超时时终止整个进程树，避免 wsl.exe 子进程残留并阻塞后续恢复。
function Stop-ProcessTree {
    param([Parameter(Mandatory = $true)][Diagnostics.Process]$Process)

    try {
        & taskkill.exe /PID $Process.Id /T /F | Out-Null
    } catch {
        Write-Host "终止进程树失败，进程可能已退出或当前账户无权限: $($_.Exception.Message)"
    }
    try {
        if (-not $Process.HasExited) { $Process.Kill() }
    } catch {
        Write-Host "终止超时进程失败，进程可能已退出或当前账户无权限: $($_.Exception.Message)"
    }
}

# Invoke-ProcessWithTimeout 执行外部进程并在超过边界时终止本次探测，避免 WSL 卡死拖住部署入口。
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
    if ([IO.Path]::GetFileName($FilePath) -eq 'wsl.exe') {
        $process.StartInfo.StandardOutputEncoding = [Text.Encoding]::Unicode
        $process.StartInfo.StandardErrorEncoding = [Text.Encoding]::Unicode
    }
    # Windows PowerShell 5.1 没有 ProcessStartInfo.ArgumentList，使用受控的空格/引号编码。
    $process.StartInfo.Arguments = [string]::Join(" ", ($Arguments | ForEach-Object {
        if ($_ -match '[\s"]') {
            '"' + ($_ -replace '(\\*)"', '$1$1\"') + '"'
        } else {
            $_
        }
    }))

    try {
        [void]$process.Start()
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
            Stop-ProcessTree -Process $process
            return [pscustomobject]@{
                ExitCode = $null
                TimedOut = $true
                Stdout = ""
                Stderr = "进程在 ${TimeoutSeconds}s 内未退出"
            }
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

# Write-Evidence 记录每次探测和恢复动作，便于区分 WSL 故障与 Kubernetes 数据面故障。
function Write-Evidence {
    param([Parameter(Mandatory = $true)][string]$Message)

    $line = "[$(Get-Date -Format o)] $Message"
    Add-Content -LiteralPath $logPath -Value $line -Encoding UTF8
    Write-Host $line
}

# Test-RuntimeProbe 验证真实Linux engine API,内部WSL发行版不作为可用引擎的额外重启门禁。
function Test-RuntimeProbe {
    $result = Invoke-ProcessWithTimeout -FilePath 'docker.exe' -Arguments @('info','--format','{{.OSType}} {{.ServerVersion}}') -TimeoutSeconds $commandSeconds
    if ($result.TimedOut -or $result.ExitCode -ne 0) {
        $detail = (($result.Stderr + ' ' + $result.Stdout).Trim() -replace '\s+', ' ')
        Write-Evidence "docker-api失败(exit=$($result.ExitCode),timeout=$($result.TimedOut)): $detail"
        return $false
    }
    if ($result.Stdout.Trim() -notmatch '^linux\s+\S+$') {
        Write-Evidence 'Docker API未返回Linux engine,拒绝继续部署'
        return $false
    }
    return $true
}

# Get-DockerDesktopState 只读取 Docker Desktop 控制面的状态，不访问 WSL，避免正常启动阶段产生额外 WSL 请求。
function Get-DockerDesktopState {
    $result = Invoke-ProcessWithTimeout -FilePath "docker.exe" -Arguments @("desktop", "status") -TimeoutSeconds 10
    if ($result.TimedOut -or $result.ExitCode -ne 0) {
        return $null
    }
    $text = ($result.Stdout + "`n" + $result.Stderr)
    if ($text -match '(?im)^\s*Status\s+(\S+)\s*$') {
        return $matches[1].ToLowerInvariant()
    }
    return $null
}

# Wait-DockerDesktopStart 等待 Docker Desktop 自己完成启动，避免入口在 WSL/VHD 正常挂载期间主动停止它。
function Wait-DockerDesktopStart {
    param([int]$TimeoutSeconds = 300)

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $state = Get-DockerDesktopState
        if ($state -eq "running") {
            return $true
        }
        if ($state -in @("stopped", "error")) {
            return $false
        }
        Start-Sleep -Seconds 5
    } while ((Get-Date) -lt $deadline)
    return $false
}

# Wait-RuntimeReady 等待Docker Desktop重启后的Linux engine实际响应。
function Wait-RuntimeReady {
    param([int]$TimeoutSeconds = 180)

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        if (Test-RuntimeProbe) {
            return
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    throw "Docker Desktop/WSL 在 ${TimeoutSeconds}s 内未恢复可用"
}

# Restart-WslService 对已证实无响应的服务执行有界重启,停止超时只终止该服务当前的确切进程。
function Restart-WslService {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    if (-not ([Security.Principal.WindowsPrincipal]$identity).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw 'WSL服务恢复需要管理员权限;请安装具有最高可用权限的本项目监护任务'
    }
    Write-Evidence 'WSL关闭无响应,开始重启WSLService'
    Set-Service -Name WSLService -StartupType Automatic
    $service = Get-CimInstance Win32_Service -Filter "Name='WSLService'"
    if ($null -eq $service) { throw '未找到WSLService,拒绝操作其它服务' }
    if ($service.State -ne 'Stopped') {
        $stop = Invoke-ProcessWithTimeout -FilePath 'sc.exe' -Arguments @('stop', 'WSLService') -TimeoutSeconds $commandSeconds
        Write-Evidence "WSLService停止请求 exit=$($stop.ExitCode) timeout=$($stop.TimedOut)"
        $deadline = (Get-Date).AddSeconds($commandSeconds)
        do {
            $service = Get-CimInstance Win32_Service -Filter "Name='WSLService'"
            if ($service.State -eq 'Stopped') { break }
            Start-Sleep -Milliseconds 500
        } while ((Get-Date) -lt $deadline)
        if ($service.State -ne 'Stopped') {
            $serviceProcess = Get-CimInstance Win32_Process -Filter "ProcessId=$($service.ProcessId)"
            if ($service.ProcessId -le 0 -or $null -eq $serviceProcess -or $serviceProcess.Name -ne 'wslservice.exe' -or
                [string]::IsNullOrWhiteSpace($serviceProcess.ExecutablePath) -or
                $service.PathName.Trim('"') -ne $serviceProcess.ExecutablePath) {
                throw 'WSLService当前进程身份不匹配,拒绝终止'
            }
            Write-Evidence "WSLService停止超时,终止已核对的服务进程 pid=$($service.ProcessId) path=$($serviceProcess.ExecutablePath)"
            Stop-Process -Id $service.ProcessId -Force -ErrorAction Stop
            (Get-Service WSLService).WaitForStatus([ServiceProcess.ServiceControllerStatus]::Stopped, [TimeSpan]::FromSeconds($commandSeconds))
        }
    }
    $start = Invoke-ProcessWithTimeout -FilePath 'sc.exe' -Arguments @('start', 'WSLService') -TimeoutSeconds $commandSeconds
    if ($start.TimedOut -or $start.ExitCode -ne 0) { throw "WSLService启动失败: $($start.Stderr) $($start.Stdout)" }
    (Get-Service WSLService).WaitForStatus([ServiceProcess.ServiceControllerStatus]::Running, [TimeSpan]::FromSeconds($commandSeconds))
    Write-Evidence 'WSLService已重新启动,继续验证WSL关闭与Docker实际可用性'
}

# Start-Runtime 让Desktop完成正常启动并核对Linux engine,冷启动无需先重启服务。
function Start-Runtime {
    Write-Evidence '启动Docker Desktop并等待真实API可用'
    $start = Invoke-ProcessWithTimeout -FilePath 'docker.exe' -Arguments @('desktop', 'start', '--timeout', [string]$startSeconds) -TimeoutSeconds ($startSeconds + $commandSeconds)
    if ($start.TimedOut -or $start.ExitCode -ne 0) {
        throw "Docker Desktop启动失败: $($start.Stderr) $($start.Stdout)"
    }
    Wait-RuntimeReady -TimeoutSeconds $startSeconds
}

# Stop-UnresponsiveDesktop 只在官方停止未收敛时结束当前用户且路径已核对的Desktop进程。
function Stop-UnresponsiveDesktop {
    $dockerPath = (Get-Command docker.exe -ErrorAction Stop).Source
    $desktopRoot = [IO.Path]::GetFullPath((Join-Path (Split-Path -Parent $dockerPath) '..\..'))
    $allowedPaths = @(
        (Join-Path $desktopRoot 'Docker Desktop.exe'),
        (Join-Path $desktopRoot 'frontend\Docker Desktop.exe'),
        (Join-Path $desktopRoot 'resources\com.docker.backend.exe')
    )
    $currentSid = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    $targets = @()
    foreach ($candidate in @(Get-CimInstance Win32_Process -Filter "Name='Docker Desktop.exe' OR Name='com.docker.backend.exe'")) {
        $owner = Invoke-CimMethod -InputObject $candidate -MethodName GetOwnerSid
        if ($owner.ReturnValue -ne 0) { throw "无法核对Desktop进程所有者: $($candidate.ProcessId)" }
        if ($owner.Sid -ne $currentSid) { continue }
        if ($candidate.ExecutablePath -notin $allowedPaths) { throw "Desktop进程路径未核对,拒绝结束: $($candidate.ProcessId)" }
        $targets += $candidate
    }
    foreach ($target in @($targets | Sort-Object @{ Expression = { $_.Name -eq 'com.docker.backend.exe' } })) {
        $current = Get-CimInstance Win32_Process -Filter "ProcessId=$($target.ProcessId)"
        if ($null -eq $current) { continue }
        if ($current.CreationDate -ne $target.CreationDate -or $current.ExecutablePath -ne $target.ExecutablePath) {
            throw "Desktop进程身份变化,拒绝结束: $($target.ProcessId)"
        }
        Write-Evidence "官方停止未收敛,结束当前用户确切Desktop进程 pid=$($target.ProcessId) path=$($target.ExecutablePath)"
        Stop-Process -Id $target.ProcessId -Force -ErrorAction Stop
    }
}

# Stop-Desktop 等待官方停止真正完成,卡在stopping时按确切进程身份收敛,避免立即重启的竞态。
function Stop-Desktop {
    $stop = Invoke-ProcessWithTimeout -FilePath 'docker.exe' -Arguments @('desktop','stop','--force','--timeout',[string]$shutdownSeconds) -TimeoutSeconds ($shutdownSeconds + $commandSeconds)
    Write-Evidence "Desktop停止请求 exit=$($stop.ExitCode) timeout=$($stop.TimedOut)"
    $deadline = (Get-Date).AddSeconds($commandSeconds)
    do {
        $state = Get-DockerDesktopState
        if ($state -eq 'stopped') { return }
        if ($null -eq $state -and @(Get-CimInstance Win32_Process -Filter "Name='com.docker.backend.exe'").Count -eq 0) { return }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)
    Write-Evidence "Desktop停止未完成(state=$state),核对当前用户进程后收敛"
    Stop-UnresponsiveDesktop
}

# Recover-Runtime 统一停止Desktop、关闭WSL并按需重启无响应服务,不删除卷或镜像。
function Recover-Runtime {
    Write-Evidence "开始受控 Docker Desktop/WSL 恢复"
    Stop-Desktop

    $shutdown = Invoke-ProcessWithTimeout -FilePath "wsl.exe" -Arguments @("--shutdown") -TimeoutSeconds $shutdownSeconds
    if ($shutdown.TimedOut -or $shutdown.ExitCode -ne 0) {
        Restart-WslService
        $shutdown = Invoke-ProcessWithTimeout -FilePath "wsl.exe" -Arguments @("--shutdown") -TimeoutSeconds $shutdownSeconds
    }
    if ($shutdown.TimedOut -or $shutdown.ExitCode -ne 0) {
        $detail = (($shutdown.Stderr + " " + $shutdown.Stdout).Trim() -replace "\s+", " ")
        throw "WSL 受控关闭失败: $detail"
    }

    Start-Runtime
    Write-Evidence "Docker Desktop/WSL 恢复完成"
}

$runtimeMutex = New-Object System.Threading.Mutex($false, "Global\Chaimir.DockerRuntimeRecovery")
$runtimeLocked = $false
try { $runtimeLocked = $runtimeMutex.WaitOne(0) } catch [System.Threading.AbandonedMutexException] {
    $runtimeLocked = $true
    Write-Evidence '接管上次异常退出的运行时恢复互斥'
}
if (-not $runtimeLocked) {
    $runtimeMutex.Dispose()
    throw "已有 Docker Desktop/WSL 恢复流程运行，拒绝并发操作"
}
try {
    if ($Action -eq "Recover") {
        Recover-Runtime
        return
    }

    $desktopState = Get-DockerDesktopState
    if ($desktopState -in @('stopped', $null)) {
        try { Start-Runtime; return } catch { Write-Evidence "正常启动未完成: $($_.Exception.Message)" }
    } elseif ($desktopState -eq "starting") {
        Write-Evidence "Docker Desktop 正在启动，等待其完成 WSL/VHD 挂载"
        if ((Wait-DockerDesktopStart -TimeoutSeconds $startSeconds) -and (Test-RuntimeProbe)) {
            Write-Evidence "Docker Linux engine API 已就绪"
            return
        }
        Write-Evidence "Docker Desktop 启动等待未完成，进入受控恢复"
    } elseif ($desktopState -eq 'running' -and (Test-RuntimeProbe)) {
        Write-Evidence "Docker Linux engine API 已就绪"
        return
    } elseif ($desktopState -eq 'stopping') {
        Write-Evidence 'Docker Desktop停机未完成,进入受控恢复而不复用旧端口转发'
    }

    Recover-Runtime
} finally {
    $runtimeMutex.ReleaseMutex()
    $runtimeMutex.Dispose()
}
