# 本脚本在项目固定的 EVM、Chainlink、PostgreSQL 和 Go 构建器中生成并验收真实 OCR2 与 Automation 材料。
param(
    [Parameter(Mandatory = $true)][string]$DigestLockPath,
    [string]$OfficialSourcePath = '',
    [string]$OutputPath = '',
    [ValidateSet('runtime/evm-besu', 'runtime/evm-foundry', 'runtime/evm-geth', 'runtime/evm-hardhat', 'runtime/op-geth')]
    [string]$RuntimeImage = 'runtime/evm-foundry'
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Import-Module (Join-Path $repoRoot 'images\lib\ImageMetadata.psm1') -Force

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

function Invoke-Docker {
    param(
        [string[]]$Arguments,
        [int]$TimeoutSeconds = 300
    )
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = [Diagnostics.ProcessStartInfo]::new()
    $process.StartInfo.FileName = 'docker.exe'
    $process.StartInfo.Arguments = (($Arguments | ForEach-Object { ConvertTo-ProcessArgument ([string]$_) }) -join ' ')
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.CreateNoWindow = $true
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    $process.StartInfo.StandardOutputEncoding = [Text.Encoding]::UTF8
    $process.StartInfo.StandardErrorEncoding = [Text.Encoding]::UTF8
    try {
        if (-not $process.Start()) { throw 'Docker CLI 启动失败' }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
            try {
                & taskkill.exe /PID $process.Id /T /F | Out-Null
            } catch {
                Write-Warning ("Docker 超时后的进程树终止失败: {0}" -f $_.Exception.Message)
            }
            try {
                $process.Kill($true)
            } catch {
                Write-Warning ("Docker 超时后的进程终止失败: {0}" -f $_.Exception.Message)
            }
            $stdout = $stdoutTask.GetAwaiter().GetResult()
            $stderr = $stderrTask.GetAwaiter().GetResult()
            return [pscustomobject]@{ Code = 124; Text = ("Docker CLI 超时（{0} 秒）: {1}{2}{3}" -f $TimeoutSeconds, ($Arguments -join ' '), [Environment]::NewLine, (@($stdout, $stderr) -join [Environment]::NewLine)).Trim() }
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        return [pscustomobject]@{ Code = $process.ExitCode; Text = ((@($stdout, $stderr) | Where-Object { $_ }) -join [Environment]::NewLine).Trim() }
    } finally {
        $process.Dispose()
    }
}

function Require-Value([hashtable]$Values, [string]$Name) {
    if (-not $Values.ContainsKey($Name) -or [string]::IsNullOrWhiteSpace($Values[$Name]) -or $Values[$Name] -match '^changeme') {
        throw "平台 Secret 源中的键无有效值: $Name"
    }
    return [string]$Values[$Name]
}

function Read-EnvFile([string]$Path, [string[]]$RequiredNames) {
    $values = Read-ChaimirEnvFile -Path $Path -RequiredKeys $RequiredNames
    foreach ($name in $RequiredNames) { [void](Require-Value $values $name) }
    return $values
}

function Get-ImageRef([hashtable]$Lock, [string]$Registry, [string]$Image) {
    $digest = $Lock[$Image]
    if ($digest -notmatch '^sha256:[0-9a-f]{64}$') { throw "候选锁缺少合法 digest: $Image" }
    return "$Registry/$Image@$digest"
}

function Wait-Running([string]$Name, [int]$Seconds = 180) {
    $deadline = [DateTime]::UtcNow.AddSeconds($Seconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        $state = Invoke-Docker @('inspect', '--format', '{{.State.Status}}|{{.State.ExitCode}}', $Name)
        if ($state.Code -eq 0 -and $state.Text -match '^running\|0$') { return }
        if ($state.Code -eq 0 -and $state.Text -match '^exited\|[1-9][0-9]*$') {
            throw "容器提前退出: $Name $($state.Text)"
        }
        Start-Sleep -Seconds 1
    }
    throw "容器在限定时间内未运行: $Name"
}

function Wait-Rpc([string]$Network, [string]$ToolsRef) {
    $body = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'))
    $deadline = [DateTime]::UtcNow.AddSeconds(180)
    while ([DateTime]::UtcNow -lt $deadline) {
        $probe = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $Network, '--entrypoint', 'sh', $ToolsRef, '-c', "printf '%s' '$body' | base64 -d >/tmp/rpc.json; curl --connect-timeout 2 --max-time 5 -fsS -H 'content-type: application/json' --data-binary @/tmp/rpc.json http://runtime:8545 | grep -Eq 'result.*0x[0-9a-fA-F]+'")
        if ($probe.Code -eq 0) { return }
        Start-Sleep -Seconds 1
    }
    throw '受控 EVM runtime 在限定时间内未就绪'
}

function Invoke-ChainlinkApi([string]$Network, [string]$ToolsRef, [string]$StateVolume, [string]$ApiHost, [string]$Email, [string]$Password, [string]$Stage, [string]$Command) {
    $result = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $Network, '--entrypoint', 'sh', '--mount', ('source=' + $StateVolume + ',target=/runtime-state,readonly'), $ToolsRef, '-c', ($Command.Replace('__API_HOST__', 'chainlink-node').Replace('__API_CONNECT_TARGET__', $ApiHost)) )
    if ($result.Code -ne 0) { throw "Chainlink API 操作失败: stage=$Stage result=$($result.Text)" }
    return $result.Text
}

function Get-ChainlinkLogs([string]$Name) {
    return (Invoke-Docker @('logs', '--tail', '500', $Name)).Text
}

