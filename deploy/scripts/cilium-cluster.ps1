# 本文件负责创建无默认 CNI 的 Kind 集群、安装正式 Cilium、执行网络数据面验收并维护外部 HTTPS 入口。
param(
    [ValidateSet("Up", "Check", "Edge", "Down")]
    [string]$Action = "Up",
    [string]$ClusterName = "chaimir-cilium",
    [string]$DependencyContext = "docker-desktop"
)

$ErrorActionPreference = "Stop"

$kindVersion = "v0.32.0"
$kindSha256 = "0bcb2d1cfedc1912d664014db716937e8a0e843e91c6807b4db2025dbc8989fa"
$kindDownloadUrl = "https://github.com/kubernetes-sigs/kind/releases/download/$kindVersion/kind-windows-amd64"
$nodeImage = "kindest/node:v1.36.1@sha256:3489c7674813ba5d8b1a9977baea8a6e553784dab7b84759d1014dbd78f7ebd5"
$ciliumChart = "oci://quay.io/cilium/charts/cilium@sha256:906ce40d35daad838d12add8a5ba7033e767767f51799a93c7eace2cec9cdc05"
$registryHost = "registry.chaimir.io"
$runtimeContext = "kind-$ClusterName"
$imagePullSecret = "chaimir-harbor-pull"

$deployRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$repoRoot = (Resolve-Path (Join-Path $deployRoot "..")).Path
$kindConfigPath = Join-Path $deployRoot "clusters\cilium-kind\kind.yaml"
$ciliumValuesPath = Join-Path $deployRoot "charts\cilium\values.yaml"
$trustedPolicyPath = Join-Path $deployRoot "components\cilium\trusted-control-plane-network-policies.yaml"
$healthPolicyPath = Join-Path $deployRoot "components\cilium\health-network-policy.yaml"
$ingressPolicyPath = Join-Path $deployRoot "components\cilium\ingress-control-plane-network-policy.yaml"
$edgeManifestPath = Join-Path $deployRoot "clusters\cilium-kind\external-edge-gateway.yaml"
$digestLockPath = Join-Path $repoRoot "images\image-digests.lock"
$registryCAPath = Join-Path $deployRoot "config\chaimir-tls\tls.crt"
$registryKeyPath = Join-Path $deployRoot "config\chaimir-tls\tls.key"
$dockerConfigPath = Join-Path $deployRoot "config\docker-auth\config.json"
$composePath = Join-Path $deployRoot "image-supply-chain.compose.yaml"
$composeEnvPath = Join-Path $deployRoot "config\chaimir.env"
$runtimeKubeconfigPath = Join-Path $deployRoot "config\supply-chain.kubeconfig"
$dependencyKubeconfigPath = Join-Path $deployRoot "config\harbor.kubeconfig"
$tmpRoot = Join-Path $repoRoot ".tmp\cilium-cluster"
$kindPath = Join-Path $repoRoot ".tmp\tools\kind\$kindVersion\kind.exe"

# Invoke-Native 执行外部命令并把非零退出码转换为明确错误。
function Invoke-Native {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "命令执行失败($LASTEXITCODE): $FilePath $($Arguments -join ' ')"
    }
}

# Write-Utf8NoBom 写入供 kubectl、containerd 消费的临时结构化文件,避免 Windows BOM 干扰解析。
function Write-Utf8NoBom {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Content
    )

    $encoding = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($Path, $Content, $encoding)
}

# Get-LockedDigest 从权威镜像锁读取唯一 digest,禁止在 Cilium values 中复制第二份真相源。
function Get-LockedDigest {
    param([Parameter(Mandatory = $true)][string]$LogicalImage)

    foreach ($line in Get-Content -LiteralPath $digestLockPath) {
        if ($line -match '^\s*([^#\s]+)\s+(sha256:[0-9a-f]{64})\s*$' -and $Matches[1] -eq $LogicalImage) {
            return $Matches[2]
        }
    }
    throw "权威镜像锁缺少 $LogicalImage"
}

# Get-Kind 确保使用项目固定并经过 SHA256 校验的 Kind 二进制。
function Get-Kind {
    $kindDirectory = Split-Path -Parent $kindPath
    New-Item -ItemType Directory -Force -Path $kindDirectory | Out-Null

    $valid = $false
    if (Test-Path -LiteralPath $kindPath) {
        $valid = (Get-FileHash -LiteralPath $kindPath -Algorithm SHA256).Hash.ToLowerInvariant() -eq $kindSha256
    }
    if (-not $valid) {
        $downloadPath = "$kindPath.download"
        Invoke-Native -FilePath "curl.exe" -Arguments @(
            "-fL", "--retry", "5", "--retry-delay", "2", "--retry-all-errors",
            "--connect-timeout", "30", "--max-time", "600", "--continue-at", "-",
            "--output", $downloadPath, $kindDownloadUrl
        )
        $actual = (Get-FileHash -LiteralPath $downloadPath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -ne $kindSha256) {
            Remove-Item -LiteralPath $downloadPath -Force
            throw "Kind SHA256 校验失败: expected=$kindSha256 actual=$actual"
        }
        Move-Item -LiteralPath $downloadPath -Destination $kindPath -Force
    }
    return $kindPath
}

# Assert-RequiredFiles 在改变集群前确认镜像认证、TLS 与固定配置完整。
function Assert-RequiredFiles {
    foreach ($path in @($kindConfigPath, $ciliumValuesPath, $trustedPolicyPath, $healthPolicyPath, $ingressPolicyPath, $edgeManifestPath, $digestLockPath, $registryCAPath, $registryKeyPath, $dockerConfigPath, $composePath, $composeEnvPath)) {
        if (-not (Test-Path -LiteralPath $path)) {
            throw "缺少 Cilium 集群前置文件: $path"
        }
    }
    Invoke-Native -FilePath "docker" -Arguments @("info", "--format", "{{.ServerVersion}}")
    Invoke-Native -FilePath "kubectl" -Arguments @("version", "--client")
}

# Assert-DependencyRegistry 确认迁移期外部 Harbor 可用;该集群不会继续承载 Chaimir 业务。
function Assert-DependencyRegistry {
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $DependencyContext, "-n", "harbor", "rollout", "status", "deployment/harbor-core", "--timeout=120s")
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $DependencyContext, "-n", "harbor", "rollout", "status", "deployment/harbor-registry", "--timeout=120s")

    $statusCode = & curl.exe -sS -o NUL -w "%{http_code}" "https://$registryHost/v2/"
    if ($LASTEXITCODE -ne 0 -or $statusCode -ne "401") {
        throw "外部 Harbor 健康检查失败: http=$statusCode"
    }
}

