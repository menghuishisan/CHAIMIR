# start-sms-gateway.ps1 创建 local-dev 的临时短信验收夹具并完成真实后端探活。
# 网关实现来自同目录 sms-gateway.mjs；本脚本只负责 Kubernetes 编排和临时端口转发。

[CmdletBinding()]
param(
    [string]$LogDir,
    [string]$Namespace = "chaimir-system",
    [int]$GatewayLocalPort = 18080,
    [int]$BackendLocalPort = 18081,
    [int]$BackendReadyTimeoutSeconds = 120,
    [switch]$Cleanup
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$scriptDir = $PSScriptRoot
$repoRoot = Resolve-Path (Join-Path $scriptDir "../..")
if (-not $LogDir) {
    $LogDir = Join-Path $repoRoot ".tmp/e2e-sms"
}
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

$label = "chaimir.io/e2e=sms-gateway"
$gatewayImage = "registry.chaimir.io/base/node-builder@sha256:2e2a8ce7847dd5a58a6f5c02c7d1e10201e9ea3ec7f8cd286caa166fb4426d76"
$gatewayManifest = Join-Path $LogDir "sms-gateway-resources.yaml"
$gatewayConfig = Join-Path $LogDir "sms-gateway-configmap.yaml"
$gatewayForwardLog = Join-Path $LogDir "sms-gateway-port-forward.log"
$backendForwardLog = Join-Path $LogDir "backend-port-forward.log"

function Invoke-Kubectl {
    param([string[]]$Arguments)
    & kubectl @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "kubectl 失败: kubectl $($Arguments -join ' ')"
    }
}

function Remove-Fixture {
    $pidFile = Join-Path $LogDir "port-forward.pids"
    if (Test-Path -LiteralPath $pidFile) {
        foreach ($rawPid in Get-Content -LiteralPath $pidFile) {
            $forwardPid = 0
            if ([int]::TryParse($rawPid.Trim(), [ref]$forwardPid)) {
                $process = Get-Process -Id $forwardPid -ErrorAction SilentlyContinue
                if ($process -and $process.ProcessName -eq "kubectl") {
                    Stop-Process -Id $forwardPid -Force
                }
            }
        }
        Remove-Item -LiteralPath $pidFile -Force
    }
    & kubectl -n $Namespace delete deployment,service,configmap,networkpolicy -l $label --ignore-not-found
    if ($LASTEXITCODE -ne 0) {
        throw "清理短信验收夹具失败"
    }
    foreach ($generatedFile in @($gatewayManifest, $gatewayConfig)) {
        if (Test-Path -LiteralPath $generatedFile) {
            Remove-Item -LiteralPath $generatedFile -Force
        }
    }
}

if ($Cleanup) {
    Remove-Fixture
    exit 0
}

# 删除同一标签的上轮残留，避免旧 token 或旧脚本继续接收请求。
Remove-Fixture

# ConfigMap 只包含测试网关代码；SMS_HTTP_TOKEN 由运行时 Secret 注入，不写入文件。
$backendDeployment = kubectl -n $Namespace get deployment/chaimir-backend -o json | ConvertFrom-Json
$secretName = ($backendDeployment.spec.template.spec.containers[0].envFrom |
    Where-Object { $_.PSObject.Properties.Name -contains "secretRef" -and $_.secretRef } |
    Select-Object -First 1).secretRef.name
if ([string]::IsNullOrWhiteSpace($secretName)) {
    throw "无法从 chaimir-backend Deployment 解析当前哈希 Secret 名称"
}
& kubectl -n $Namespace create configmap sms-gateway-script `
    --from-file="sms-gateway.mjs=$($scriptDir)\sms-gateway.mjs" `
    --dry-run=client -o yaml |
    kubectl label --local -f - $label -o yaml |
    Set-Content -Path $gatewayConfig -Encoding utf8
if ($LASTEXITCODE -ne 0) {
    throw "生成短信网关 ConfigMap 失败"
}

$resources = @"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sms-gateway
  namespace: $Namespace
  labels:
    chaimir.io/e2e: sms-gateway
spec:
  replicas: 1
  selector:
    matchLabels:
      chaimir.io/e2e: sms-gateway
  template:
    metadata:
      labels:
        chaimir.io/e2e: sms-gateway
        app.kubernetes.io/name: sms-gateway
    spec:
      automountServiceAccountToken: false
      imagePullSecrets:
        - name: chaimir-harbor-pull
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
        fsGroup: 1000
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: gateway
          image: $gatewayImage
          command: ["node", "/opt/chaimir/sms-gateway.mjs"]
          env:
            - name: SMS_GATEWAY_PORT
              value: "18080"
            - name: SMS_GATEWAY_LOG_PATH
              value: /tmp/sms/messages.ndjson
            - name: SMS_HTTP_TOKEN
              valueFrom:
                secretKeyRef:
                  name: $secretName
                  key: SMS_HTTP_TOKEN
          ports:
            - name: http
              containerPort: 18080
          readinessProbe:
            httpGet:
              path: /-/readyz
              port: http
            initialDelaySeconds: 1
            periodSeconds: 5
          livenessProbe:
            httpGet:
              path: /-/readyz
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
          volumeMounts:
            - name: script
              mountPath: /opt/chaimir/sms-gateway.mjs
              subPath: sms-gateway.mjs
              readOnly: true
            - name: tmp
              mountPath: /tmp
      volumes:
        - name: script
          configMap:
            name: sms-gateway-script
            defaultMode: 0444
        - name: tmp
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: sms-gateway
  namespace: $Namespace
  labels:
    chaimir.io/e2e: sms-gateway
