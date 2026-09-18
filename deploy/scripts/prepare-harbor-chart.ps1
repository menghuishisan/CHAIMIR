# prepare-harbor-chart 校验并解包受控 Harbor chart，再从已签名的引导证据生成仅含 digest 的 values。
param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path,
    [string]$ConfigPath = "",
    [string]$EvidencePath = "",
    [string]$OutputDirectory = "",
    [int]$DockerTimeoutSeconds = 300
)

$ErrorActionPreference = "Stop"
if ($DockerTimeoutSeconds -lt 1) {
    throw "DockerTimeoutSeconds 必须大于等于 1"
}

# Resolve-RepoPath 将仓库相对路径限制在当前 Chaimir 工作区内。
function Resolve-RepoPath {
    param(
        [string]$BasePath,
        [string]$Path,
        [string]$Description
    )
    $resolved = if ([System.IO.Path]::IsPathRooted($Path)) {
        [System.IO.Path]::GetFullPath($Path)
    } else {
        [System.IO.Path]::GetFullPath((Join-Path $BasePath $Path))
    }
    if (-not $resolved.StartsWith($BasePath + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "$Description 必须位于仓库内: $Path"
    }
    return $resolved
}

# Read-EnvFile 使用统一严格解析器读取部署配置,不输出其中的敏感值。
function Read-EnvFile {
    param([string]$Path)
    return Read-ChaimirEnvFile -Path $Path
}

# Get-RequiredConfigValue 统一拒绝缺失或空白的供应链配置。
function Get-RequiredConfigValue {
    param(
        [hashtable]$Config,
        [string]$Key
    )
    $value = [string]$Config[$Key]
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "缺少 $Key"
    }
    return $value
}

# Assert-Sha256 验证给定文件与不可变 sha256 声明一致。
function Assert-Sha256 {
    param(
        [string]$Path,
        [string]$Expected,
        [string]$Description
    )
    if ($Expected -notmatch "^sha256:[0-9a-f]{64}$") {
        throw "$Description 的 sha256 非法: $Expected"
    }
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "缺少 ${Description}: $Path"
    }
    $actual = "sha256:" + (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $Expected) {
        throw "$Description 摘要不匹配: expected=$Expected actual=$actual"
    }
}