$lock = Read-ChaimirDigestLock -Path (Resolve-Path -LiteralPath $DigestLockPath).Path -Required
$secrets = Read-EnvFile (Join-Path $repoRoot 'deploy\config\secret.env') @(
    'PG_PASSWORD', 'CHAINLINK_KEYSTORE_PASSWORD', 'CHAINLINK_API_EMAIL', 'CHAINLINK_API_PASSWORD',
    'CHAINLINK_TLS_CERT_B64', 'CHAINLINK_TLS_KEY_B64', 'CHAIMIR_EVM_DEPLOYER_MNEMONIC',
    'CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY', 'OP_GETH_JWT_SECRET'
)
$config = Read-EnvFile (Join-Path $repoRoot 'deploy\config\chaimir.env') @(
    'CHAINLINK_GAS_PRICE_MAX_WEI', 'CHAINLINK_VRF_GAS_LIMIT', 'GO_MODULE_PROXY',
    'SUPPLY_CHAIN_REGISTRY', 'SUPPLY_CHAIN_REGISTRY_ENDPOINT', 'SUPPLY_CHAIN_HARBOR_EXTERNAL_URL', 'IMAGE_REGISTRY'
)
$registry = Get-ChaimirCanonicalRegistry -Config $config
$official = if ([string]::IsNullOrWhiteSpace($OfficialSourcePath)) {
    Join-Path $repoRoot '.tmp\backend-functional-test\evidence\chainlink-source-2.61.0\devenv'
} else {
    (Resolve-Path -LiteralPath $OfficialSourcePath).Path
}
if (-not (Test-Path -LiteralPath (Join-Path $official 'go.mod') -PathType Leaf)) {
    throw "Chainlink 官方固定源码目录不完整: $official"
}
$output = if ([string]::IsNullOrWhiteSpace($OutputPath)) {
    Join-Path $repoRoot '.tmp\backend-functional-test\evidence\controlled-chainlink-oracle-automation-material.json'
} else {
    [IO.Path]::GetFullPath((Join-Path (Get-Location) $OutputPath))
}
$outputParent = Split-Path -Parent $output
$workRoot = Join-Path $repoRoot ('.tmp\backend-functional-test\evidence\chainlink-oracle-automation-work-' + [Guid]::NewGuid().ToString('N'))
$network = 'chaimir-chainlink-oracle-automation-' + [Guid]::NewGuid().ToString('N').Substring(0, 12)
$runtimeVolume = $network + '-runtime'
$postgresTLSVolume = $network + '-postgres-tls'
$runtime = $network + '-runtime-node'
$nodeCount = 4
$chainlinkVolumes = @()
$postgresNames = @()
$chainlinkNames = @()
$chainlinkHosts = @()
for ($index = 1; $index -le $nodeCount; $index++) {
    $chainlinkVolumes += $network + '-chainlink-' + $index
    $postgresNames += $network + '-postgres-' + $index
    $chainlinkNames += $network + '-chainlink-node-' + $index
    $chainlinkHosts += 'chainlink-' + $index
}
$runtimeRef = Get-ImageRef $lock $registry $RuntimeImage
$chainlinkRef = Get-ImageRef $lock $registry 'infra/chainlink-node'
$postgresRef = Get-ImageRef $lock $registry 'middleware/postgres'
$toolsRef = Get-ImageRef $lock $registry 'base/chain-tools'
$foundryRef = Get-ImageRef $lock $registry 'runtime/evm-foundry'
$judgeRef = Get-ImageRef $lock $registry 'base/judge-min'
$builderRef = Get-ImageRef $lock $registry 'base/go-builder'
$deployerKey = $secrets['CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY']
$result = $null
$postgresCertificate = Join-Path $repoRoot 'deploy\config\chaimir-tls\postgres.crt'
$postgresPrivateKey = Join-Path $repoRoot 'deploy\config\chaimir-tls\postgres.key'
foreach ($path in @($postgresCertificate, $postgresPrivateKey)) { if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "缺少 PostgreSQL TLS 材料: $path" } }