# Remove-LegacyWorkloads 删除旧 Kindnet 集群中的平台测试资源,只保留 Harbor 与外部入口。
function Remove-LegacyWorkloads {
    & kubectl --context $DependencyContext delete validatingwebhookconfiguration,mutatingwebhookconfiguration -l app.kubernetes.io/instance=policy-controller --ignore-not-found=true
    if ($LASTEXITCODE -ne 0) {
        throw "删除旧签名准入 webhook 失败"
    }
    & kubectl --context $DependencyContext delete clusterimagepolicy --all --ignore-not-found=true 2>$null
    if ($LASTEXITCODE -ne 0) {
        throw "删除旧签名准入策略失败"
    }
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $DependencyContext, "delete", "validatingadmissionpolicybinding", "-l", "app.kubernetes.io/part-of=chaimir", "--ignore-not-found=true")
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $DependencyContext, "delete", "validatingadmissionpolicy", "-l", "app.kubernetes.io/part-of=chaimir", "--ignore-not-found=true")
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $DependencyContext, "delete", "namespace", "chaimir-system", "chaimir-data", "chaimir-prepull", "cosign-system", "monitoring", "--ignore-not-found=true", "--wait=true", "--timeout=300s")

    $remaining = @(& kubectl --context $DependencyContext get namespace chaimir-system chaimir-data chaimir-prepull cosign-system monitoring --ignore-not-found -o name)
    if ($LASTEXITCODE -ne 0 -or $remaining.Count -gt 0) {
        throw "旧 Kindnet 集群仍残留 Chaimir 业务资源: $($remaining -join ',')"
    }
}

# Resume-ClusterNodes 固定 Kind 节点的 Docker 重启策略,并恢复仍存在但已停止的节点。
function Resume-ClusterNodes {
    $nodes = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$ClusterName" --format "{{.Names}}")
    if ($LASTEXITCODE -ne 0 -or $nodes.Count -eq 0) {
        throw "无法读取 Kind 集群 $ClusterName 的节点容器"
    }

    foreach ($node in $nodes) {
        Invoke-Native -FilePath "docker" -Arguments @("update", "--restart", "unless-stopped", $node)
        $running = [string](& docker inspect --format "{{.State.Running}}" $node)
        if ($LASTEXITCODE -ne 0) {
            throw "读取 Kind 节点 $node 状态失败"
        }
        if ($running.Trim() -ne "true") {
            Invoke-Native -FilePath "docker" -Arguments @("start", $node)
        }
    }
}

# Ensure-Cluster 创建唯一的无默认 CNI 集群;已有集群只复用,绝不静默重建并丢失数据。
function Ensure-Cluster {
    $kind = Get-Kind
    $clusters = @(& $kind get clusters)
    if ($LASTEXITCODE -ne 0) {
        throw "读取 Kind 集群失败"
    }
    if ($clusters -notcontains $ClusterName) {
        Invoke-Native -FilePath $kind -Arguments @("create", "cluster", "--name", $ClusterName, "--config", $kindConfigPath, "--image", $nodeImage)
    }
    Resume-ClusterNodes
    Invoke-Native -FilePath $kind -Arguments @("export", "kubeconfig", "--name", $ClusterName)
    Invoke-Native -FilePath "kubectl" -Arguments @("config", "use-context", $runtimeContext)
    Wait-KubernetesApi
}

# Wait-KubernetesApi 在节点运行时重启后等待控制面恢复,避免后续步骤与 API 重连竞争。
function Wait-KubernetesApi {
    $deadline = (Get-Date).AddSeconds(120)
    do {
        & kubectl --context $runtimeContext get --raw=/readyz 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            return
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)

    throw "Kubernetes API 在节点运行时重启后未恢复"
}

# Approve-KubeletServingCertificates 仅批准受控 Kind 节点提交且 SAN 匹配的 kubelet 服务证书。
function Approve-KubeletServingCertificates {
    $kind = Get-Kind
    $nodes = @(& $kind get nodes --name $ClusterName)
    if ($LASTEXITCODE -ne 0 -or $nodes.Count -eq 0) {
        throw "无法读取 kubelet 服务证书的目标节点"
    }

    New-Item -ItemType Directory -Force -Path $tmpRoot | Out-Null
    $controlPlane = "$ClusterName-control-plane"
    $approvedNodes = [System.Collections.Generic.HashSet[string]]::new()
    $deadline = (Get-Date).AddSeconds(180)
    do {
        $csrRaw = & kubectl --context $runtimeContext get certificatesigningrequests -o json
        if ($LASTEXITCODE -ne 0) {
            throw "读取 kubelet 服务证书请求失败"
        }
        $csrList = $csrRaw | ConvertFrom-Json
        foreach ($csr in $csrList.items) {
            if ($csr.spec.signerName -ne "kubernetes.io/kubelet-serving" -or $csr.status.conditions.Count -gt 0) {
                continue
            }

            $node = [string]$csr.spec.username -replace '^system:node:', ''
            if ($csr.spec.username -ne "system:node:$node" -or $nodes -notcontains $node -or $csr.spec.groups -notcontains "system:nodes") {
                throw "拒绝身份不匹配的 kubelet 服务证书请求: csr=$($csr.metadata.name) user=$($csr.spec.username)"
            }

            $nodeRaw = & kubectl --context $runtimeContext get node $node -o json
            if ($LASTEXITCODE -ne 0) {
                throw "读取 $node 失败"
            }
            $nodeState = $nodeRaw | ConvertFrom-Json
            $nodeIP = [string]($nodeState.status.addresses | Where-Object { $_.type -eq "InternalIP" } | Select-Object -First 1 -ExpandProperty address)
            if ($nodeIP -notmatch '^([0-9]{1,3}\.){3}[0-9]{1,3}$') {
                throw "无法读取 $node 的 InternalIP"
            }

            $csrPath = Join-Path $tmpRoot "$($csr.metadata.name).pem"
            [System.IO.File]::WriteAllBytes($csrPath, [Convert]::FromBase64String([string]$csr.spec.request))
            Invoke-Native -FilePath "docker" -Arguments @("cp", $csrPath, "${controlPlane}:/root/kubelet-serving.pem")
            $requestDetails = @(& docker exec $controlPlane openssl req -in /root/kubelet-serving.pem -noout -subject -text)
            if ($LASTEXITCODE -ne 0) {
                throw "解析 $node 的 kubelet 服务证书请求失败"
            }
            Invoke-Native -FilePath "docker" -Arguments @("exec", $controlPlane, "rm", "-f", "/root/kubelet-serving.pem")
            $details = $requestDetails -join "`n"
            if ($details -notmatch "CN\s*=\s*system:node:$([Regex]::Escape($node))" -or
                $details -notmatch 'O\s*=\s*system:nodes' -or
                $details -notmatch [Regex]::Escape("DNS:$node") -or
                $details -notmatch [Regex]::Escape("IP Address:$nodeIP")) {
                throw "拒绝 SAN 或主体不匹配的 kubelet 服务证书请求: csr=$($csr.metadata.name) node=$node ip=$nodeIP"
            }

            Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "certificate", "approve", $csr.metadata.name)
            [void]$approvedNodes.Add($node)
        }

        foreach ($node in $nodes) {
            & docker exec $node test -s /var/lib/kubelet/pki/kubelet-server-current.pem 2>$null
            if ($LASTEXITCODE -eq 0) {
                [void]$approvedNodes.Add($node)
            }
        }
        if ($approvedNodes.Count -eq $nodes.Count) {
            return
        }
        Start-Sleep -Seconds 2
    } while ((Get-Date) -lt $deadline)

    throw "kubelet 服务证书未覆盖全部节点: expected=$($nodes.Count) actual=$($approvedNodes.Count)"
}

