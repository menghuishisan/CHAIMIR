# 本文件在工作负载滚动完成后清理未被当前模板或 Pod 引用的 Kustomize 哈希配置。
param(
    [string]$Context = "kind-chaimir-cilium"
)

$ErrorActionPreference = "Stop"

# Add-Reference 记录命名空间、资源类型和名称组成的唯一配置引用。
function Add-Reference {
    param(
        [System.Collections.Generic.HashSet[string]]$References,
        [string]$Namespace,
        [string]$Kind,
        [string]$Name
    )
    if (-not [string]::IsNullOrWhiteSpace($Name)) {
        [void]$References.Add("$Namespace|$Kind|$Name")
    }
}

# Add-PodSpecReferences 收集容器环境变量和卷中的 ConfigMap/Secret 引用。
function Add-PodSpecReferences {
    param(
        [System.Collections.Generic.HashSet[string]]$References,
        [string]$Namespace,
        [object]$PodSpec
    )
    if ($null -eq $PodSpec) {
        return
    }
    foreach ($container in @($PodSpec.initContainers) + @($PodSpec.containers) + @($PodSpec.ephemeralContainers)) {
        foreach ($source in @($container.envFrom)) {
            Add-Reference -References $References -Namespace $Namespace -Kind "configmap" -Name $source.configMapRef.name
            Add-Reference -References $References -Namespace $Namespace -Kind "secret" -Name $source.secretRef.name
        }
        foreach ($entry in @($container.env)) {
            Add-Reference -References $References -Namespace $Namespace -Kind "configmap" -Name $entry.valueFrom.configMapKeyRef.name
            Add-Reference -References $References -Namespace $Namespace -Kind "secret" -Name $entry.valueFrom.secretKeyRef.name
        }
    }
    foreach ($volume in @($PodSpec.volumes)) {
        Add-Reference -References $References -Namespace $Namespace -Kind "configmap" -Name $volume.configMap.name
        Add-Reference -References $References -Namespace $Namespace -Kind "secret" -Name $volume.secret.secretName
        foreach ($source in @($volume.projected.sources)) {
            Add-Reference -References $References -Namespace $Namespace -Kind "configmap" -Name $source.configMap.name
            Add-Reference -References $References -Namespace $Namespace -Kind "secret" -Name $source.secret.name
        }
    }
}

# Get-CurrentReferences 汇总当前控制器模板和真实 Pod 的配置引用,避免删除滚动中的旧版本。
function Get-CurrentReferences {
    $raw = & kubectl --context $Context get deployment,statefulset,daemonset,job,cronjob,pod -A -o json
    if ($LASTEXITCODE -ne 0) {
        throw "读取工作负载配置引用失败"
    }
    $references = [System.Collections.Generic.HashSet[string]]::new()
    foreach ($item in ($raw | ConvertFrom-Json).items) {
        $podSpec = switch ($item.kind) {
            "CronJob" { $item.spec.jobTemplate.spec.template.spec }
            "Pod" { $item.spec }
            default { $item.spec.template.spec }
        }
        Add-PodSpecReferences -References $references -Namespace $item.metadata.namespace -PodSpec $podSpec
    }
    return $references
}

# Remove-UnreferencedGeneratedResources 只处理平台已知的哈希前缀,固定名称资源不在清理范围内。
function Remove-UnreferencedGeneratedResources {
    param([System.Collections.Generic.HashSet[string]]$References)

    $rules = @(
        @{ Namespace = "chaimir-system"; Kind = "configmap"; Prefix = "chaimir-config-" },
        @{ Namespace = "chaimir-system"; Kind = "configmap"; Prefix = "chaimir-frontend-runtime-config-" },
        @{ Namespace = "chaimir-system"; Kind = "secret"; Prefix = "chaimir-secret-" },
        @{ Namespace = "chaimir-data"; Kind = "configmap"; Prefix = "chaimir-data-config-" },
        @{ Namespace = "chaimir-data"; Kind = "secret"; Prefix = "chaimir-data-secret-" }
    )
    foreach ($rule in $rules) {
        $raw = & kubectl --context $Context -n $rule.Namespace get $rule.Kind -o json
        if ($LASTEXITCODE -ne 0) {
            throw "读取 $($rule.Namespace) 的 $($rule.Kind) 失败"
        }
        # 显式过滤空管道结果,避免 PowerShell 将空结果包装成一个空字符串并误触发保护门禁。
        $candidates = @((($raw | ConvertFrom-Json).items | Where-Object {
                    -not [string]::IsNullOrWhiteSpace($_.metadata.name) -and $_.metadata.name.StartsWith($rule.Prefix)
                }).metadata.name | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
        $active = @($candidates | Where-Object { $References.Contains("$($rule.Namespace)|$($rule.Kind)|$_") })
        if ($candidates.Count -gt 0 -and $active.Count -eq 0) {
            throw "$($rule.Namespace) 的 $($rule.Prefix) 未找到当前引用,拒绝清理"
        }
        foreach ($name in @($candidates | Where-Object { -not $References.Contains("$($rule.Namespace)|$($rule.Kind)|$_") })) {
            & kubectl --context $Context -n $rule.Namespace delete $rule.Kind $name
            if ($LASTEXITCODE -ne 0) {
                throw "删除未引用资源 $($rule.Namespace)/$name 失败"
            }
        }
    }
}

Remove-UnreferencedGeneratedResources -References (Get-CurrentReferences)
Write-Host "未引用的 Kustomize 哈希配置已清理。"
