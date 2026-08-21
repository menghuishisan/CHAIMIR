# run-acceptance-seed-job.ps1 在 acceptance 迁移完成后调度 canonical 验收夹具种子。
# 脚本只复制当前迁移 Job 的运行时配置和不可变镜像,业务数据仍由 cmd/migrate seed-acceptance 负责。

[CmdletBinding()]
param(
    [string]$Namespace = "chaimir-system",
    [string]$MigrationJob = "chaimir-migrate",
    [string]$SeedJob = "chaimir-seed-acceptance",
    [int]$TimeoutSeconds = 180
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-Kubectl {
    param([Parameter(Mandatory)][string[]]$Arguments)
    $output = & kubectl @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "kubectl 失败: kubectl $($Arguments -join ' ')"
    }
    return $output
}

Invoke-Kubectl @("-n", $Namespace, "wait", "--for=condition=complete", "job/$MigrationJob", "--timeout=${TimeoutSeconds}s") | Out-Host
Invoke-Kubectl @("-n", $Namespace, "delete", "job", $SeedJob, "--ignore-not-found=true") | Out-Host

# 从已成功的迁移 Job 复制完整 Pod 安全上下文、镜像拉取凭据、ConfigMap 和 Secret 引用,
# 只替换 Job 名称及 canonical 子命令,避免维护第二份部署配置。
$source = Invoke-Kubectl @("-n", $Namespace, "get", "job/$MigrationJob", "-o", "json") | ConvertFrom-Json
$source.metadata.name = $SeedJob
$source.metadata.labels."app.kubernetes.io/name" = $SeedJob
foreach ($property in @("uid", "resourceVersion", "creationTimestamp", "generation", "managedFields", "selfLink", "ownerReferences")) {
    if ($source.metadata.PSObject.Properties[$property]) {
        $source.metadata.PSObject.Properties.Remove($property)
    }
}
if ($source.metadata.PSObject.Properties["annotations"]) {
    $source.metadata.PSObject.Properties.Remove("annotations")
}

foreach ($property in @("selector", "completionMode", "manualSelector", "podReplacementPolicy", "suspend", "parallelism", "completions")) {
    if ($source.spec.PSObject.Properties[$property]) {
        $source.spec.PSObject.Properties.Remove($property)
    }
}
$source.spec.template.metadata.labels = @{
    "app.kubernetes.io/component" = "migration"
    "app.kubernetes.io/name"      = $SeedJob
    "app.kubernetes.io/part-of"   = "chaimir"
}
$source.spec.template.spec.containers[0].args = @("seed-acceptance")
if ($source.PSObject.Properties["status"]) {
    $source.PSObject.Properties.Remove("status")
}

$manifest = $source | ConvertTo-Json -Depth 100 -Compress
$manifest | & kubectl apply -f - | Out-Host
if ($LASTEXITCODE -ne 0) {
    throw "kubectl 失败: kubectl apply -f -"
}
Invoke-Kubectl @("-n", $Namespace, "wait", "--for=condition=complete", "job/$SeedJob", "--timeout=${TimeoutSeconds}s") | Out-Host
Invoke-Kubectl @("-n", $Namespace, "logs", "job/$SeedJob", "--all-containers=true", "--timestamps") | Out-Host