New-Item -ItemType Directory -Path $workRoot, $outputParent -Force | Out-Null
try {
    foreach ($name in @($runtime) + $postgresNames + $chainlinkNames) { [void](Invoke-Docker @('rm', '-f', '-v', $name)) }
    [void](Invoke-Docker @('network', 'rm', $network))
    foreach ($volume in @($runtimeVolume) + $chainlinkVolumes + @($postgresTLSVolume)) { [void](Invoke-Docker @('volume', 'rm', $volume)) }

    $networkResult = Invoke-Docker @('network', 'create', '--internal', $network)
    if ($networkResult.Code -ne 0) { throw "创建 Chainlink Oracle/Automation 网络失败: $($networkResult.Text)" }
    foreach ($volume in @($runtimeVolume) + $chainlinkVolumes + @($postgresTLSVolume)) {
        $volumeResult = Invoke-Docker @('volume', 'create', $volume)
        if ($volumeResult.Code -ne 0) { throw "创建 Chainlink 测试卷失败: $($volumeResult.Text)" }
    }
    $tlsSeed = Invoke-Docker @('run', '--rm', '--pull', 'never', '--user', '0:0', '--mount', ('type=bind,source=' + $postgresCertificate + ',target=/seed/tls.crt,readonly'), '--mount', ('type=bind,source=' + $postgresPrivateKey + ',target=/seed/tls.key,readonly'), '--mount', ('type=volume,source=' + $postgresTLSVolume + ',target=/tls'), '--entrypoint', 'sh', $postgresRef, '-c', 'set -eu; cp /seed/tls.crt /tls/tls.crt; cp /seed/tls.key /tls/tls.key; chown 70:70 /tls/tls.crt /tls/tls.key; chmod 644 /tls/tls.crt; chmod 600 /tls/tls.key')
    if ($tlsSeed.Code -ne 0) { throw "注入 Chainlink PostgreSQL TLS 材料失败: $($tlsSeed.Text)" }
    foreach ($volume in $chainlinkVolumes) {
        $statePrepare = Invoke-Docker @('run', '--rm', '--pull', 'never', '--user', '0:0', '--mount', ('source=' + $volume + ',target=/runtime-state'), '--entrypoint', 'sh', $judgeRef, '-c', 'mkdir -p /runtime-state; chown -R 1000:1000 /runtime-state')
        if ($statePrepare.Code -ne 0) { throw "准备 Chainlink runtime-state 权限失败: $($statePrepare.Text)" }
    }
    for ($index = 0; $index -lt $nodeCount; $index++) {
        $postgresResult = Invoke-Docker @('run', '-d', '--pull', 'never', '--name', $postgresNames[$index], '--network', $network, '--network-alias', ('pg-' + ($index + 1)), '--mount', ('type=volume,source=' + $postgresTLSVolume + ',target=/etc/postgresql/tls,readonly'), '-e', 'POSTGRES_USER=chainlink', '-e', 'POSTGRES_DB=chainlink', '-e', "POSTGRES_PASSWORD=$($secrets['PG_PASSWORD'])", '-e', 'POSTGRES_INITDB_ARGS=--auth-local=scram-sha-256 --auth-host=scram-sha-256 -E UTF8 --locale=C', $postgresRef, 'postgres', '-c', 'ssl=on', '-c', 'ssl_cert_file=/etc/postgresql/tls/tls.crt', '-c', 'ssl_key_file=/etc/postgresql/tls/tls.key')
        if ($postgresResult.Code -ne 0) { throw "启动 Chainlink PostgreSQL 节点 $($index + 1) 失败: $($postgresResult.Text)" }
    }

    $runtimeArguments = @('run', '-d', '--pull', 'never', '--name', $runtime, '--network', $network, '--network-alias', 'runtime', '--mount', ('source=' + $runtimeVolume + ',target=/runtime-state'))
    switch ($RuntimeImage) {
        'runtime/evm-foundry' {
            $runtimeArguments += @('-e', 'CHAIMIR_RUNTIME_RPC_PORT=8545', '-e', 'CHAIMIR_CHAIN_STATE_DIR=/runtime-state', '-e', "CHAIMIR_EVM_DEPLOYER_MNEMONIC=$($secrets['CHAIMIR_EVM_DEPLOYER_MNEMONIC'])")
        }
        'runtime/evm-besu' {
            $runtimeArguments += @('-e', 'CHAIMIR_RUNTIME_RPC_PORT=8545', '-e', 'CHAIMIR_CHAIN_STATE_DIR=/runtime-state', '-e', "CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY=$deployerKey")
        }
        'runtime/evm-geth' {
            $runtimeArguments += @('-e', 'CHAIMIR_RUNTIME_RPC_PORT=8545', '-e', 'CHAIMIR_CHAIN_STATE_DIR=/runtime-state', '-e', "CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY=$deployerKey")
        }
        'runtime/evm-hardhat' {
            $runtimeArguments += @('-e', 'CHAIMIR_RUNTIME_RPC_PORT=8545', '-e', 'CHAIMIR_CHAIN_STATE_DIR=/runtime-state', '-e', "CHAIMIR_EVM_DEPLOYER_MNEMONIC=$($secrets['CHAIMIR_EVM_DEPLOYER_MNEMONIC'])")
        }
        'runtime/op-geth' {
            throw 'Chainlink Oracle/Automation 首轮仅允许在标准 EVM runtime 中生成；OP-Stack 需要单独的 L1/L2/Beacon/OP-Node 合法组合执行器'
        }
        default { throw "不支持的 EVM runtime: $RuntimeImage" }
    }
    $runtimeArguments += $runtimeRef
    $runtimeResult = Invoke-Docker $runtimeArguments
    if ($runtimeResult.Code -ne 0) { throw "启动受控 EVM runtime 失败: $($runtimeResult.Text)" }
    Wait-Running $runtime
    Wait-Rpc $network $toolsRef

    for ($index = 0; $index -lt $nodeCount; $index++) {
        $dbReady = $false
        $dbDeadline = [DateTime]::UtcNow.AddSeconds(180)
        while ([DateTime]::UtcNow -lt $dbDeadline) {
            $db = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'psql', '-e', "PGPASSWORD=$($secrets['PG_PASSWORD'])", '-e', 'PGSSLMODE=require', $postgresRef, '-h', ('pg-' + ($index + 1)), '-U', 'chainlink', '-d', 'chainlink', '-At', '-c', 'SELECT 1;')
            if ($db.Code -eq 0 -and $db.Text.Trim() -eq '1') { $dbReady = $true; break }
            Start-Sleep -Seconds 1
        }
        if (-not $dbReady) { throw "Chainlink PostgreSQL 节点 $($index + 1) 在限定时间内未就绪" }
    }

    $chainIDText = (Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'chain-id', '--rpc-url', 'http://runtime:8545')).Text.Trim()
    if ($chainIDText -notmatch '^[0-9]+$' -or [uint64]$chainIDText -eq 0) { throw "runtime 返回非法 chain id: $chainIDText" }
    $chainID = [uint64]$chainIDText

    for ($index = 0; $index -lt $nodeCount; $index++) {
        $chainlinkResult = Invoke-Docker @(
            'run', '-d', '--pull', 'never', '--name', $chainlinkNames[$index], '--network', $network,
            '--network-alias', $chainlinkHosts[$index], '--network-alias', ('chainlink-node-' + ($index + 1)),
            '--mount', ('source=' + $chainlinkVolumes[$index] + ',target=/runtime-state'),
            '-e', 'ETH_URL=http://runtime:8545',
            '-e', ('CHAIMIR_CHAINLINK_DATABASE_ADDRESS=pg-' + ($index + 1) + ':5432'),
            '-e', 'CHAIMIR_CHAINLINK_DATABASE_NAME=chainlink',
            '-e', 'CHAIMIR_CHAINLINK_DATABASE_USER=chainlink',
            '-e', "CHAINLINK_CHAIN_ID=$chainID",
            '-e', "CHAINLINK_GAS_PRICE_MAX_WEI=$($config['CHAINLINK_GAS_PRICE_MAX_WEI'])",
            '-e', "CHAINLINK_VRF_GAS_LIMIT=$($config['CHAINLINK_VRF_GAS_LIMIT'])",
            '-e', "POSTGRES_PASSWORD=$($secrets['PG_PASSWORD'])",
            '-e', "CHAINLINK_KEYSTORE_PASSWORD=$($secrets['CHAINLINK_KEYSTORE_PASSWORD'])",
            '-e', "CHAINLINK_API_EMAIL=$($secrets['CHAINLINK_API_EMAIL'])",
            '-e', "CHAINLINK_API_PASSWORD=$($secrets['CHAINLINK_API_PASSWORD'])",
            '-e', "CHAINLINK_TLS_CERT_B64=$($secrets['CHAINLINK_TLS_CERT_B64'])",
            '-e', "CHAINLINK_TLS_KEY_B64=$($secrets['CHAINLINK_TLS_KEY_B64'])",
            $chainlinkRef
        )
        if ($chainlinkResult.Code -ne 0) { throw "启动 Chainlink 节点 $($index + 1) 失败: $($chainlinkResult.Text)" }
    }
    $sessionBody = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((@{ email = $secrets['CHAINLINK_API_EMAIL']; password = $secrets['CHAINLINK_API_PASSWORD'] } | ConvertTo-Json -Compress)))
    $apiSessionPrefixes = @()
    $apiPrefixes = @()
    for ($index = 0; $index -lt $nodeCount; $index++) {
        $apiHost = $chainlinkHosts[$index]
        $apiSessionPrefix = ('set -eu; cp /runtime-state/chainlink/tls/server.crt /tmp/chainlink-ca.crt; printf "%s" "__SESSION_BODY__" | base64 -d >/tmp/session.json; session_code=$(curl --cacert /tmp/chainlink-ca.crt --connect-to chainlink-node:6688:' + $apiHost + ':6688 -sS -o /tmp/session.response -D /tmp/session.headers -c /tmp/session.cookies -w ''%{http_code}'' -H ''content-type: application/json'' --data-binary ''@/tmp/session.json'' https://chainlink-node:6688/sessions); test "$session_code" = 200; cookie_value=$(grep clsession /tmp/session.cookies | cut -f7); test -n "$cookie_value";').Replace('__SESSION_BODY__', $sessionBody)
        $apiSessionPrefixes += $apiSessionPrefix
        $apiPrefixes += ($apiSessionPrefix + ' curl --cacert /tmp/chainlink-ca.crt --connect-to chainlink-node:6688:' + $apiHost + ':6688 -sS -b clsession=$cookie_value https://chainlink-node:6688')
        $apiReady = $false
        $apiDeadline = [DateTime]::UtcNow.AddSeconds(180)
        while ([DateTime]::UtcNow -lt $apiDeadline) {
            $chainlinkState = Invoke-Docker @('inspect', '--format', '{{.State.Status}}|{{.State.ExitCode}}', $chainlinkNames[$index])
            if ($chainlinkState.Code -eq 0 -and $chainlinkState.Text -match '^exited\|([1-9][0-9]*)$') {
                throw "Chainlink 节点 $($index + 1) 提前退出: $($chainlinkState.Text)" + [Environment]::NewLine + (Get-ChainlinkLogs $chainlinkNames[$index])
            }
            $apiProbe = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'sh', '--mount', ('source=' + $chainlinkVolumes[$index] + ',target=/runtime-state,readonly'), $toolsRef, '-c', ('test -s /runtime-state/chainlink/tls/server.crt && curl --fail --cacert /runtime-state/chainlink/tls/server.crt --connect-to chainlink-node:6688:' + $apiHost + ':6688 --connect-timeout 2 --max-time 4 https://chainlink-node:6688/readyz >/dev/null'))
            if ($apiProbe.Code -eq 0) { $apiReady = $true; break }
            Start-Sleep -Seconds 1
        }
        if (-not $apiReady) { throw "Chainlink 节点 $($index + 1) API 在限定时间内未就绪: $(Get-ChainlinkLogs $chainlinkNames[$index])" }
    }
    $invalidSessionBody = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((@{ email = $secrets['CHAINLINK_API_EMAIL']; password = 'invalid-chainlink-password' } | ConvertTo-Json -Compress)))
    $invalidAuthCommand = ('set -eu; cp /runtime-state/chainlink/tls/server.crt /tmp/chainlink-ca.crt; printf "%s" "__INVALID_SESSION_BODY__" | base64 -d >/tmp/invalid-session.json; code=$(curl --cacert /tmp/chainlink-ca.crt --connect-to chainlink-node:6688:' + $chainlinkHosts[0] + ':6688 -sS -o /tmp/invalid-session.response -w ''%{http_code}'' --connect-timeout 2 --max-time 4 -H ''content-type: application/json'' --data-binary ''@/tmp/invalid-session.json'' https://chainlink-node:6688/sessions); case "$code" in 401|403) ;; *) printf ''invalid_auth_http_status=%s\n'' "$code"; exit 1 ;; esac; printf ''invalid_auth_rejected\n''').Replace('__INVALID_SESSION_BODY__', $invalidSessionBody)
    $invalidAuthResult = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'sh', '--mount', ('source=' + $chainlinkVolumes[0] + ',target=/runtime-state,readonly'), $toolsRef, '-c', $invalidAuthCommand)
    if ($invalidAuthResult.Code -ne 0 -or $invalidAuthResult.Text -notmatch 'invalid_auth_rejected') { throw "Chainlink 错误鉴权未被拒绝: $($invalidAuthResult.Text)" }

    $nodePublic = @()
    for ($index = 0; $index -lt $nodeCount; $index++) {
        $ethKeys = (Invoke-ChainlinkApi $network $toolsRef $chainlinkVolumes[$index] $chainlinkHosts[$index] $secrets['CHAINLINK_API_EMAIL'] $secrets['CHAINLINK_API_PASSWORD'] ("read-eth-keys-node-$($index + 1)") ($apiPrefixes[$index] + '/v2/keys/eth')) | ConvertFrom-Json
        $ethKey = @($ethKeys.data | Where-Object { $_.attributes.address -match '^0x[0-9a-fA-F]{40}$' })[0]
        $ocrKeys = (Invoke-ChainlinkApi $network $toolsRef $chainlinkVolumes[$index] $chainlinkHosts[$index] $secrets['CHAINLINK_API_EMAIL'] $secrets['CHAINLINK_API_PASSWORD'] ("read-ocr2-keys-node-$($index + 1)") ($apiPrefixes[$index] + '/v2/keys/ocr2')) | ConvertFrom-Json
        $ocrKey = @($ocrKeys.data | Where-Object { $_.attributes.chainType -eq 'evm' -and $_.attributes.configPublicKey -and $_.attributes.offChainPublicKey -and $_.attributes.onChainPublicKey })[0]
        $p2pKeys = (Invoke-ChainlinkApi $network $toolsRef $chainlinkVolumes[$index] $chainlinkHosts[$index] $secrets['CHAINLINK_API_EMAIL'] $secrets['CHAINLINK_API_PASSWORD'] ("read-p2p-keys-node-$($index + 1)") ($apiPrefixes[$index] + '/v2/keys/p2p')) | ConvertFrom-Json
        $p2pKey = @($p2pKeys.data | Where-Object { $_.attributes.peerId -and $_.attributes.peerId -notmatch '^$' })[0]
        if ($null -eq $ethKey -or $null -eq $ocrKey -or $null -eq $p2pKey) {
            throw "Chainlink 节点 $($index + 1) 未返回完整 EVM/OCR2/P2P 公开身份: eth=$(@($ethKeys.data).Count) ocr2=$(@($ocrKeys.data).Count) p2p=$(@($p2pKeys.data).Count)"
        }
        $nodePublic += [ordered]@{
            transmitter = [string]$ethKey.attributes.address
            p2p_peer_id = [string]$p2pKey.attributes.peerId
            ocr2_config_public_key = [string]$ocrKey.attributes.configPublicKey
            ocr2_offchain_public_key = [string]$ocrKey.attributes.offChainPublicKey
            ocr2_onchain_public_key = [string]$ocrKey.attributes.onChainPublicKey
            ocr2_key_id = [string]$ocrKey.id
        }
        $oracleFunding = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'send', $ethKey.attributes.address, '--value', '5 ether', '--rpc-url', 'http://runtime:8545', '--private-key', $deployerKey, '--json')
        if ($oracleFunding.Code -ne 0) { throw "Chainlink EVM oracle 钱包 $($index + 1) 注资失败: $($oracleFunding.Text)" }
    }
    if ($nodePublic.Count -ne $nodeCount) { throw "Chainlink 节点公开身份数量错误: $($nodePublic.Count)" }
    $nodeJsonPath = Join-Path $workRoot 'chainlink-node-public.json'
    $nodeJson = ($nodePublic | ConvertTo-Json -Depth 4) + [Environment]::NewLine
    [IO.File]::WriteAllText($nodeJsonPath, $nodeJson, [Text.UTF8Encoding]::new($false))

    $modulePath = Join-Path $repoRoot 'deploy\scripts\chainlink-material-export'
    $exportBase = @(
        'run', '--rm', '--pull', 'never', '--user', '0:0', '--network', $network,
        '-v', ((Resolve-Path -LiteralPath $modulePath).Path + ':/work:ro'),
        '-v', ($official + ':/official/devenv:ro'),
        '-v', ($workRoot + ':/out'),
        '--mount', 'source=chaimir-composition-go-mod-cache,target=/runtime-state',
        '--mount', 'source=chaimir-composition-go-build-cache,target=/tmp/go-cache',
        '-e', "GOPROXY=$($config['GO_MODULE_PROXY'])",
        '-e', "DEPLOYER_PRIVATE_KEY=$deployerKey",
        '-e', "CHAIN_ID=$chainID"
    )
    $buildAndExport = 'set -eu; export GOPATH=/runtime-state/go GOMODCACHE=/runtime-state/go/pkg/mod GOCACHE=/tmp/go-cache; cd /work; /usr/local/go/bin/go build -mod=readonly -o /tmp/chainlink-material-export .; /tmp/chainlink-material-export -rpc http://runtime:8545 -private-key "$DEPLOYER_PRIVATE_KEY" -chain-id "$CHAIN_ID" -node-json /node-public.json -protocol __PROTOCOL__ -out /out/__OUTPUT__'
    $nodeMount = ($nodeJsonPath + ':/node-public.json:ro')
    $ocrExport = Invoke-Docker -TimeoutSeconds 1200 ($exportBase + @('-v', $nodeMount, $builderRef, 'sh', '-c', $buildAndExport.Replace('__PROTOCOL__', 'ocr2').Replace('__OUTPUT__', 'chainlink-ocr2-material.json')))
    if ($ocrExport.Code -ne 0) { throw "Chainlink 官方 OCR2 材料部署失败: $($ocrExport.Text)" }
    $automationExport = Invoke-Docker -TimeoutSeconds 1200 ($exportBase + @('-v', $nodeMount, $builderRef, 'sh', '-c', $buildAndExport.Replace('__PROTOCOL__', 'automation').Replace('__OUTPUT__', 'chainlink-automation-material.json')))
    if ($automationExport.Code -ne 0) { throw "Chainlink 官方 Automation 材料部署失败: $($automationExport.Text)" }
    $ocrPath = Join-Path $workRoot 'chainlink-ocr2-material.json'
    $automationPath = Join-Path $workRoot 'chainlink-automation-material.json'
    if (-not (Test-Path -LiteralPath $ocrPath -PathType Leaf) -or -not (Test-Path -LiteralPath $automationPath -PathType Leaf)) {
        throw 'Chainlink OCR2 或 Automation 材料未生成'
    }
    $ocr = Get-Content -LiteralPath $ocrPath -Raw | ConvertFrom-Json
    $automation = Get-Content -LiteralPath $automationPath -Raw | ConvertFrom-Json
    foreach ($address in @($ocr.aggregator, $ocr.source_feed, $ocr.link_eth_feed, $automation.registry, $automation.registrar, $automation.upkeep)) {
        if ([string]$address -notmatch '^0x[0-9a-fA-F]{40}$') { throw "Chainlink 合约材料地址无效: $address" }
    }
    if ([string]$automation.upkeep_id -notmatch '^[1-9][0-9]*$' -or [int]$ocr.f -ne 1 -or [int]$automation.f -ne 1 -or $nodePublic.Count -ne 4) {
        throw 'Chainlink OCR2/Automation 配置不是项目受控的四节点 f=1 组合'
    }

    $ocrObservation = @'