# Configure-RegistryOnNodes 为每个 Kind 节点配置 canonical Harbor 的 split-DNS、CA 与 containerd hosts。
function Configure-RegistryOnNodes {
    $kind = Get-Kind
    $nodes = @(& $kind get nodes --name $ClusterName)
    if ($LASTEXITCODE -ne 0 -or $nodes.Count -eq 0) {
        throw "未找到 Kind 节点"
    }

    New-Item -ItemType Directory -Force -Path $tmpRoot | Out-Null
    $hostsPath = Join-Path $tmpRoot "registry-hosts.toml"
    $hostsContent = @"
server = "https://$registryHost"

[host."https://$registryHost"]
  capabilities = ["pull", "resolve"]
  ca = ["/etc/containerd/certs.d/$registryHost/ca.crt"]
"@
    Write-Utf8NoBom -Path $hostsPath -Content $hostsContent

    # Docker 会在 Kind 节点容器重启时重建 /etc/hosts,因此由 containerd 每次启动前恢复 canonical Harbor 映射。
    $registryHostsScriptPath = Join-Path $tmpRoot "chaimir-registry-hosts"
    $registryHostsScriptContent = @'
#!/bin/sh
set -eu

gateway="$(getent ahostsv4 host.docker.internal | awk 'NR == 1 { print $1 }')"
case "$gateway" in
    ''|*[!0-9.]*)
        echo "无法解析有效的 host.docker.internal IPv4 地址" >&2
        exit 1
        ;;
esac

temporary_hosts="$(mktemp)"
trap 'rm -f "$temporary_hosts"' EXIT
grep -vE '[[:space:]]__REGISTRY_HOST__([[:space:]]|$)' /etc/hosts >"$temporary_hosts" || true
printf '%s\t%s\n' "$gateway" '__REGISTRY_HOST__' >>"$temporary_hosts"
cat "$temporary_hosts" >/etc/hosts
'@
    Write-Utf8NoBom -Path $registryHostsScriptPath -Content ($registryHostsScriptContent.Replace("__REGISTRY_HOST__", $registryHost))

    foreach ($node in $nodes) {
        $gatewayLines = @(& docker exec $node getent ahostsv4 host.docker.internal)
        if ($LASTEXITCODE -ne 0 -or $gatewayLines.Count -eq 0) {
            throw "$node 无法解析 host.docker.internal"
        }
        $gateway = (($gatewayLines[0] -split '\s+')[0]).Trim()
        if ($gateway -notmatch '^([0-9]{1,3}\.){3}[0-9]{1,3}$') {
            throw "$node 返回了非法宿主机网关: $gateway"
        }

        Invoke-Native -FilePath "docker" -Arguments @("exec", $node, "mkdir", "-p", "/etc/containerd/certs.d/$registryHost")
        Invoke-Native -FilePath "docker" -Arguments @("cp", $registryCAPath, "${node}:/etc/containerd/certs.d/$registryHost/ca.crt")
        Invoke-Native -FilePath "docker" -Arguments @("cp", $hostsPath, "${node}:/etc/containerd/certs.d/$registryHost/hosts.toml")
        Invoke-Native -FilePath "docker" -Arguments @("cp", $registryHostsScriptPath, "${node}:/usr/local/sbin/chaimir-registry-hosts")
        Invoke-Native -FilePath "docker" -Arguments @("exec", $node, "chmod", "0755", "/usr/local/sbin/chaimir-registry-hosts")

        $nodeEnvironment = @(& docker inspect $node --format '{{range .Config.Env}}{{println .}}{{end}}')
        if ($LASTEXITCODE -ne 0) {
            throw "读取 $node 的代理环境失败"
        }
        $noProxyLine = [string]($nodeEnvironment | Where-Object { $_ -like "NO_PROXY=*" } | Select-Object -First 1)
        $noProxyEntries = [System.Collections.Generic.List[string]]::new()
        foreach ($entry in @((($noProxyLine -replace '^NO_PROXY=', '') -split ','))) {
            if (-not [string]::IsNullOrWhiteSpace($entry) -and -not $noProxyEntries.Contains($entry.Trim())) {
                $noProxyEntries.Add($entry.Trim())
            }
        }
        # canonical Harbor 由节点 split-DNS 直连宿主入口,不能经过宿主公网代理。
        foreach ($entry in @($registryHost, ".chaimir.io")) {
            if (-not $noProxyEntries.Contains($entry)) {
                $noProxyEntries.Add($entry)
            }
        }

        # Kind 节点继承的 127.0.0.1/localhost 代理指向节点自身,必须改写为 Docker 的宿主网关名称。
        # 非回环代理保持原值;未配置代理时让节点直接访问固定上游仓库。
        $serviceProxyValues = [ordered]@{}
        foreach ($proxyName in @("HTTP_PROXY", "HTTPS_PROXY")) {
            $proxyLine = [string]($nodeEnvironment | Where-Object { $_ -like "$proxyName=*" } | Select-Object -First 1)
            if ([string]::IsNullOrWhiteSpace($proxyLine)) {
                continue
            }
            $proxyValue = ($proxyLine -replace "^$proxyName=", '').Trim()
            if ([string]::IsNullOrWhiteSpace($proxyValue)) {
                continue
            }
            try {
                $proxyBuilder = [System.UriBuilder]::new($proxyValue)
            } catch {
                throw "$node 继承了非法 $proxyName 地址: $proxyValue"
            }
            if ($proxyBuilder.Host -in @("127.0.0.1", "localhost", "::1")) {
                $proxyBuilder.Host = "host.docker.internal"
                $proxyValue = $proxyBuilder.Uri.AbsoluteUri.TrimEnd('/')
            }
            $serviceProxyValues[$proxyName] = $proxyValue
        }

        $noProxy = $noProxyEntries -join ','
        $dropInPath = Join-Path $tmpRoot "${node}-containerd-registry.conf"
        $dropInLines = [System.Collections.Generic.List[string]]::new()
        $dropInLines.Add("[Service]")
        $dropInLines.Add("ExecStartPre=/usr/local/sbin/chaimir-registry-hosts")
        $dropInLines.Add("Environment=`"NO_PROXY=$noProxy`"")
        $dropInLines.Add("Environment=`"no_proxy=$noProxy`"")
        foreach ($proxyName in $serviceProxyValues.Keys) {
            $proxyValue = $serviceProxyValues[$proxyName]
            $dropInLines.Add("Environment=`"$proxyName=$proxyValue`"")
            $dropInLines.Add("Environment=`"$($proxyName.ToLowerInvariant())=$proxyValue`"")
        }
        $dropInContent = ($dropInLines -join "`n") + "`n"
        Write-Utf8NoBom -Path $dropInPath -Content $dropInContent
        Invoke-Native -FilePath "docker" -Arguments @("exec", $node, "mkdir", "-p", "/etc/systemd/system/containerd.service.d")
        Invoke-Native -FilePath "docker" -Arguments @("cp", $dropInPath, "${node}:/etc/systemd/system/containerd.service.d/chaimir-registry.conf")
        Invoke-Native -FilePath "docker" -Arguments @("exec", $node, "systemctl", "daemon-reload")
        Invoke-Native -FilePath "docker" -Arguments @("exec", $node, "systemctl", "restart", "containerd")

        $resolved = @(& docker exec $node getent ahostsv4 $registryHost)
        if ($LASTEXITCODE -ne 0 -or $resolved.Count -eq 0 -or (($resolved[0] -split '\s+')[0]).Trim() -ne $gateway) {
            throw "$node 的 $registryHost split-DNS 未指向宿主机网关"
        }
        $serviceEnvironment = @(& docker exec $node systemctl show containerd --property=Environment)
        if ($LASTEXITCODE -ne 0 -or ($serviceEnvironment -join "`n") -notmatch [Regex]::Escape($registryHost)) {
            throw "$node 的 containerd 未配置 Harbor 代理旁路"
        }
        if (($serviceEnvironment -join "`n") -match '(?i)(?:HTTP|HTTPS)_PROXY=https?://(?:127\.0\.0\.1|localhost|\[?::1\]?):') {
            throw "$node 的 containerd 仍引用节点回环代理"
        }
    }

    Wait-KubernetesApi
    foreach ($namespace in @("kube-system", "ingress-nginx", "cosign-system")) {
        & kubectl --context $runtimeContext get namespace $namespace 2>$null | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "create", "namespace", $namespace)
        }
        $pullSecretYaml = & kubectl --context $runtimeContext -n $namespace create secret generic $imagePullSecret --from-file=".dockerconfigjson=$dockerConfigPath" --type=kubernetes.io/dockerconfigjson --dry-run=client -o yaml
        if ($LASTEXITCODE -ne 0) {
            throw "生成 $namespace Harbor 拉取凭据失败"
        }
        $secretPath = Join-Path $tmpRoot "${namespace}-pull-secret.yaml"
        Write-Utf8NoBom -Path $secretPath -Content ($pullSecretYaml -join "`n")
        Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "apply", "-f", $secretPath)
    }
}

