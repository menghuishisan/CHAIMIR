# 本模块统一计算活动 Pod 的 CiliumEndpoint 就绪与当前节点地址归属,供监护和正式恢复共用。

# Get-CiliumEndpointState 只计算传入快照,不修改集群或信任可能陈旧的 Pod hostIP。
function Get-CiliumEndpointState {
    param(
        [Parameter(Mandatory = $true)][AllowEmptyCollection()][object[]]$Nodes,
        [Parameter(Mandatory = $true)][AllowEmptyCollection()][object[]]$Pods,
        [Parameter(Mandatory = $true)][AllowEmptyCollection()][object[]]$Endpoints
    )

    if ($Nodes.Count -eq 0) { throw "Node 快照为空,无法核对 CEP 节点归属" }
    $addressesByNode = @{}
    foreach ($node in $Nodes) {
        $addresses = @($node.status.addresses | Where-Object { $_.type -eq 'InternalIP' } | ForEach-Object { [string]$_.address })
        if ($addresses.Count -ne 1 -or [string]::IsNullOrWhiteSpace($addresses[0])) {
            throw "节点 $($node.metadata.name) 缺少唯一 InternalIP"
        }
        $addressesByNode[[string]$node.metadata.name] = $addresses[0]
    }
    $endpointsByKey = @{}
    foreach ($endpoint in $Endpoints) {
        $endpointsByKey["$($endpoint.metadata.namespace)/$($endpoint.metadata.name)"] = $endpoint
    }

    $expectedKeys = @()
    $healthyKeys = @()
    $staleEndpoints = @()
    $missingKeys = @()
    $notReadyKeys = @()
    $unknownKeys = @()
    foreach ($pod in $Pods) {
        # 仅已分配工作负载网络的活动 Pod 需要 CEP;正常调度/回收不触发全局恢复。
        if ($pod.spec.hostNetwork -or $pod.metadata.deletionTimestamp -or
            $pod.status.phase -in @('Succeeded', 'Failed') -or
            [string]::IsNullOrWhiteSpace([string]$pod.status.podIP)) {
            continue
        }
        $key = "$($pod.metadata.namespace)/$($pod.metadata.name)"
        $expectedKeys += $key
        $nodeName = [string]$pod.spec.nodeName
        if ([string]::IsNullOrWhiteSpace($nodeName) -or -not $addressesByNode.ContainsKey($nodeName)) {
            $unknownKeys += $key
            continue
        }
        $endpoint = $endpointsByKey[$key]
        if (-not $endpoint) {
            $missingKeys += $key
            continue
        }
        if ([string]::IsNullOrWhiteSpace([string]$endpoint.status.networking.node)) {
            $unknownKeys += $key
            continue
        }
        $addressMatches = $endpoint.status.networking.node -eq $addressesByNode[$nodeName]
        if (-not $addressMatches) { $staleEndpoints += $endpoint }
        if ($endpoint.status.state -ne 'ready') { $notReadyKeys += $key }
        if ($addressMatches -and $endpoint.status.state -eq 'ready') { $healthyKeys += $key }
    }

    return [pscustomobject]@{
        Ready = ($expectedKeys.Count -eq $healthyKeys.Count)
        Total = $Endpoints.Count
        ExpectedPodKeys = $expectedKeys
        HealthyPodKeys = $healthyKeys
        StaleEndpoints = $staleEndpoints
        MissingKeys = $missingKeys
        NotReadyKeys = $notReadyKeys
        UnknownKeys = $unknownKeys
    }
}

Export-ModuleMember -Function Get-CiliumEndpointState