selector [type=ethabiencode abi="latestRoundData()"];
read [type=ethcall contract="__SOURCE_FEED__" data="$(selector)" evmChainID="__CHAIN_ID__"];
decode [type=ethabidecode abi="uint80 roundId, int256 answer, uint256 startedAt, uint256 updatedAt, uint80 answeredInRound" data="$(read)"];
answer_lookup [type=lookup key="answer"];
selector -> read -> decode -> answer_lookup;
'@
    $ocrFeeObservation = @'
selector [type=ethabiencode abi="latestRoundData()"];
read [type=ethcall contract="__LINK_ETH_FEED__" data="$(selector)" evmChainID="__CHAIN_ID__"];
decode [type=ethabidecode abi="uint80 roundId, int256 answer, uint256 startedAt, uint256 updatedAt, uint80 answeredInRound" data="$(read)"];
answer_lookup [type=lookup key="answer"];
selector -> read -> decode -> answer_lookup;
'@
    $ocrJobSpec = @'
type = "offchainreporting2"
schemaVersion = 1
name = "chaimir-ocr2-median"
forwardingAllowed = false
relay = "evm"
contractID = "__AGGREGATOR__"
pluginType = "median"
ocrKeyBundleID = "__OCR_KEY_ID__"
transmitterID = "__TRANSMITTER__"
p2pv2Bootstrappers = ["__P2P_PEER__@chainlink-1:6690"]
contractConfigTrackerPollInterval = "1s"
observationSource = """
__OCR_OBSERVATION__
"""
[pluginConfig]
juelsPerFeeCoinSource = """
__OCR_FEE_OBSERVATION__
"""
[relayConfig]
chainID = __CHAIN_ID__
'@
    $bootstrapPeerID = [string]$nodePublic[0].p2p_peer_id
    if ($bootstrapPeerID -match '^p2p_') { $bootstrapPeerID = $bootstrapPeerID.Substring(4) }
    if ($bootstrapPeerID -notmatch '^[1-9A-HJ-NP-Za-km-z]+$') { throw "Chainlink bootstrap Peer ID 不是合法 Base58 值: $bootstrapPeerID" }
    $ocrJobSpec = $ocrJobSpec.Replace('__AGGREGATOR__', [string]$ocr.aggregator).Replace('__OCR_OBSERVATION__', $ocrObservation.Trim()).Replace('__OCR_FEE_OBSERVATION__', $ocrFeeObservation.Trim()).Replace('__SOURCE_FEED__', [string]$ocr.source_feed).Replace('__LINK_ETH_FEED__', [string]$ocr.link_eth_feed).Replace('__P2P_PEER__', $bootstrapPeerID).Replace('__CHAIN_ID__', [string]$chainID)
    $bootstrapJobSpec = @'