# Configure-RegistryPodDns 为集群 Pod 配置 canonical Harbor split-DNS,并用真实 Pod 解析验证结果。
function Configure-RegistryPodDns {
    $kind = Get-Kind
    $nodes = @(& $kind get nodes --name $ClusterName)
    if ($LASTEXITCODE -ne 0 -or $nodes.Count -eq 0) {
        throw "无法为 Pod DNS 读取 Kind 节点"
    }

    $gatewayLines = @(& docker exec $nodes[0] getent ahostsv4 host.docker.internal)
    if ($LASTEXITCODE -ne 0 -or $gatewayLines.Count -eq 0) {
        throw "$($nodes[0]) 无法解析 host.docker.internal"
    }
    $gateway = (($gatewayLines[0] -split '\s+')[0]).Trim()
    if ($gateway -notmatch '^([0-9]{1,3}\.){3}[0-9]{1,3}$') {
        throw "Pod DNS 收到非法宿主机网关: $gateway"
    }

    $corefile = (& kubectl --context $runtimeContext -n kube-system get configmap coredns -o 'jsonpath={.data.Corefile}') -join "`n"
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($corefile)) {
        throw "读取 CoreDNS Corefile 失败"
    }
    $managedPattern = '(?ms)^[ \t]*# BEGIN CHAIMIR REGISTRY SPLIT DNS\r?\n.*?^[ \t]*# END CHAIMIR REGISTRY SPLIT DNS\r?\n?'
    $withoutManagedBlock = [Regex]::Replace($corefile, $managedPattern, '')
    $managedBlock = @"
    # BEGIN CHAIMIR REGISTRY SPLIT DNS
    hosts {
        $gateway $registryHost
        fallthrough
    }
    # END CHAIMIR REGISTRY SPLIT DNS
"@
    $updatedCorefile = [Regex]::Replace(
        $withoutManagedBlock,
        '(?m)^(\.:53\s*\{)\s*$',
        { param($match) $match.Groups[1].Value + "`n" + $managedBlock },
        1
    )
    if ($updatedCorefile -eq $withoutManagedBlock) {
        throw "CoreDNS Corefile 缺少主 zone,无法写入 Harbor split-DNS"
    }

    if ($updatedCorefile -ne $corefile) {
        $patch = @{ data = @{ Corefile = $updatedCorefile } } | ConvertTo-Json -Compress
        $coreDnsPatchPath = Join-Path $tmpRoot "coredns-registry-patch.json"
        Write-Utf8NoBom -Path $coreDnsPatchPath -Content $patch
        Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", "kube-system", "patch", "configmap", "coredns", "--type=merge", "--patch-file", $coreDnsPatchPath)
        Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", "kube-system", "rollout", "restart", "deployment/coredns")
    }
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", "kube-system", "rollout", "status", "deployment/coredns", "--timeout=180s")

    $checkPod = "chaimir-registry-dns-check"
    $checkDigest = Get-LockedDigest -LogicalImage "base/chain-tools"
    $checkPodPath = Join-Path $tmpRoot "registry-dns-check.json"
    $checkPodManifest = [ordered]@{
        apiVersion = "v1"
        kind = "Pod"
        metadata = @{ name = $checkPod; namespace = "kube-system"; labels = @{ "app.kubernetes.io/part-of" = "chaimir" } }
        spec = [ordered]@{
            restartPolicy = "Never"
            automountServiceAccountToken = $false
            imagePullSecrets = @(@{ name = $imagePullSecret })
            securityContext = @{ runAsNonRoot = $true; seccompProfile = @{ type = "RuntimeDefault" } }
            containers = @([ordered]@{
                name = "dns-check"
                image = "$registryHost/base/chain-tools@$checkDigest"
                imagePullPolicy = "IfNotPresent"
                command = @("sh", "-c", "getent ahostsv4 $registryHost")
                resources = @{ requests = @{ cpu = "10m"; memory = "16Mi" }; limits = @{ cpu = "100m"; memory = "64Mi" } }
                securityContext = @{ allowPrivilegeEscalation = $false; readOnlyRootFilesystem = $true; capabilities = @{ drop = @("ALL") } }
            })
        }
    }
    Write-Utf8NoBom -Path $checkPodPath -Content ($checkPodManifest | ConvertTo-Json -Depth 20)
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", "kube-system", "delete", "pod", $checkPod, "--ignore-not-found=true", "--wait=true")
    try {
        Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "apply", "-f", $checkPodPath)
        Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", "kube-system", "wait", "--for=jsonpath={.status.phase}=Succeeded", "pod/$checkPod", "--timeout=120s")
        $dnsOutput = @(& kubectl --context $runtimeContext -n kube-system logs $checkPod)
        if ($LASTEXITCODE -ne 0 -or -not ($dnsOutput | Where-Object { $_ -match "^$([Regex]::Escape($gateway))\s" })) {
            throw "Pod 内 $registryHost 未解析到宿主机网关 $gateway"
        }
    } finally {
        Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", "kube-system", "delete", "pod", $checkPod, "--ignore-not-found=true", "--wait=true")
    }
}

