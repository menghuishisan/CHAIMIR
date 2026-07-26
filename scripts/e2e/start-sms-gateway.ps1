# start-sms-gateway.ps1 启动本地短信 mock 并在跑用例前完成链路探活。
# 网关进程自身从 deploy/config/secret.env 读取 SMS_HTTP_TOKEN（见 scripts/e2e/env.mjs），
# 与后端进程同源，因此本脚本不再向子进程注入任何短信相关变量。

[CmdletBinding()]
param(
    [string]$LogDir,
    [string]$BackendBaseUrl = "http://127.0.0.1:8080",
    [int]$BackendReadyTimeoutSeconds = 120
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$scriptDir = $PSScriptRoot
$repoRoot = Resolve-Path (Join-Path $scriptDir "../..")
if (-not $LogDir) {
    $LogDir = Join-Path $repoRoot ".tmp/e2e-sms"
}
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

$nodeExe = if ($env:E2E_NODE_EXE) { $env:E2E_NODE_EXE } else { "node" }
$outLog = Join-Path $LogDir "sms-gateway.out.log"
$errLog = Join-Path $LogDir "sms-gateway.err.log"

# 端口被占用时多半是上一轮残留的旧进程，它可能持有过期 token，必须让本轮显式失败。
$existing = Get-NetTCPConnection -LocalPort 18888 -State Listen -ErrorAction SilentlyContinue
if ($existing) {
    throw "端口 18888 已被进程 $($existing.OwningProcess) 占用；请先结束该进程，避免用例连到持有旧 token 的短信网关"
}

$gateway = Start-Process -FilePath $nodeExe `
    -ArgumentList (Join-Path $scriptDir "sms-gateway.mjs") `
    -WorkingDirectory $repoRoot `
    -RedirectStandardOutput $outLog -RedirectStandardError $errLog `
    -PassThru -WindowStyle Hidden
Set-Content -Path (Join-Path $LogDir "sms-gateway.pid") -Value $gateway.Id

# 等待网关监听就绪；进程提前退出说明配置加载失败，直接把它的错误日志抛出来。
$deadline = (Get-Date).AddSeconds(30)
while ($true) {
    if ($gateway.HasExited) {
        throw "短信网关启动失败（退出码 $($gateway.ExitCode)）：`n$(Get-Content -Raw -ErrorAction SilentlyContinue $errLog)"
    }
    if (Get-NetTCPConnection -LocalPort 18888 -State Listen -ErrorAction SilentlyContinue) { break }
    if ((Get-Date) -gt $deadline) { throw "短信网关 30 秒内未监听 18888 端口，详见 $errLog" }
    Start-Sleep -Milliseconds 200
}
Write-Host "短信网关已启动 (pid $($gateway.Id))，日志 $outLog"

# 探活依赖后端在线，先等待后端 readiness 再发真实验证码请求。
$deadline = (Get-Date).AddSeconds($BackendReadyTimeoutSeconds)
while ($true) {
    try {
        $ready = Invoke-WebRequest -Uri "$BackendBaseUrl/-/readyz" -UseBasicParsing -TimeoutSec 5
        if ($ready.StatusCode -eq 200) { break }
    } catch { }
    if ((Get-Date) -gt $deadline) {
        throw "后端 $BackendReadyTimeoutSeconds 秒内未就绪（$BackendBaseUrl/-/readyz），无法完成短信链路探活"
    }
    Start-Sleep -Seconds 2
}

$env:E2E_BACKEND_BASE_URL = $BackendBaseUrl
& $nodeExe (Join-Path $scriptDir "preflight-sms.mjs")
if ($LASTEXITCODE -ne 0) {
    throw "短信链路探活未通过，已终止本轮联调；请按上方提示修复后重跑"
}