# Resolve-EvidenceFile 拒绝引导证据引用目录外的文件。
function Resolve-EvidenceFile {
    param(
        [string]$EvidenceDirectory,
        [string]$RelativePath,
        [string]$Description
    )
    if ([string]::IsNullOrWhiteSpace($RelativePath) -or [System.IO.Path]::IsPathRooted($RelativePath)) {
        throw "$Description 必须是证据目录内的相对路径"
    }
    $resolved = [System.IO.Path]::GetFullPath((Join-Path $EvidenceDirectory $RelativePath))
    if (-not $resolved.StartsWith($EvidenceDirectory + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "$Description 越出证据目录: $RelativePath"
    }
    return $resolved
}

# ConvertTo-ProcessArgument 将参数安全编码为 Windows 原生进程参数。
function ConvertTo-ProcessArgument([string]$Value) {
    if ($null -eq $Value -or $Value.Length -eq 0) { return '""' }
    if ($Value -notmatch '[\s"]') { return $Value }
    $builder = [Text.StringBuilder]::new()
    [void]$builder.Append('"')
    $slashes = 0
    foreach ($character in $Value.ToCharArray()) {
        if ($character -eq '\') { $slashes++; continue }
        if ($character -eq '"') {
            if ($slashes -gt 0) { [void]$builder.Append(('\' * ($slashes * 2)) -join '') }
            [void]$builder.Append('\').Append('"')
            $slashes = 0
            continue
        }
        if ($slashes -gt 0) { [void]$builder.Append(('\' * $slashes) -join '') }
        [void]$builder.Append($character)
        $slashes = 0
    }
    if ($slashes -gt 0) { [void]$builder.Append(('\' * ($slashes * 2)) -join '') }
    [void]$builder.Append('"')
    return $builder.ToString()
}

# Invoke-DockerBounded 以明确上限执行 Cosign,防止签名验证无限等待。
function Invoke-DockerBounded {
    param(
        [Parameter(Mandatory = $true)][string[]]$Arguments,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds
    )
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = [Diagnostics.ProcessStartInfo]::new()
    $process.StartInfo.FileName = "docker.exe"
    $process.StartInfo.Arguments = (($Arguments | ForEach-Object { ConvertTo-ProcessArgument ([string]$_) }) -join " ")
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.CreateNoWindow = $true
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    $process.StartInfo.StandardOutputEncoding = [Text.Encoding]::UTF8
    $process.StartInfo.StandardErrorEncoding = [Text.Encoding]::UTF8
    try {
        if (-not $process.Start()) { throw "Docker CLI 启动失败" }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
            try {
                & taskkill.exe /PID $process.Id /T /F | Out-Null
            } catch {
                Write-Warning ("Cosign 超时后的进程树终止失败: {0}" -f $_.Exception.Message)
            }
            try {
                $process.Kill($true)
            } catch {
                Write-Warning ("Cosign 超时后的进程终止失败: {0}" -f $_.Exception.Message)
            }
            $stdout = $stdoutTask.GetAwaiter().GetResult()
            $stderr = $stderrTask.GetAwaiter().GetResult()
            return [pscustomobject]@{ Code = 124; Text = ((@(("Docker CLI 超时（{0} 秒）: {1}" -f $TimeoutSeconds, ($Arguments -join " ")), $stdout, $stderr) | Where-Object { $_ }) -join "`n").Trim() }
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        return [pscustomobject]@{ Code = $process.ExitCode; Text = ((@($stdout, $stderr) | Where-Object { $_ }) -join "`n").Trim() }
    } finally {
        $process.Dispose()
    }
}

# Invoke-CosignVerifyBlob 用固定 Cosign 工具镜像在私有基础设施模式下验证不上传公共透明日志的签名。
function Invoke-CosignVerifyBlob {
    param(
        [string]$CosignImage,
        [string]$EvidenceDirectory,
        [string]$CosignDirectory,
        [string]$BlobPath,
        [string]$SignaturePath,
        [string]$Description
    )
    $signatureName = [System.IO.Path]::GetFileName($SignaturePath)
    $blobName = [System.IO.Path]::GetFileName($BlobPath)
    $arguments = @(
        "run", "--rm",
        "-v", "$EvidenceDirectory`:/evidence:ro",
        "-v", "$CosignDirectory`:/cosign:ro",
        "--entrypoint", "cosign",
        $CosignImage,
        "verify-blob", "--private-infrastructure", "--key", "/cosign/cosign.pub",
        "--signature", "/evidence/$signatureName", "/evidence/$blobName"
    )
    $result = Invoke-DockerBounded -Arguments $arguments -TimeoutSeconds $DockerTimeoutSeconds
    if ($result.Text) { Write-Output $result.Text }
    if ($result.Code -ne 0) {
        throw "Cosign 验证失败: $Description; $($result.Text)"
    }
}

# Get-TrivyFindingCount 只接受 Trivy JSON 中没有 HIGH/CRITICAL 漏洞的组件报告。
function Get-TrivyFindingCount {
    param(
        [string]$Path,
        [string]$Severity
    )
    $report = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
    $count = 0
    foreach ($result in @($report.Results)) {
        foreach ($vulnerability in @($result.Vulnerabilities)) {
            if ([string]::Equals([string]$vulnerability.Severity, $Severity, [System.StringComparison]::OrdinalIgnoreCase)) {
                $count++
            }
        }
    }
    return $count
}

# Add-ImageValues 写入 chart 所需的 repository@digest 覆盖项，禁止 chart 回退到 tag。
function Add-ImageValues {
    param(
        [System.Collections.Generic.List[string]]$Lines,
        [System.Collections.Generic.HashSet[string]]$DeclaredPaths,
        [string]$ValuesPath,
        [string]$Repository,
        [string]$Digest
    )
    $segments = $ValuesPath.Split(".")
    foreach ($index in 0..($segments.Count - 1)) {
        # 共享前缀(例如 registry.registry 与 registry.controller)只能声明一次;
        # 必须按完整路径去重，不能把不同父节点下的同名 image 误判为同一路径。
        $pathKey = ($segments[0..$index] -join ".")
        $pathLine = ("  " * $index) + "$($segments[$index]):"
        if ($DeclaredPaths.Add($pathKey)) {
            $Lines.Add($pathLine)
        }
    }
    $indent = "  " * $segments.Count
    $Lines.Add("${indent}repository: $Repository")
    $Lines.Add("${indent}digest: $Digest")
}

# Patch-HarborChart 仅替换已锁定官方 chart 的已知不安全渲染点，源文本变化即失败。
function Patch-HarborChart {
    param([string]$ChartDirectory)
    $helperPath = Join-Path $ChartDirectory "templates\_helpers.tpl"
    $helperText = Get-Content -LiteralPath $helperPath -Raw
    if ($helperText.Contains('define "harbor.imageRef"')) {
        throw "chart 已包含 harbor.imageRef，拒绝重复或未知补丁状态"
    }
    $helperText += @'

{{/* Render a verified immutable Harbor bootstrap image reference. */}}
{{- define "harbor.imageRef" -}}
{{- $repository := required "Harbor bootstrap image repository is required" .repository -}}
{{- $digest := required "Harbor bootstrap image digest is required" .digest -}}
{{- if not (regexMatch "^sha256:[0-9a-f]{64}$" $digest) -}}
{{- fail (printf "Harbor bootstrap image digest is invalid: %s" $digest) -}}
{{- end -}}
{{- printf "%s@%s" $repository $digest -}}
{{- end -}}
'@
    [System.IO.File]::WriteAllText($helperPath, $helperText, [System.Text.UTF8Encoding]::new($false))

    $replacements = @(
        @{ Path = "templates\core\core-dpl.yaml"; Source = "image: {{ .Values.core.image.repository }}:{{ .Values.core.image.tag }}"; Target = "image: {{ include `"harbor.imageRef`" (dict `"repository`" .Values.core.image.repository `"digest`" .Values.core.image.digest) }}" },
        @{ Path = "templates\core\core-pre-upgrade-job.yaml"; Source = "image: {{ .Values.core.image.repository }}:{{ .Values.core.image.tag }}"; Target = "image: {{ include `"harbor.imageRef`" (dict `"repository`" .Values.core.image.repository `"digest`" .Values.core.image.digest) }}" },
        @{ Path = "templates\jobservice\jobservice-dpl.yaml"; Source = "image: {{ .Values.jobservice.image.repository }}:{{ .Values.jobservice.image.tag }}"; Target = "image: {{ include `"harbor.imageRef`" (dict `"repository`" .Values.jobservice.image.repository `"digest`" .Values.jobservice.image.digest) }}" },
        @{ Path = "templates\portal\deployment.yaml"; Source = "image: {{ .Values.portal.image.repository }}:{{ .Values.portal.image.tag }}"; Target = "image: {{ include `"harbor.imageRef`" (dict `"repository`" .Values.portal.image.repository `"digest`" .Values.portal.image.digest) }}" },
        @{ Path = "templates\registry\registry-dpl.yaml"; Source = "image: {{ .Values.registry.registry.image.repository }}:{{ .Values.registry.registry.image.tag }}"; Target = "image: {{ include `"harbor.imageRef`" (dict `"repository`" .Values.registry.registry.image.repository `"digest`" .Values.registry.registry.image.digest) }}" },
        @{ Path = "templates\registry\registry-dpl.yaml"; Source = "image: {{ .Values.registry.controller.image.repository }}:{{ .Values.registry.controller.image.tag }}"; Target = "image: {{ include `"harbor.imageRef`" (dict `"repository`" .Values.registry.controller.image.repository `"digest`" .Values.registry.controller.image.digest) }}" },
        @{ Path = "templates\database\database-ss.yaml"; Source = "image: {{ .Values.database.internal.image.repository }}:{{ .Values.database.internal.image.tag }}"; Target = "image: {{ include `"harbor.imageRef`" (dict `"repository`" .Values.database.internal.image.repository `"digest`" .Values.database.internal.image.digest) }}"; ExpectedCount = 2 },
        @{ Path = "templates\redis\statefulset.yaml"; Source = "image: {{ .Values.redis.internal.image.repository }}:{{ .Values.redis.internal.image.tag }}"; Target = "image: {{ include `"harbor.imageRef`" (dict `"repository`" .Values.redis.internal.image.repository `"digest`" .Values.redis.internal.image.digest) }}" },
        @{ Path = "templates\trivy\trivy-sts.yaml"; Source = "image: {{ .Values.trivy.image.repository }}:{{ .Values.trivy.image.tag }}"; Target = "image: {{ include `"harbor.imageRef`" (dict `"repository`" .Values.trivy.image.repository `"digest`" .Values.trivy.image.digest) }}" }
    )
    foreach ($replacement in $replacements) {
        $path = Join-Path $ChartDirectory $replacement.Path
        $text = Get-Content -LiteralPath $path -Raw
        $count = [regex]::Matches($text, [regex]::Escape($replacement.Source)).Count
        # 一个组件可能同时出现在 initContainer 与主容器；必须覆盖模板中的全部预期引用。
        $expectedCount = if ($replacement.ContainsKey("ExpectedCount")) { [int]$replacement.ExpectedCount } else { 1 }
        if ($count -ne $expectedCount) {
            throw "chart 补丁目标数量错误: $($replacement.Path) -> expected=$expectedCount actual=$count"
        }
        $text = $text.Replace($replacement.Source, $replacement.Target)
        [System.IO.File]::WriteAllText($path, $text, [System.Text.UTF8Encoding]::new($false))
    }

    # Harbor registry 的 canonical 入口同时服务 Docker/OCI 客户端和 Harbor core。
    # 官方模板的空地址在当前 Docker Desktop 内核下可能只创建 IPv6 listener，
    # 使 IPv4 Service/Ingress 请求收到 connection refused；显式绑定 IPv4，
    # 不改变端口、TLS 终止或 registry 数据路径。
    $registryConfigPath = Join-Path $ChartDirectory "templates\registry\registry-cm.yaml"
    $registryConfigText = Get-Content -LiteralPath $registryConfigPath -Raw
    $registryAddrSource = '      addr: :{{ template "harbor.registry.containerPort" . }}'
    $registryAddrTarget = '      addr: 0.0.0.0:{{ template "harbor.registry.containerPort" . }}'
    if ([regex]::Matches($registryConfigText, [regex]::Escape($registryAddrSource)).Count -ne 1) {
        throw "registry IPv4 绑定补丁目标数量错误"
    }
    $registryConfigText = $registryConfigText.Replace($registryAddrSource, $registryAddrTarget)
    [System.IO.File]::WriteAllText($registryConfigPath, $registryConfigText, [System.Text.UTF8Encoding]::new($false))

    $databasePath = Join-Path $ChartDirectory "templates\database\database-ss.yaml"
    $databaseText = Get-Content -LiteralPath $databasePath -Raw
    $safeCommand = 'args: ["-ec", "mkdir -p /var/lib/postgresql/data/pgdata; chmod -R 700 /var/lib/postgresql/data/pgdata"]'
    $unsafePattern = 'args: \["-c", "chmod -R 700 /var/lib/postgresql/data/pgdata \|\| true"\]'
    if ([regex]::Matches($databaseText, $unsafePattern).Count -ne 1) {
        throw "chart 数据库权限补丁目标数量错误"
    }
    $databaseText = $databaseText -replace $unsafePattern, $safeCommand
    # Harbor DB 镜像把版本号放在 Entrypoint 参数中，普通 Container.Args 无法传入 postgres 配置。
    # 包装器仍执行官方入口；在 initdb 创建配置后写入一次集群内监听配置，再等待原进程接管。
    $databaseArgsSource = "        {{- end }}`n        livenessProbe:"
    $databaseArgsTarget = @'
        {{- end }}
        command: ["/bin/bash", "-ec"]
        args:
          - |
            set -eu
            /docker-entrypoint.sh 15 18 &
            entrypoint_pid=$!
            config_path="${PGDATA}/pg18/postgresql.conf"
            while [ ! -f "$config_path" ]; do
              if ! kill -0 "$entrypoint_pid" 2>/dev/null; then
                wait "$entrypoint_pid"
                exit $?
              fi
              sleep 1
            done
            if ! grep -qx "listen_addresses = '*'" "$config_path"; then
              printf "\nlisten_addresses = '*'\n" >> "$config_path"
            fi
            wait "$entrypoint_pid"
        livenessProbe:
'@.TrimEnd("`r", "`n")
    if ([regex]::Matches($databaseText, [regex]::Escape($databaseArgsSource)).Count -ne 1) {
        throw "chart 数据库监听参数补丁目标数量错误"
    }
    $databaseText = $databaseText.Replace($databaseArgsSource, $databaseArgsTarget)
    $databaseEnvSource = "          - name: PGDATA`n            value: `"/var/lib/postgresql/data/pgdata`"`n{{- with .Values.database.internal.extraEnvVars }}"
    $databaseEnvTarget = @'
          - name: PGDATA
            value: "/var/lib/postgresql/data/pgdata"
          - name: POSTGRES_INITDB_ARGS
            value: "--pwfile=/run/harbor-db-password/POSTGRES_PASSWORD --auth-local=scram-sha-256 --auth-host=scram-sha-256"
          - name: PGPASSWORD
            valueFrom:
              secretKeyRef:
                name: "{{ template "harbor.database" . }}"
                key: POSTGRES_PASSWORD
{{- with .Values.database.internal.extraEnvVars }}
'@.TrimEnd("`r", "`n")
    if ([regex]::Matches($databaseText, [regex]::Escape($databaseEnvSource)).Count -ne 1) {
        throw "chart 数据库 SCRAM 环境补丁目标数量错误"
    }
    $databaseText = $databaseText.Replace($databaseEnvSource, $databaseEnvTarget)
    $databaseMountSource = "        - name: shm-volume`n          mountPath: /dev/shm`n      volumes:"
    $databaseMountTarget = @'
        - name: shm-volume
          mountPath: /dev/shm
        - name: database-password
          mountPath: /run/harbor-db-password
          readOnly: true
      volumes:
'@.TrimEnd("`r", "`n")
    if ([regex]::Matches($databaseText, [regex]::Escape($databaseMountSource)).Count -ne 1) {
        throw "chart 数据库密码挂载补丁目标数量错误"
    }
    $databaseText = $databaseText.Replace($databaseMountSource, $databaseMountTarget)
    $databaseVolumeSource = "      - name: shm-volume`n        emptyDir:`n          medium: Memory`n          sizeLimit: {{ .Values.database.internal.shmSizeLimit }}"
    $databaseVolumeTarget = @'
      - name: shm-volume
        emptyDir:
          medium: Memory
          sizeLimit: {{ .Values.database.internal.shmSizeLimit }}
      - name: database-password
        secret:
          secretName: "{{ template "harbor.database" . }}"
          defaultMode: 288
          items:
            - key: POSTGRES_PASSWORD
              path: POSTGRES_PASSWORD
'@.TrimEnd("`r", "`n")
    if ([regex]::Matches($databaseText, [regex]::Escape($databaseVolumeSource)).Count -ne 1) {
        throw "chart 数据库密码卷补丁目标数量错误"
    }
    $databaseText = $databaseText.Replace($databaseVolumeSource, $databaseVolumeTarget)
    [System.IO.File]::WriteAllText($databasePath, $databaseText, [System.Text.UTF8Encoding]::new($false))
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
Import-Module (Join-Path $RepoRoot "images\\lib\\ImageMetadata.psm1") -Force
if ([string]::IsNullOrWhiteSpace($ConfigPath)) {
    $ConfigPath = Join-Path $RepoRoot "deploy\config\chaimir.env"
}
$ConfigPath = [System.IO.Path]::GetFullPath($ConfigPath)
$config = Read-ChaimirEnvFile -Path $ConfigPath -RequiredKeys @(
    "SUPPLY_CHAIN_REGISTRY", "SUPPLY_CHAIN_REGISTRY_ENDPOINT", "SUPPLY_CHAIN_HARBOR_EXTERNAL_URL", "IMAGE_REGISTRY",
    "SUPPLY_CHAIN_COSIGN_IMAGE", "SUPPLY_CHAIN_COSIGN_KEY_HOST_DIR"
)
$registry = Get-ChaimirCanonicalRegistry -Config $config
$lockPath = Join-Path $RepoRoot "deploy\charts\harbor\bootstrap.lock.json"
$lock = Get-Content -LiteralPath $lockPath -Raw | ConvertFrom-Json

if ([string]::IsNullOrWhiteSpace($EvidencePath)) {
    $EvidencePath = Get-RequiredConfigValue -Config $config -Key "SUPPLY_CHAIN_HARBOR_BOOTSTRAP_EVIDENCE_PATH"
}
$EvidencePath = Resolve-RepoPath -BasePath $RepoRoot -Path $EvidencePath -Description "Harbor bootstrap 证据"
$evidenceDirectory = Split-Path -Parent $EvidencePath
$evidenceSignaturePath = "$EvidencePath.sig"

if ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
    $OutputDirectory = Join-Path $RepoRoot ".tmp\harbor-bootstrap\chart"
}
$OutputDirectory = Resolve-RepoPath -BasePath $RepoRoot -Path $OutputDirectory -Description "Harbor chart 输出目录"
$bootstrapRoot = Join-Path $RepoRoot ".tmp\harbor-bootstrap"
if (-not $OutputDirectory.StartsWith($bootstrapRoot + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Harbor chart 输出只能位于 $bootstrapRoot"
}

$chartVersion = Get-RequiredConfigValue -Config $config -Key "SUPPLY_CHAIN_HARBOR_CHART_VERSION"
if ($chartVersion -ne [string]$lock.chart.version) {
    throw "SUPPLY_CHAIN_HARBOR_CHART_VERSION 与 bootstrap.lock.json 不一致"
}
$chartRepository = Get-RequiredConfigValue -Config $config -Key "SUPPLY_CHAIN_HARBOR_CHART_REPOSITORY"
if ($chartRepository.TrimEnd("/") -ne ([string]$lock.chart.repository).TrimEnd("/")) {
    throw "SUPPLY_CHAIN_HARBOR_CHART_REPOSITORY 与 bootstrap.lock.json 不一致"
}

$cosignImage = Get-RequiredConfigValue -Config $config -Key "SUPPLY_CHAIN_COSIGN_IMAGE"
if ($cosignImage -notmatch "^[A-Za-z0-9._/-]+@sha256:[0-9a-f]{64}$") {
    throw "SUPPLY_CHAIN_COSIGN_IMAGE 必须固定 digest"
}
$cosignDirectory = Resolve-RepoPath -BasePath (Join-Path $RepoRoot "deploy") -Path (Get-RequiredConfigValue -Config $config -Key "SUPPLY_CHAIN_COSIGN_KEY_HOST_DIR") -Description "Cosign 密钥目录"
if (-not (Test-Path -LiteralPath (Join-Path $cosignDirectory "cosign.pub") -PathType Leaf)) {
    throw "缺少 Harbor bootstrap 验签公钥: $cosignDirectory\cosign.pub"
}
if (-not (Test-Path -LiteralPath $EvidencePath -PathType Leaf) -or -not (Test-Path -LiteralPath $evidenceSignaturePath -PathType Leaf)) {
    throw "缺少 Harbor bootstrap 证据或签名: $EvidencePath"
}
Invoke-CosignVerifyBlob -CosignImage $cosignImage -EvidenceDirectory $evidenceDirectory -CosignDirectory $cosignDirectory -BlobPath $EvidencePath -SignaturePath $evidenceSignaturePath -Description "Harbor bootstrap 证据"

$evidence = Get-Content -LiteralPath $EvidencePath -Raw | ConvertFrom-Json
if ([int]$evidence.schema_version -ne 1 -or [string]$evidence.chart_archive_sha256 -ne [string]$lock.chart.archive_sha256) {
    throw "Harbor bootstrap 证据版本或 chart 摘要不匹配"
}
$bundlePath = Resolve-EvidenceFile -EvidenceDirectory $evidenceDirectory -RelativePath ([string]$evidence.bundle.file) -Description "Harbor OCI 引导包"
$bundleSignaturePath = Resolve-EvidenceFile -EvidenceDirectory $evidenceDirectory -RelativePath ([string]$evidence.bundle.signature_file) -Description "Harbor OCI 引导包签名"
Assert-Sha256 -Path $bundlePath -Expected ([string]$evidence.bundle.sha256) -Description "Harbor OCI 引导包"
Invoke-CosignVerifyBlob -CosignImage $cosignImage -EvidenceDirectory $evidenceDirectory -CosignDirectory $cosignDirectory -BlobPath $bundlePath -SignaturePath $bundleSignaturePath -Description "Harbor OCI 引导包"

$expectedComponents = @($lock.topology.components)
$evidenceComponents = @($evidence.components)
if ($evidenceComponents.Count -ne $expectedComponents.Count) {
    throw "Harbor bootstrap 组件数量不完整: expected=$($expectedComponents.Count) actual=$($evidenceComponents.Count)"
}
$evidenceByID = @{}
foreach ($component in $evidenceComponents) {
    $id = [string]$component.id
    if ([string]::IsNullOrWhiteSpace($id) -or $evidenceByID.ContainsKey($id)) {
        throw "Harbor bootstrap 证据含有缺失或重复组件 id"
    }
    $evidenceByID[$id] = $component
}

$generatedValues = [System.Collections.Generic.List[string]]::new()
$declaredPaths = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::Ordinal)
$generatedValues.Add("# 由 prepare-harbor-chart.ps1 从已签名 bootstrap 证据生成，禁止手工编辑。")
$generatedValues.Add("imagePullPolicy: IfNotPresent")
$generatedValues.Add("expose:")
$generatedValues.Add("  type: ingress")
$generatedValues.Add("metrics:")
$generatedValues.Add("  enabled: false")
foreach ($expected in $expectedComponents) {
    $id = [string]$expected.id
    if (-not $evidenceByID.ContainsKey($id)) {
        throw "Harbor bootstrap 证据缺少组件: $id"
    }
    $actual = $evidenceByID[$id]
    $repository = [string]$actual.repository
    $digest = [string]$actual.digest
    $expectedRepository = "$registry/$([string]$expected.image)"
    if ($repository -ne $expectedRepository -or $digest -notmatch "^sha256:[0-9a-f]{64}$") {
        throw "Harbor bootstrap 组件引用非法: $id"
    }
    if ([int]$actual.trivy_high -ne 0 -or [int]$actual.trivy_critical -ne 0) {
        throw "Harbor bootstrap 组件未通过漏洞门禁: $id"
    }
    $trivyPath = Resolve-EvidenceFile -EvidenceDirectory $evidenceDirectory -RelativePath ([string]$actual.trivy_report.file) -Description "$id Trivy 报告"
    Assert-Sha256 -Path $trivyPath -Expected ([string]$actual.trivy_report.sha256) -Description "$id Trivy 报告"
    if ((Get-TrivyFindingCount -Path $trivyPath -Severity "HIGH") -ne 0 -or (Get-TrivyFindingCount -Path $trivyPath -Severity "CRITICAL") -ne 0) {
        throw "Harbor bootstrap 组件 Trivy 报告仍含 HIGH/CRITICAL: $id"
    }
    $sbomPath = Resolve-EvidenceFile -EvidenceDirectory $evidenceDirectory -RelativePath ([string]$actual.sbom.file) -Description "$id SBOM"
    Assert-Sha256 -Path $sbomPath -Expected ([string]$actual.sbom.sha256) -Description "$id SBOM"
    Add-ImageValues -Lines $generatedValues -DeclaredPaths $declaredPaths -ValuesPath ([string]$expected.values_path) -Repository $repository -Digest $digest
}

New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$archivePath = Join-Path $OutputDirectory "harbor-$chartVersion.tgz"
# Harbor 是供应链启动根；安装阶段只接受已由签名 bootstrap 证据约束的本地固定归档，
# 禁止在恢复或部署时从公网重新下载 chart，避免引入未审计的传输输入。
Assert-Sha256 -Path $archivePath -Expected ([string]$lock.chart.archive_sha256) -Description "Harbor chart 归档"
$chartDirectory = Join-Path $OutputDirectory "harbor"
if (Test-Path -LiteralPath $chartDirectory) {
    Remove-Item -LiteralPath $chartDirectory -Recurse -Force
}
try {
    # Windows bsdtar treats a drive-letter path as a remote archive; extract from the output directory.
    Push-Location $OutputDirectory
    & tar -xzf (Split-Path -Leaf $archivePath)
    if ($LASTEXITCODE -ne 0) {
        throw "解压 Harbor chart 失败"
    }
}
finally {
    Pop-Location
}
if (-not (Test-Path -LiteralPath (Join-Path $chartDirectory "Chart.yaml") -PathType Leaf)) {
    throw "Harbor chart 归档结构非法"
}
Patch-HarborChart -ChartDirectory $chartDirectory
$generatedValuesPath = Join-Path $OutputDirectory "bootstrap-images.values.yaml"
[System.IO.File]::WriteAllLines($generatedValuesPath, $generatedValues, [System.Text.UTF8Encoding]::new($false))

Write-Output "chart_directory=$chartDirectory"
Write-Output "generated_values=$generatedValuesPath"