type = "bootstrap"
schemaVersion = 1
name = "chaimir-ocr2-bootstrap"
forwardingAllowed = false
relay = "evm"
contractID = "__AGGREGATOR__"
p2pv2Bootstrappers = []
contractConfigTrackerPollInterval = "1s"
[relayConfig]
chainID = __CHAIN_ID__
'@
    $bootstrapJobSpec = $bootstrapJobSpec.Replace('__AGGREGATOR__', [string]$ocr.aggregator).Replace('__CHAIN_ID__', [string]$chainID)
    $automationJobSpec = @'
type = "offchainreporting2"
schemaVersion = 1
name = "chaimir-automation-v2"
forwardingAllowed = false
relay = "evm"
contractID = "__REGISTRY__"
pluginType = "ocr2automation"
ocrKeyBundleID = "__OCR_KEY_ID__"
transmitterID = "__TRANSMITTER__"
p2pv2Bootstrappers = ["__P2P_PEER__@chainlink-1:6690"]
contractConfigTrackerPollInterval = "1s"
[pluginConfig]
contractVersion = "v2.0"
maxServiceWorkers = 10
cacheEvictionInterval = "1s"
[relayConfig]
chainID = __CHAIN_ID__
'@
    $automationJobSpec = $automationJobSpec.Replace('__REGISTRY__', [string]$automation.registry).Replace('__P2P_PEER__', $bootstrapPeerID).Replace('__CHAIN_ID__', [string]$chainID)

    function New-ChainlinkJob([int]$NodeIndex, [string]$Spec, [string]$Name) {
        $body = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((@{ toml = $Spec } | ConvertTo-Json -Compress)))
        $command = $apiSessionPrefixes[$NodeIndex] + ' printf "%s" "__JOB_BODY__" | base64 -d >/tmp/job.json; code=$(curl --cacert /tmp/chainlink-ca.crt --connect-to chainlink-node:6688:' + $chainlinkHosts[$NodeIndex] + ':6688 -sS -o /tmp/job.response -w ''%{http_code}'' -b clsession=$cookie_value -H ''content-type: application/json'' --data-binary @/tmp/job.json https://chainlink-node:6688/v2/jobs); if test "$code" != 200 && test "$code" != 201; then printf ''job_http_status=%s\n'' "$code"; jq -c ''.errors // .'' /tmp/job.response; exit 1; fi; jq -r ''.data.id'' /tmp/job.response'
        $result = Invoke-ChainlinkApi $network $toolsRef $chainlinkVolumes[$NodeIndex] $chainlinkHosts[$NodeIndex] $secrets['CHAINLINK_API_EMAIL'] $secrets['CHAINLINK_API_PASSWORD'] $Name $command.Replace('__JOB_BODY__', $body)
        $jobID = $result.Trim().Trim('"')
        if ($jobID -notmatch '^(?:[1-9][0-9]*|[0-9a-fA-F-]{20,})$') { throw "Chainlink $Name job 创建结果无效: $jobID" }
        return $jobID
    }

    $bootstrapJobID = New-ChainlinkJob 0 $bootstrapJobSpec 'create-ocr2-bootstrap-job-node-1'
    $ocrJobIDs = @()
    $automationJobIDs = @()
    for ($index = 1; $index -lt $nodeCount; $index++) {
        $nodeOCRJobSpec = $ocrJobSpec.Replace('__OCR_KEY_ID__', [string]$nodePublic[$index].ocr2_key_id).Replace('__TRANSMITTER__', [string]$nodePublic[$index].transmitter)
        $nodeAutomationJobSpec = $automationJobSpec.Replace('__OCR_KEY_ID__', [string]$nodePublic[$index].ocr2_key_id).Replace('__TRANSMITTER__', [string]$nodePublic[$index].transmitter)
        $ocrJobIDs += New-ChainlinkJob $index $nodeOCRJobSpec ("create-ocr2-job-node-$($index + 1)")
        $automationJobIDs += New-ChainlinkJob $index $nodeAutomationJobSpec ("create-automation-job-node-$($index + 1)")
    }

    # 所有 job 已创建后回放配置交易所在区块，使新增的 OCR2 过滤器能够消费既有 ConfigSet 日志。
    for ($index = 0; $index -lt $nodeCount; $index++) {
        $replayCommand = $apiSessionPrefixes[$index] + ' code=$(curl --cacert /tmp/chainlink-ca.crt --connect-to chainlink-node:6688:' + $chainlinkHosts[$index] + ':6688 -sS -o /tmp/replay.response -w ''%{http_code}'' -b clsession=$cookie_value -X POST "https://chainlink-node:6688/v2/replay_from_block/1?force=true&family=evm&ChainID=' + $chainID + '" -H "content-type: application/json" --data-binary "{}"); test "$code" = 200; grep -q "Replay started" /tmp/replay.response; printf "Replay started\\n"'
        $replayResult = Invoke-ChainlinkApi $network $toolsRef $chainlinkVolumes[$index] $chainlinkHosts[$index] $secrets['CHAINLINK_API_EMAIL'] $secrets['CHAINLINK_API_PASSWORD'] ("replay-chainlink-log-poller-node-$($index + 1)") $replayCommand
        if ($replayResult -notmatch 'Replay started') { throw "Chainlink 节点 $($index + 1) 日志回放响应无效: $replayResult" }
    }

    $deployerAddress = (Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'wallet', 'address', '--private-key', $deployerKey)).Text.Trim()
    if ($deployerAddress -notmatch '^0x[0-9a-fA-F]{40}$') { throw "受控部署账户地址无效: $deployerAddress" }
    $ocrReady = $false
    $automationReady = $false
    $deadline = [DateTime]::UtcNow.AddSeconds(360)
    $headAdvanceCount = 0
    while ([DateTime]::UtcNow -lt $deadline) {
        if ($headAdvanceCount -lt 90) {
            $advanceHead = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'send', $deployerAddress, '--value', '0', '--rpc-url', 'http://runtime:8545', '--private-key', $deployerKey, '--json')
            if ($advanceHead.Code -ne 0) { throw "推进受控链头的真实交易失败: $($advanceHead.Text)" }
            $headAdvanceCount++
        }
        $answer = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'call', $ocr.aggregator, 'latestAnswer()(int256)', '--rpc-url', 'http://runtime:8545')
        $answerMatch = [regex]::Match($answer.Text.Trim(), '^(?<value>-?[0-9]+)(?:\s|$)')
        if ($answer.Code -eq 0 -and $answerMatch.Success -and $answerMatch.Groups['value'].Value -eq '12345') { $ocrReady = $true }
        $counter = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'call', $automation.upkeep, 'counter()(uint256)', '--rpc-url', 'http://runtime:8545')
        $counterMatch = [regex]::Match($counter.Text.Trim(), '^(?<value>[1-9][0-9]*)(?:\s|$)')
        if ($counter.Code -eq 0 -and $counterMatch.Success) { $automationReady = $true }
        if ($ocrReady -and $automationReady) { break }
        Start-Sleep -Seconds 2
    }
    if (-not $ocrReady -or -not $automationReady) {
        $latestBlock = (Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'block', 'latest', '--rpc-url', 'http://runtime:8545', '--json')).Text
        $ocrTransmission = (Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'call', $ocr.aggregator, 'latestTransmissionDetails()(bytes32,uint32,uint8,int192,uint64)', '--rpc-url', 'http://runtime:8545')).Text.Trim()
        $ocrRoundData = (Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'call', $ocr.aggregator, 'latestRoundData()(uint80,int256,uint256,uint256,uint80)', '--rpc-url', 'http://runtime:8545')).Text.Trim()
        $ocrEvents = (Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'logs', '--address', $ocr.aggregator, '--from-block', '1', '--to-block', 'latest', '--rpc-url', 'http://runtime:8545')).Text.Trim()
        $logs = ($chainlinkNames | ForEach-Object { Get-ChainlinkLogs $_ }) -join [Environment]::NewLine
        throw "Chainlink OCR2/Automation 真实联动未在限定时间内完成: ocr=$ocrReady automation=$automationReady latest_block=$latestBlock ocr_transmission=$ocrTransmission ocr_round_data=$ocrRoundData ocr_events=$ocrEvents" + [Environment]::NewLine + $logs
    }

    $result = [ordered]@{
        schema_version = 1
        protocol = 'chainlink-oracle-automation'
        version = '2.61.0'
        chain_id = $chainID
        runtime_image = $RuntimeImage
        runtime_digest = $lock[$RuntimeImage]
        chainlink_digest = $lock['infra/chainlink-node']
        postgres_digest = $lock['middleware/postgres']
        ocr2 = $ocr
        automation = $automation
        jobs = @{
            bootstrap = $bootstrapJobID
            ocr2 = $ocrJobIDs
            automation = $automationJobIDs
        }
        verified = @{
            api_authentication = $true
            ocr2_source_to_aggregator = $true
            automation_upkeep_check_perform = $true
            cleanup = $false
        }
    }
} finally {
    foreach ($name in @($runtime) + $postgresNames + $chainlinkNames) {
        $removed = Invoke-Docker @('rm', '-f', '-v', $name)
        if ($removed.Code -ne 0 -and $removed.Text -notmatch 'No such container') { throw "清理 Chainlink 测试容器失败: $name $($removed.Text)" }
    }
    $networkRemoved = Invoke-Docker @('network', 'rm', $network)
    if ($networkRemoved.Code -ne 0 -and $networkRemoved.Text -notmatch 'No such network') { throw "清理 Chainlink 测试网络失败: $network $($networkRemoved.Text)" }
    foreach ($volume in @($runtimeVolume) + $chainlinkVolumes + @($postgresTLSVolume)) {
        $removed = Invoke-Docker @('volume', 'rm', $volume)
        if ($removed.Code -ne 0 -and $removed.Text -notmatch 'No such volume') { throw "清理 Chainlink 测试卷失败: $volume $($removed.Text)" }
    }
    $leftovers = @()
    foreach ($name in @($runtime) + $postgresNames + $chainlinkNames) {
        $state = Invoke-Docker @('ps', '-aq', '--filter', "name=^/$name`$")
        if ($state.Code -ne 0) { throw "核验 Chainlink 测试容器清理失败: $name $($state.Text)" }
        if (-not [string]::IsNullOrWhiteSpace($state.Text)) { $leftovers += "container:$name" }
    }
    foreach ($volume in @($runtimeVolume) + $chainlinkVolumes) {
        $state = Invoke-Docker @('volume', 'ls', '-q', '--filter', "name=^$volume`$")
        if ($state.Code -ne 0) { throw "核验 Chainlink 测试卷清理失败: $volume $($state.Text)" }
        if (-not [string]::IsNullOrWhiteSpace($state.Text)) { $leftovers += "volume:$volume" }
    }
    $networkState = Invoke-Docker @('network', 'ls', '-q', '--filter', "name=^$network`$")
    if ($networkState.Code -ne 0) { throw "核验 Chainlink 测试网络清理失败: $network $($networkState.Text)" }
    if (-not [string]::IsNullOrWhiteSpace($networkState.Text)) { $leftovers += "network:$network" }
    if ($leftovers.Count -gt 0) { throw "Chainlink 测试资源清理后仍有残留: $($leftovers -join ', ')" }
    if ($null -ne $result) {
        $result.verified.cleanup = $true
        $resultJson = ($result | ConvertTo-Json -Depth 12) + [Environment]::NewLine
        [IO.File]::WriteAllText($output, $resultJson, [Text.UTF8Encoding]::new($false))
        Write-Output "受控 Chainlink OCR2 与 Automation 材料及真实联动已生成: $output"
    }
    if (Test-Path -LiteralPath $workRoot) { Remove-Item -LiteralPath $workRoot -Recurse -Force }
}