# Write-ContainerKubeconfig 为容器化 Helm 生成指定集群的独立 kubeconfig,避免工具误用宿主机当前 context。
function Write-ContainerKubeconfig {
    param(
        [Parameter(Mandatory = $true)][string]$Context,
        [Parameter(Mandatory = $true)][string]$Path
    )

    $raw = & kubectl config view --raw --minify --context $Context -o json
    if ($LASTEXITCODE -ne 0) {
        throw "读取 $Context kubeconfig 失败"
    }
    $config = $raw | ConvertFrom-Json
    $server = [Uri]$config.clusters[0].cluster.server
    $config.clusters[0].cluster.server = "https://host.docker.internal:$($server.Port)"
    $config.clusters[0].cluster | Add-Member -NotePropertyName "tls-server-name" -NotePropertyValue "localhost" -Force
    Write-Utf8NoBom -Path $Path -Content ($config | ConvertTo-Json -Depth 100)
}

# Sync-ContainerKubeconfigs 固定运行集群与外部 Harbor 依赖的 Helm 目标,两者不得复用同一个隐含 context。
function Sync-ContainerKubeconfigs {
    Write-ContainerKubeconfig -Context $runtimeContext -Path $runtimeKubeconfigPath
    Write-ContainerKubeconfig -Context $DependencyContext -Path $dependencyKubeconfigPath
}

# Install-Cilium 使用固定 OCI Chart digest 和权威镜像锁安装 Cilium,随后等待数据面就绪。
function Install-Cilium {
    $agentDigest = Get-LockedDigest -LogicalImage "network/cilium"
    $operatorDigest = Get-LockedDigest -LogicalImage "network/cilium-operator"
    $envoyDigest = Get-LockedDigest -LogicalImage "network/cilium-envoy"

    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "apply", "-f", $trustedPolicyPath)
    $oldKubeconfig = [Environment]::GetEnvironmentVariable("SUPPLY_CHAIN_KUBECONFIG_HOST_PATH", "Process")
    [Environment]::SetEnvironmentVariable("SUPPLY_CHAIN_KUBECONFIG_HOST_PATH", $runtimeKubeconfigPath, "Process")
    try {
        Push-Location $deployRoot
        try {
            Invoke-Native -FilePath "docker" -Arguments @(
                "compose", "--project-name", "chaimir-supply-chain", "--env-file", $composeEnvPath, "-f", $composePath,
                "run", "--rm", "helm", "upgrade", "--install", "cilium", $ciliumChart,
                "--namespace", "kube-system", "--values", "/workspace/deploy/charts/cilium/values.yaml",
                "--set-string", "image.override=$registryHost/network/cilium@$agentDigest",
                "--set-string", "operator.image.override=$registryHost/network/cilium-operator@$operatorDigest",
                "--set-string", "envoy.image.override=$registryHost/network/cilium-envoy@$envoyDigest",
                "--set", "imagePullSecrets[0].name=$imagePullSecret",
                "--set-string", "cluster.name=$ClusterName", "--set", "cluster.id=1"
            )
        } finally {
            Pop-Location
        }
    } finally {
        [Environment]::SetEnvironmentVariable("SUPPLY_CHAIN_KUBECONFIG_HOST_PATH", $oldKubeconfig, "Process")
    }

    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", "kube-system", "rollout", "status", "daemonset/cilium", "--timeout=600s")
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", "kube-system", "rollout", "status", "daemonset/cilium-envoy", "--timeout=600s")
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", "kube-system", "rollout", "status", "deployment/cilium-operator", "--timeout=600s")
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "wait", "--for=condition=Ready", "nodes", "--all", "--timeout=600s")
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "apply", "-f", $healthPolicyPath)
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "apply", "-f", $trustedPolicyPath)
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "apply", "-f", $ingressPolicyPath)
}

# Repair-StaleCiliumEndpoints 在节点地址变化后删除失效的派生端点,并等待当前 Agent 重新发布。
function Repair-StaleCiliumEndpoints {
    $podsRaw = (& kubectl --context $runtimeContext get pods --all-namespaces -o json) -join "`n"
    if ($LASTEXITCODE -ne 0) {
        throw "读取 Pod 列表失败,无法核对 CiliumEndpoint 节点归属"
    }
    $endpointsRaw = (& kubectl --context $runtimeContext get ciliumendpoints --all-namespaces -o json) -join "`n"
    if ($LASTEXITCODE -ne 0) {
        throw "读取 CiliumEndpoint 列表失败"
    }

    $pods = ($podsRaw | ConvertFrom-Json).items
    $podByKey = @{}
    foreach ($pod in $pods) {
        $podByKey["$($pod.metadata.namespace)/$($pod.metadata.name)"] = $pod
    }

    $staleEndpoints = @()
    foreach ($endpoint in ($endpointsRaw | ConvertFrom-Json).items) {
        $key = "$($endpoint.metadata.namespace)/$($endpoint.metadata.name)"
        $pod = $podByKey[$key]
        if ($pod -and $pod.status.hostIP -and $endpoint.status.networking.node -and $endpoint.status.networking.node -ne $pod.status.hostIP) {
            $staleEndpoints += $endpoint
        }
    }
    if ($staleEndpoints.Count -eq 0) {
        return
    }

    foreach ($group in ($staleEndpoints | Group-Object { $_.metadata.namespace })) {
        $arguments = @(
            "--context", $runtimeContext, "-n", $group.Name, "delete", "ciliumendpoint", "--wait=true"
        ) + @($group.Group | ForEach-Object { $_.metadata.name })
        Invoke-Native -FilePath "kubectl" -Arguments $arguments
    }

    $targetKeys = @($staleEndpoints | ForEach-Object { "$($_.metadata.namespace)/$($_.metadata.name)" })
    $deadline = (Get-Date).AddSeconds(120)
    do {
        $currentPodsRaw = (& kubectl --context $runtimeContext get pods --all-namespaces -o json) -join "`n"
        if ($LASTEXITCODE -ne 0) {
            throw "等待 CiliumEndpoint 重建时读取 Pod 状态失败"
        }
        $currentEndpointsRaw = (& kubectl --context $runtimeContext get ciliumendpoints --all-namespaces -o json) -join "`n"
        if ($LASTEXITCODE -ne 0) {
            throw "等待 CiliumEndpoint 重建时读取端点状态失败"
        }
        $currentPodByKey = @{}
        foreach ($pod in (($currentPodsRaw | ConvertFrom-Json).items)) {
            $currentPodByKey["$($pod.metadata.namespace)/$($pod.metadata.name)"] = $pod
        }
        $currentEndpointByKey = @{}
        foreach ($endpoint in (($currentEndpointsRaw | ConvertFrom-Json).items)) {
            $currentEndpointByKey["$($endpoint.metadata.namespace)/$($endpoint.metadata.name)"] = $endpoint
        }

        $pending = @()
        foreach ($key in $targetKeys) {
            $pod = $currentPodByKey[$key]
            if (-not $pod) {
                continue
            }
            $endpoint = $currentEndpointByKey[$key]
            if (-not $endpoint -or $endpoint.status.state -ne "ready" -or $endpoint.status.networking.node -ne $pod.status.hostIP) {
                $pending += $key
            }
        }
        if ($pending.Count -eq 0) {
            Write-Output "已修复 $($staleEndpoints.Count) 个节点地址失效的 CiliumEndpoint"
            return
        }
        if ((Get-Date) -ge $deadline) {
            throw "CiliumEndpoint 未在节点地址变化后完成重建: $($pending -join ',')"
        }
        Start-Sleep -Seconds 2
    } while ($true)
}