spec:
  selector:
    chaimir.io/e2e: sms-gateway
  ports:
    - name: http
      port: 18080
      targetPort: http
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: sms-gateway-allow-backend
  namespace: $Namespace
  labels:
    chaimir.io/e2e: sms-gateway
spec:
  podSelector:
    matchLabels:
      chaimir.io/e2e: sms-gateway
  policyTypes: ["Ingress"]
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/component: control-plane
      ports:
        - protocol: TCP
          port: 18080
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: backend-allow-sms-gateway
  namespace: $Namespace
  labels:
    chaimir.io/e2e: sms-gateway
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: control-plane
  policyTypes: ["Egress"]
  egress:
    - to:
        - podSelector:
            matchLabels:
              chaimir.io/e2e: sms-gateway
      ports:
        - protocol: TCP
          port: 18080
"@
Set-Content -Path $gatewayManifest -Value $resources -Encoding utf8
Invoke-Kubectl @("apply", "-f", $gatewayConfig)
Invoke-Kubectl @("apply", "-f", $gatewayManifest)

kubectl -n $Namespace wait --for=condition=available deployment/sms-gateway --timeout=60s
if ($LASTEXITCODE -ne 0) { throw "短信验收网关 60 秒内未就绪" }

function Start-PortForward {
    param([string]$Resource, [int]$LocalPort, [int]$RemotePort, [string]$LogPath)
    $process = Start-Process -FilePath "kubectl" `
        -ArgumentList @("-n", $Namespace, "port-forward", $Resource, "$LocalPort`:$RemotePort", "--address", "127.0.0.1") `
        -RedirectStandardOutput "$LogPath.out.log" -RedirectStandardError "$LogPath.err.log" `
        -PassThru -WindowStyle Hidden
    return $process
}

$gatewayForward = Start-PortForward -Resource "svc/sms-gateway" -LocalPort $GatewayLocalPort -RemotePort 18080 -LogPath $gatewayForwardLog
$backendForward = Start-PortForward -Resource "svc/chaimir-backend" -LocalPort $BackendLocalPort -RemotePort 80 -LogPath $backendForwardLog
Set-Content -Path (Join-Path $LogDir "port-forward.pids") -Value "$($gatewayForward.Id)`n$($backendForward.Id)"

$deadline = (Get-Date).AddSeconds(30)
while ($true) {
    if ($gatewayForward.HasExited -or $backendForward.HasExited) {
        throw "短信或后端 port-forward 启动失败，详见 $LogDir"
    }
    $gatewayListening = Get-NetTCPConnection -LocalPort $GatewayLocalPort -State Listen -ErrorAction SilentlyContinue
    $backendListening = Get-NetTCPConnection -LocalPort $BackendLocalPort -State Listen -ErrorAction SilentlyContinue
    if ($gatewayListening -and $backendListening) { break }
    if ((Get-Date) -gt $deadline) { throw "port-forward 30 秒内未监听，详见 $LogDir" }
    Start-Sleep -Milliseconds 200
}

$deadline = (Get-Date).AddSeconds($BackendReadyTimeoutSeconds)
while ($true) {
    try {
        $ready = Invoke-WebRequest -Uri "http://127.0.0.1:$BackendLocalPort/-/readyz" -UseBasicParsing -TimeoutSec 5
        if ($ready.StatusCode -eq 200) { break }
    } catch { }
    if ((Get-Date) -gt $deadline) {
        throw "后端 $BackendReadyTimeoutSeconds 秒内未就绪，无法完成短信链路探活"
    }
    Start-Sleep -Seconds 2
}

$env:E2E_BACKEND_BASE_URL = "http://127.0.0.1:$BackendLocalPort"
$env:E2E_SMS_ENDPOINT = "http://127.0.0.1:$GatewayLocalPort/sms"
$nodeExe = if ($env:E2E_NODE_EXE) { $env:E2E_NODE_EXE } else { "node" }
& $nodeExe (Join-Path $scriptDir "preflight-sms.mjs")
if ($LASTEXITCODE -ne 0) {
    throw "短信链路探活未通过，已终止本轮联调；请按上方提示修复后重跑"
}
Write-Host "短信验收夹具已就绪，port-forward 进程与日志位于 $LogDir；E2E 完成后执行本脚本 -Cleanup 并结束 port-forward PID。"
