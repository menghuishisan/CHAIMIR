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

    $stdoutPath = Join-Path $evidenceRoot ([guid]::NewGuid().ToString() + ".stdout")
    $stderrPath = Join-Path $evidenceRoot ([guid]::NewGuid().ToString() + ".stderr")
    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = New-Object System.Diagnostics.ProcessStartInfo
    $process.StartInfo.FileName = $FilePath
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.CreateNoWindow = $true
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
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
        Remove-Item -LiteralPath $stdoutPath, $stderrPath -Force -ErrorAction SilentlyContinue
    }
}

# Write-Evidence 记录每次探测和恢复动作，便于区分 WSL 故障与 Kubernetes 数据面故障。
function Write-Evidence {
    param([Parameter(Mandatory = $true)][string]$Message)

    $line = "[$(Get-Date -Format o)] $Message"
    Add-Content -LiteralPath $logPath -Value $line -Encoding UTF8
    Write-Host $line
}

# Test-RuntimeProbe 验证 WSL 列表、Docker Desktop 发行版命令和 Docker API 均在有界时间内返回。
function Test-RuntimeProbe {
    $checks = @(
        @{ Name = "wsl-list"; File = "wsl.exe"; Args = @("-l", "-v", "--all"); Timeout = 60 },
        @{ Name = "docker-desktop-init"; File = "wsl.exe"; Args = @("-d", "docker-desktop", "-e", "/bin/true"); Timeout = 60 },
        @{ Name = "docker-api"; File = "docker.exe"; Args = @("info", "--format", "{{.ServerVersion}}"); Timeout = 30 }
    )
    foreach ($check in $checks) {
        $result = Invoke-ProcessWithTimeout -FilePath $check.File -Arguments $check.Args -TimeoutSeconds $check.Timeout
        if ($result.TimedOut) {
            Write-Evidence "$($check.Name) 超时: $($result.Stderr)"
            return $false
        }
        if ($result.ExitCode -ne 0) {
            $detail = (($result.Stderr + " " + $result.Stdout).Trim() -replace "\s+", " ")
            Write-Evidence "$($check.Name) 失败(exit=$($result.ExitCode)): $detail"
            return $false
        }
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

# Wait-RuntimeReady 等待 Docker Desktop 重启后的 API 与 WSL 发行版重新稳定。
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

# Recover-Runtime 有序停止 Docker Desktop、关闭卡死的 WSL VM，再重新启动 Docker Desktop；卷和镜像数据不被删除。
function Recover-Runtime {
    Write-Evidence "开始受控 Docker Desktop/WSL 恢复"
    $stop = Invoke-ProcessWithTimeout -FilePath "docker.exe" -Arguments @("desktop", "stop") -TimeoutSeconds 90
    if ($stop.TimedOut) {
        Write-Evidence "docker desktop stop 超时，继续请求 WSL 关闭"
    } elseif ($stop.ExitCode -ne 0) {
        $detail = (($stop.Stderr + " " + $stop.Stdout).Trim() -replace "\s+", " ")
        Write-Evidence "docker desktop stop 返回 exit=$($stop.ExitCode): $detail"
    }

    $shutdown = Invoke-ProcessWithTimeout -FilePath "wsl.exe" -Arguments @("--shutdown") -TimeoutSeconds 180
    if ($shutdown.TimedOut) {
        Write-Evidence "WSL 关闭超过 180s，终止 Docker 专用发行版后重试"
        foreach ($distribution in @("docker-desktop", "docker-desktop-data")) {
            $terminate = Invoke-ProcessWithTimeout -FilePath "wsl.exe" -Arguments @("--terminate", $distribution) -TimeoutSeconds 30
            if ($terminate.TimedOut) {
                Write-Evidence "WSL 发行版 $distribution 终止超时"
            } elseif ($terminate.ExitCode -ne 0) {
                $detail = (($terminate.Stderr + " " + $terminate.Stdout).Trim() -replace "\s+", " ")
                Write-Evidence "WSL 发行版 $distribution 终止返回 exit=$($terminate.ExitCode): $detail"
            } else {
                Write-Evidence "WSL 发行版 $distribution 已终止"
            }
        }
        $shutdown = Invoke-ProcessWithTimeout -FilePath "wsl.exe" -Arguments @("--shutdown") -TimeoutSeconds 90
    }
    if ($shutdown.TimedOut -or $shutdown.ExitCode -ne 0) {
        $detail = (($shutdown.Stderr + " " + $shutdown.Stdout).Trim() -replace "\s+", " ")
        throw "WSL 受控关闭失败: $detail"
    }

    $start = Invoke-ProcessWithTimeout -FilePath "docker.exe" -Arguments @("desktop", "start") -TimeoutSeconds 120
    if ($start.TimedOut -or $start.ExitCode -ne 0) {
        $detail = (($start.Stderr + " " + $start.Stdout).Trim() -replace "\s+", " ")
        throw "Docker Desktop 启动失败: $detail"
    }
    Wait-RuntimeReady -TimeoutSeconds 300
    Write-Evidence "Docker Desktop/WSL 恢复完成"
}

$runtimeMutex = New-Object System.Threading.Mutex($false, "Global\Chaimir.DockerRuntimeRecovery")
if (-not $runtimeMutex.WaitOne(0)) {
    throw "已有 Docker Desktop/WSL 恢复流程运行，拒绝并发操作"
}
try {
    if ($Action -eq "Recover") {
        Recover-Runtime
        return
    }

    $desktopState = Get-DockerDesktopState
    if ($desktopState -eq "starting") {
        Write-Evidence "Docker Desktop 正在启动，等待其完成 WSL/VHD 挂载"
        if (Wait-DockerDesktopStart -TimeoutSeconds 300 -and (Test-RuntimeProbe)) {
            Write-Evidence "Docker Desktop/WSL/Docker API 已就绪"
            return
        }
        Write-Evidence "Docker Desktop 启动等待未完成，进入受控恢复"
    } elseif (Test-RuntimeProbe) {
        Write-Evidence "Docker Desktop/WSL/Docker API 已就绪"
        return
    }

    Recover-Runtime
} finally {
    $runtimeMutex.ReleaseMutex()
    $runtimeMutex.Dispose()
}