# Remove-DisallowedNodeImages 删除 Kind node image 内预缓存但未部署的 Kindnet 镜像,避免遗留第二套 CNI 资产。
function Remove-DisallowedNodeImages {
    $kind = Get-Kind
    $nodes = @(& $kind get nodes --name $ClusterName)
    if ($LASTEXITCODE -ne 0 -or $nodes.Count -eq 0) {
        throw "无法读取待清理 Kindnet 缓存的节点"
    }

    foreach ($node in $nodes) {
        $images = @(& docker exec $node ctr --namespace k8s.io images list --quiet)
        if ($LASTEXITCODE -ne 0) {
            throw "读取 $node 的 containerd 镜像失败"
        }
        foreach ($image in @($images | Where-Object { $_ -match '(^|/)kindnetd([:@]|$)' })) {
            Invoke-Native -FilePath "docker" -Arguments @("exec", $node, "ctr", "--namespace", "k8s.io", "images", "remove", $image)
        }
    }
}

# Wait-CiliumHealth 等待所有节点的 Cilium 健康端点互通,避免只依据 Pod Ready 误判数据面可用。
function Wait-CiliumHealth {
    param([string[]]$Nodes)

    $deadline = (Get-Date).AddSeconds(180)
    $expected = "Cluster health:\s+$($Nodes.Count)/$($Nodes.Count) reachable"
    do {
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        $healthResult = @(& kubectl --context $runtimeContext -n kube-system exec daemonset/cilium -c cilium-agent -- cilium-health status --verbose 2>&1)
        $healthExitCode = $LASTEXITCODE
        $ErrorActionPreference = $previousErrorActionPreference
        $healthText = $healthResult -join "`n"
        if ($healthExitCode -eq 0 -and $healthText -match $expected) {
            return
        }
        if ((Get-Date) -ge $deadline) {
            throw "Cilium 节点健康探测未全部通过: $healthText"
        }
        Start-Sleep -Seconds 10
    } while ($true)
}

# Assert-CiliumState 校验 CNI 唯一性、策略模式、节点覆盖和正式镜像引用。
function Assert-CiliumState {
    $kind = Get-Kind
    $nodes = @(& $kind get nodes --name $ClusterName)
    if ($LASTEXITCODE -ne 0 -or $nodes.Count -eq 0) {
        throw "Cilium 集群不存在"
    }

    $kindnet = @(& kubectl --context $runtimeContext -n kube-system get daemonset kindnet --ignore-not-found -o name)
    if ($LASTEXITCODE -ne 0 -or $kindnet.Count -gt 0) {
        throw "运行集群仍存在 Kindnet"
    }

    $agent = (& kubectl --context $runtimeContext -n kube-system get daemonset cilium -o json) | ConvertFrom-Json
    $envoy = (& kubectl --context $runtimeContext -n kube-system get daemonset cilium-envoy -o json) | ConvertFrom-Json
    $operator = (& kubectl --context $runtimeContext -n kube-system get deployment cilium-operator -o json) | ConvertFrom-Json
    if ($agent.status.numberReady -ne $nodes.Count -or $envoy.status.numberReady -ne $nodes.Count -or $operator.status.availableReplicas -lt 1) {
        throw "Cilium 组件未覆盖全部节点: nodes=$($nodes.Count) agent=$($agent.status.numberReady) envoy=$($envoy.status.numberReady) operator=$($operator.status.availableReplicas)"
    }

    $config = (& kubectl --context $runtimeContext -n kube-system get configmap cilium-config -o json) | ConvertFrom-Json
    if ($config.data.'enable-policy' -ne "always" -or $config.data.'cni-exclusive' -ne "true") {
        throw "Cilium 策略配置不符合基线: enable-policy=$($config.data.'enable-policy') cni-exclusive=$($config.data.'cni-exclusive')"
    }

    $expectedAgent = "$registryHost/network/cilium@$(Get-LockedDigest -LogicalImage 'network/cilium')"
    $expectedOperator = "$registryHost/network/cilium-operator@$(Get-LockedDigest -LogicalImage 'network/cilium-operator')"
    $expectedEnvoy = "$registryHost/network/cilium-envoy@$(Get-LockedDigest -LogicalImage 'network/cilium-envoy')"
    $actualAgent = ($agent.spec.template.spec.containers | Where-Object { $_.name -eq "cilium-agent" }).image
    $actualOperator = ($operator.spec.template.spec.containers | Where-Object { $_.name -eq "cilium-operator" }).image
    $actualEnvoy = ($envoy.spec.template.spec.containers | Where-Object { $_.name -eq "cilium-envoy" }).image
    if ($actualAgent -ne $expectedAgent -or $actualOperator -ne $expectedOperator -or $actualEnvoy -ne $expectedEnvoy) {
        throw "Cilium 运行镜像与权威锁不一致"
    }

    foreach ($node in $nodes) {
        $files = @(& docker exec $node sh -c 'for file in /etc/cni/net.d/*; do [ -f "$file" ] && basename "$file"; done')
        if ($LASTEXITCODE -ne 0 -or $files.Count -ne 1 -or $files[0] -ne "05-cilium.conflist") {
            throw "$node 的 CNI 配置不是 Cilium 独占: $($files -join ',')"
        }
        $kindnetImages = @(& docker exec $node ctr --namespace k8s.io images list --quiet | Where-Object { $_ -match '(^|/)kindnetd([:@]|$)' })
        if ($LASTEXITCODE -ne 0 -or $kindnetImages.Count -gt 0) {
            throw "$node 仍残留 Kindnet 镜像: $($kindnetImages -join ',')"
        }
    }

    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", "kube-system", "exec", "daemonset/cilium", "-c", "cilium-agent", "--", "cilium-dbg", "status", "--brief")
    Wait-CiliumHealth -Nodes $nodes
}

# Invoke-NetworkPolicySmoke 通过真实 Pod 流量验证 ingress/egress 默认拒绝与精确放行。
function Invoke-NetworkPolicySmoke {
    $namespaces = @("cilium-smoke-server", "cilium-smoke-allowed", "cilium-smoke-denied", "cilium-smoke-egress")
    $serverImage = "$registryHost/service/frontend@$(Get-LockedDigest -LogicalImage 'service/frontend')"
    $clientImage = "$registryHost/base/chain-tools@$(Get-LockedDigest -LogicalImage 'base/chain-tools')"
    $resourcePath = Join-Path $tmpRoot "network-policy-smoke.yaml"
    $egressAllowPath = Join-Path $tmpRoot "network-policy-egress-allow.yaml"
    $smokeError = $null
    $cleanupFailures = @()
    New-Item -ItemType Directory -Force -Path $tmpRoot | Out-Null

    try {
        foreach ($namespace in $namespaces) {
            & kubectl --context $runtimeContext delete namespace $namespace --ignore-not-found=true --wait=true --timeout=120s | Out-Null
            if ($LASTEXITCODE -ne 0) {
                throw "清理旧 NetworkPolicy 测试命名空间失败: $namespace"
            }
            Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "create", "namespace", $namespace)
            Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "label", "namespace", $namespace, "pod-security.kubernetes.io/enforce=restricted", "pod-security.kubernetes.io/audit=restricted", "pod-security.kubernetes.io/warn=restricted", "--overwrite")
            Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", $namespace, "create", "secret", "generic", $imagePullSecret, "--from-file=.dockerconfigjson=$dockerConfigPath", "--type=kubernetes.io/dockerconfigjson")
        }
        Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "label", "namespace", "cilium-smoke-allowed", "chaimir.io/network-smoke=allowed", "--overwrite")
        Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "label", "namespace", "cilium-smoke-denied", "chaimir.io/network-smoke=denied", "--overwrite")
        Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "label", "namespace", "cilium-smoke-egress", "chaimir.io/network-smoke=egress", "--overwrite")

        $resources = @"
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
  namespace: cilium-smoke-server
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-approved-clients
  namespace: cilium-smoke-server
spec:
  podSelector:
    matchLabels:
      app: server
  policyTypes: [Ingress]
  ingress:
    - from:
        - namespaceSelector:
            matchExpressions:
              - key: chaimir.io/network-smoke
                operator: In
                values: [allowed, egress]
          podSelector:
            matchLabels:
              app: client
      ports:
        - protocol: TCP
          port: 8080
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-server-egress
  namespace: cilium-smoke-allowed
spec:
  podSelector:
    matchLabels:
      app: client
  policyTypes: [Egress]
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: cilium-smoke-server
          podSelector:
            matchLabels:
              app: server
      ports:
        - protocol: TCP
          port: 8080
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-server-egress
  namespace: cilium-smoke-denied
spec:
  podSelector:
    matchLabels:
      app: client
  policyTypes: [Egress]
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: cilium-smoke-server
          podSelector:
            matchLabels:
              app: server
      ports:
        - protocol: TCP
          port: 8080
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-egress
  namespace: cilium-smoke-egress
spec:
  podSelector: {}
  policyTypes: [Egress]
---
apiVersion: v1
kind: Service
metadata:
  name: server
  namespace: cilium-smoke-server
spec:
  selector:
    app: server
  ports:
    - name: http
      port: 8080
      targetPort: 8080
---
apiVersion: v1
kind: Pod
metadata:
  name: server
  namespace: cilium-smoke-server
  labels:
    app: server
spec:
  imagePullSecrets:
    - name: $imagePullSecret
  securityContext:
    runAsNonRoot: true
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: server
      image: $serverImage
      readinessProbe:
        httpGet:
          path: /
          port: 8080
        initialDelaySeconds: 1
        periodSeconds: 1
        timeoutSeconds: 3
        failureThreshold: 30
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: [ALL]
      volumeMounts:
        - name: tmp
          mountPath: /tmp
        - name: runtime
          mountPath: /var/run
  volumes:
    - name: tmp
      emptyDir: {}
    - name: runtime
      emptyDir: {}
---
apiVersion: v1
kind: Pod
metadata:
  name: client
  namespace: cilium-smoke-allowed
  labels:
    app: client
spec:
  imagePullSecrets:
    - name: $imagePullSecret
  securityContext:
    runAsNonRoot: true
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: client
      image: $clientImage
      command: [/bin/busybox, sleep, "3600"]
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: [ALL]
---
apiVersion: v1
kind: Pod
metadata:
  name: client
  namespace: cilium-smoke-denied
  labels:
    app: client
spec:
  imagePullSecrets:
    - name: $imagePullSecret
  securityContext:
    runAsNonRoot: true
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: client
      image: $clientImage
      command: [/bin/busybox, sleep, "3600"]
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: [ALL]
---
apiVersion: v1
kind: Pod
metadata:
  name: client
  namespace: cilium-smoke-egress
  labels:
    app: client
spec:
  imagePullSecrets:
    - name: $imagePullSecret
  securityContext:
    runAsNonRoot: true
    seccompProfile:
      type: RuntimeDefault
  containers:
    - name: client
      image: $clientImage
      command: [/bin/busybox, sleep, "3600"]
      securityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: true
        capabilities:
          drop: [ALL]
"@
        Write-Utf8NoBom -Path $resourcePath -Content $resources
        Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "apply", "-f", $resourcePath)
        foreach ($namespace in $namespaces) {
            Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", $namespace, "wait", "--for=condition=Ready", "pod", "--all", "--timeout=300s")
        }
        $serverIP = (& kubectl --context $runtimeContext -n cilium-smoke-server get service server -o jsonpath='{.spec.clusterIP}').Trim()
        if ($LASTEXITCODE -ne 0 -or $serverIP -notmatch '^([0-9]{1,3}\.){3}[0-9]{1,3}$') {
            throw "读取 NetworkPolicy 测试 Service IP 失败"
        }

        & kubectl --context $runtimeContext -n cilium-smoke-allowed exec client -- curl --connect-timeout 5 --max-time 5 -fsS "http://${serverIP}:8080/" | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "显式 ingress/egress 放行未生效"
        }
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        $deniedIngressResult = @(& kubectl --context $runtimeContext -n cilium-smoke-denied exec client -- curl --connect-timeout 3 --max-time 3 -fsS "http://${serverIP}:8080/" 2>$null)
        $deniedIngressExitCode = $LASTEXITCODE
        if ($deniedIngressExitCode -eq 0) {
            $ErrorActionPreference = $previousErrorActionPreference
            throw "未授权命名空间绕过了 ingress deny-all"
        }
        $deniedEgressResult = @(& kubectl --context $runtimeContext -n cilium-smoke-egress exec client -- curl --connect-timeout 3 --max-time 3 -fsS "http://${serverIP}:8080/" 2>$null)
        $deniedEgressExitCode = $LASTEXITCODE
        $ErrorActionPreference = $previousErrorActionPreference
        if ($deniedEgressExitCode -eq 0) {
            throw "客户端绕过了 egress deny-all"
        }

        $egressAllow = @"
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-server-egress
  namespace: cilium-smoke-egress
spec:
  podSelector:
    matchLabels:
      app: client
  policyTypes: [Egress]
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: cilium-smoke-server
          podSelector:
            matchLabels:
              app: server
      ports:
        - protocol: TCP
          port: 8080
"@
        Write-Utf8NoBom -Path $egressAllowPath -Content $egressAllow
        Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "apply", "-f", $egressAllowPath)
        Start-Sleep -Seconds 3
        & kubectl --context $runtimeContext -n cilium-smoke-egress exec client -- curl --connect-timeout 5 --max-time 5 -fsS "http://${serverIP}:8080/" | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "显式 egress 放行未生效"
        }
        Write-Output "Cilium NetworkPolicy 数据面验收通过"
    } catch {
        $smokeError = $_
    } finally {
        foreach ($namespace in $namespaces) {
            & kubectl --context $runtimeContext delete namespace $namespace --ignore-not-found=true --wait=true --timeout=180s | Out-Null
            if ($LASTEXITCODE -ne 0) {
                $cleanupFailures += $namespace
            }
        }
    }

    if ($null -ne $smokeError) {
        if ($cleanupFailures.Count -gt 0) {
            throw "Cilium NetworkPolicy 数据面验收失败，且临时命名空间清理失败: $($cleanupFailures -join ','); 原因: $($smokeError.Exception.Message)"
        }
        throw $smokeError
    }
    if ($cleanupFailures.Count -gt 0) {
        throw "Cilium NetworkPolicy 临时命名空间清理失败: $($cleanupFailures -join ',')"
    }
}

# Configure-ExternalEdge 让 canonical HTTPS 入口转发到 Cilium 集群固定 NodePort。
function Configure-ExternalEdge {
    New-Item -ItemType Directory -Force -Path $tmpRoot | Out-Null
    $servicePatchPath = Join-Path $tmpRoot "ingress-nodeport-patch.json"
    Write-Utf8NoBom -Path $servicePatchPath -Content @'
{"spec":{"type":"NodePort","externalTrafficPolicy":"Cluster","ports":[{"name":"http","port":80,"protocol":"TCP","targetPort":"http","nodePort":30080},{"name":"https","port":443,"protocol":"TCP","targetPort":"https","nodePort":30443}]}}
'@
    Invoke-Native -FilePath "kubectl" -Arguments @(
        "--context", $runtimeContext, "-n", "ingress-nginx", "patch", "service", "ingress-nginx-controller", "--type=merge",
        "--patch-file", $servicePatchPath
    )
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $runtimeContext, "-n", "ingress-nginx", "rollout", "status", "deployment/ingress-nginx-controller", "--timeout=180s")

    $controlPlane = "$ClusterName-control-plane"
    $networkJson = (& docker inspect $controlPlane --format '{{json .NetworkSettings.Networks}}' 2>$null).Trim()
    if ($LASTEXITCODE -ne 0 -or -not $networkJson) {
        throw "读取 Cilium 控制平面 Docker 地址失败"
    }
    try {
        $networkInfo = $networkJson | ConvertFrom-Json
        $nodeIP = [string]$networkInfo.kind.IPAddress
    } catch {
        throw "解析 Cilium 控制平面 Docker 地址失败: $($_.Exception.Message)"
    }
    if ($nodeIP -notmatch '^([0-9]{1,3}\.){3}[0-9]{1,3}$') {
        throw "读取 Cilium 控制平面 Docker 地址失败"
    }

    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $DependencyContext, "apply", "-f", $edgeManifestPath)
    $edgeSecretYaml = & kubectl --context $DependencyContext -n chaimir-edge create secret tls chaimir-edge-tls --cert=$registryCAPath --key=$registryKeyPath --dry-run=client -o yaml
    if ($LASTEXITCODE -ne 0) {
        throw "生成外部入口 TLS Secret 失败"
    }
    $edgeSecretPath = Join-Path $tmpRoot "edge-tls-secret.yaml"
    Write-Utf8NoBom -Path $edgeSecretPath -Content ($edgeSecretYaml -join "`n")
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $DependencyContext, "apply", "-f", $edgeSecretPath)

    $endpointSlice = @{
        apiVersion = "discovery.k8s.io/v1"
        kind = "EndpointSlice"
        metadata = @{
            name = "chaimir-cilium-ingress"
            namespace = "chaimir-edge"
            labels = @{ "kubernetes.io/service-name" = "chaimir-cilium-ingress" }
        }
        addressType = "IPv4"
        endpoints = @(@{ addresses = @($nodeIP); conditions = @{ ready = $true } })
        ports = @(@{ name = "https"; protocol = "TCP"; port = 30443 })
    }
    $endpointPath = Join-Path $tmpRoot "edge-endpoint-slice.json"
    Write-Utf8NoBom -Path $endpointPath -Content ($endpointSlice | ConvertTo-Json -Depth 20)
    Invoke-Native -FilePath "kubectl" -Arguments @("--context", $DependencyContext, "apply", "-f", $endpointPath)
}

# Remove-Cluster 只删除项目管理的 Kind 集群,外部 Harbor 不在删除范围内。
function Remove-Cluster {
    $kind = Get-Kind
    Invoke-Native -FilePath $kind -Arguments @("delete", "cluster", "--name", $ClusterName)
}

if ($Action -eq "Down") {
    Remove-Cluster
    return
}

Assert-RequiredFiles
switch ($Action) {
    "Up" {
        Assert-DependencyRegistry
        Remove-LegacyWorkloads
        Ensure-Cluster
        Sync-ContainerKubeconfigs
        Configure-RegistryOnNodes
        Approve-KubeletServingCertificates
        Install-Cilium
        Repair-StaleCiliumEndpoints
        Configure-RegistryPodDns
        Remove-DisallowedNodeImages
        Assert-CiliumState
        Invoke-NetworkPolicySmoke
    }
    "Check" {
        Sync-ContainerKubeconfigs
        Assert-CiliumState
        Invoke-NetworkPolicySmoke
    }
    "Edge" {
        Sync-ContainerKubeconfigs
        Assert-CiliumState
        Configure-ExternalEdge
    }
}
