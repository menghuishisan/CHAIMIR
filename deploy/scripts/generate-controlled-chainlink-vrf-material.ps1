# 本脚本在项目固定的 EVM、Chainlink、PostgreSQL 和 Go 构建器中生成并验收真实 VRF 材料。
param(
    [Parameter(Mandatory = $true)][string]$DigestLockPath,
    [string]$OfficialSourcePath = '',
    [string]$OutputPath = '',
    [string]$RuntimeImage = 'runtime/evm-foundry'
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
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
            return [pscustomobject]@{ Code = 124; Text = ("Docker CLI 超时（{0} 秒）: {1}`n{2}`n{3}" -f $TimeoutSeconds, ($Arguments -join ' '), $stdout, $stderr).Trim() }
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        return [pscustomobject]@{ Code = $process.ExitCode; Text = ((@($stdout, $stderr) | Where-Object { $_ }) -join "`n").Trim() }
    } finally { $process.Dispose() }
}

function Require-Value([hashtable]$Values, [string]$Name) {
    if (-not $Values.ContainsKey($Name) -or [string]::IsNullOrWhiteSpace($Values[$Name]) -or $Values[$Name] -match '^changeme') {
        throw "平台 Secret 源中的键无有效值: $Name"
    }
    return [string]$Values[$Name]
}

function Read-PlatformSecrets {
    $path = Join-Path $repoRoot 'deploy\config\secret.env'
    $values = Read-ChaimirEnvFile -Path $path -RequiredKeys @(
        'PG_PASSWORD', 'CHAINLINK_KEYSTORE_PASSWORD', 'CHAINLINK_API_EMAIL', 'CHAINLINK_API_PASSWORD',
        'CHAINLINK_TLS_CERT_B64', 'CHAINLINK_TLS_KEY_B64', 'CHAIMIR_EVM_DEPLOYER_MNEMONIC', 'CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY'
    )
    foreach ($name in @('PG_PASSWORD', 'CHAINLINK_KEYSTORE_PASSWORD', 'CHAINLINK_API_EMAIL', 'CHAINLINK_API_PASSWORD', 'CHAINLINK_TLS_CERT_B64', 'CHAINLINK_TLS_KEY_B64', 'CHAIMIR_EVM_DEPLOYER_MNEMONIC', 'CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY')) {
        [void](Require-Value $values $name)
    }
    return $values
}

function Read-PlatformConfig {
    $path = Join-Path $repoRoot 'deploy\config\chaimir.env'
    $values = Read-ChaimirEnvFile -Path $path -RequiredKeys @(
        'CHAINLINK_GAS_PRICE_MAX_WEI', 'CHAINLINK_VRF_GAS_LIMIT', 'GO_MODULE_PROXY',
        'SUPPLY_CHAIN_REGISTRY', 'SUPPLY_CHAIN_REGISTRY_ENDPOINT', 'SUPPLY_CHAIN_HARBOR_EXTERNAL_URL', 'IMAGE_REGISTRY'
    )
    foreach ($name in @('CHAINLINK_GAS_PRICE_MAX_WEI', 'CHAINLINK_VRF_GAS_LIMIT')) {
        [void](Require-Value $values $name)
        if ($values[$name] -notmatch '^[0-9]+$' -or $values[$name] -notmatch '[1-9]') {
            throw "平台统一配置 $name 必须是正十进制整数"
        }
    }
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
        if ($state.Code -eq 0 -and $state.Text -match '^exited\|[1-9][0-9]*$') { throw "容器提前退出: $Name $($state.Text)" }
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

function Invoke-ChainlinkApi([string]$Network, [string]$ToolsRef, [string]$StateVolume, [string]$Email, [string]$Password, [string]$Stage, [string]$Command) {
    $result = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $Network, '--entrypoint', 'sh', '--mount', ('source=' + $StateVolume + ',target=/runtime-state,readonly'), '-e', "CHAINLINK_API_EMAIL=$Email", '-e', "CHAINLINK_API_PASSWORD=$Password", $ToolsRef, '-c', $Command)
    if ($result.Code -ne 0) { throw "Chainlink API 操作失败: stage=$Stage result=$($result.Text)" }
    return $result.Text
}

$lock = Read-ChaimirDigestLock -Path (Resolve-Path -LiteralPath $DigestLockPath).Path -Required
$secrets = Read-PlatformSecrets
$config = Read-PlatformConfig
$registry = Get-ChaimirCanonicalRegistry -Config $config
$official = if ([string]::IsNullOrWhiteSpace($OfficialSourcePath)) { Join-Path $repoRoot '.tmp\backend-functional-test\evidence\chainlink-source-2.61.0\devenv' } else { (Resolve-Path -LiteralPath $OfficialSourcePath).Path }
if (-not (Test-Path -LiteralPath (Join-Path $official 'go.mod') -PathType Leaf)) { throw "Chainlink 官方固定源码目录不完整: $official" }
$output = if ([string]::IsNullOrWhiteSpace($OutputPath)) { Join-Path $repoRoot '.tmp\backend-functional-test\evidence\controlled-chainlink-vrf-material.json' } else { [IO.Path]::GetFullPath((Join-Path (Get-Location) $OutputPath)) }
if (Test-Path -LiteralPath $output) { throw "拒绝覆盖已存在的 Chainlink VRF 材料: $output" }
$outputParent = Split-Path -Parent $output
$workRoot = Join-Path $repoRoot ('.tmp\backend-functional-test\evidence\chainlink-vrf-work-' + [Guid]::NewGuid().ToString('N'))
$network = 'chaimir-chainlink-vrf-' + [Guid]::NewGuid().ToString('N').Substring(0, 12)
$runtimeVolume = $network + '-runtime'
$chainlinkVolume = $network + '-chainlink'
$postgres = $network + '-postgres'
$postgresTLSVolume = $network + '-postgres-tls'
$runtime = $network + '-runtime-node'
$chainlink = $network + '-chainlink-node'
$runtimeRef = Get-ImageRef $lock $registry $RuntimeImage
$chainlinkRef = Get-ImageRef $lock $registry 'infra/chainlink-node'
$postgresRef = Get-ImageRef $lock $registry 'middleware/postgres'
$toolsRef = Get-ImageRef $lock $registry 'base/chain-tools'
$foundryRef = Get-ImageRef $lock $registry 'runtime/evm-foundry'
$judgeRef = Get-ImageRef $lock $registry 'base/judge-min'
$builderRef = Get-ImageRef $lock $registry 'base/go-builder'
$deployerKey = $secrets['CHAIMIR_EVM_DEPLOYER_PRIVATE_KEY']
$chainID = 0
$postgresCertificate = Join-Path $repoRoot 'deploy\config\chaimir-tls\postgres.crt'
$postgresPrivateKey = Join-Path $repoRoot 'deploy\config\chaimir-tls\postgres.key'
foreach ($path in @($postgresCertificate, $postgresPrivateKey)) { if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "缺少 PostgreSQL TLS 材料: $path" } }

New-Item -ItemType Directory -Path $workRoot, $outputParent -Force | Out-Null
$generationSucceeded = $false
$cleanupErrors = [System.Collections.Generic.List[string]]::new()
try {
    foreach ($name in @($postgres, $runtime, $chainlink)) { [void](Invoke-Docker @('rm', '-f', '-v', $name)) }
    [void](Invoke-Docker @('network', 'rm', $network))
    foreach ($volume in @($runtimeVolume, $chainlinkVolume, $postgresTLSVolume)) { [void](Invoke-Docker @('volume', 'rm', $volume)) }
    $networkResult = Invoke-Docker @('network', 'create', '--internal', $network)
    if ($networkResult.Code -ne 0) { throw "创建 Chainlink VRF 网络失败: $($networkResult.Text)" }
    foreach ($volume in @($runtimeVolume, $chainlinkVolume, $postgresTLSVolume)) {
        $volumeResult = Invoke-Docker @('volume', 'create', $volume)
        if ($volumeResult.Code -ne 0) { throw "创建 Chainlink VRF 测试卷失败: $($volumeResult.Text)" }
    }
    $tlsSeed = Invoke-Docker @('run', '--rm', '--pull', 'never', '--user', '0:0', '--mount', ('type=bind,source=' + $postgresCertificate + ',target=/seed/tls.crt,readonly'), '--mount', ('type=bind,source=' + $postgresPrivateKey + ',target=/seed/tls.key,readonly'), '--mount', ('type=volume,source=' + $postgresTLSVolume + ',target=/tls'), '--entrypoint', 'sh', $postgresRef, '-c', 'set -eu; cp /seed/tls.crt /tls/tls.crt; cp /seed/tls.key /tls/tls.key; chown 70:70 /tls/tls.crt /tls/tls.key; chmod 644 /tls/tls.crt; chmod 600 /tls/tls.key')
    if ($tlsSeed.Code -ne 0) { throw "注入 Chainlink PostgreSQL TLS 材料失败: $($tlsSeed.Text)" }
    $statePrepare = Invoke-Docker @('run', '--rm', '--pull', 'never', '--user', '0:0', '--mount', ('source=' + $chainlinkVolume + ',target=/runtime-state'), '--entrypoint', 'sh', $judgeRef, '-c', 'mkdir -p /runtime-state; chown -R 1000:1000 /runtime-state')
    if ($statePrepare.Code -ne 0) { throw "准备 Chainlink runtime-state 权限失败: $($statePrepare.Text)" }
    $postgresResult = Invoke-Docker @('run', '-d', '--pull', 'never', '--name', $postgres, '--network', $network, '--network-alias', 'pg', '--mount', ('type=volume,source=' + $postgresTLSVolume + ',target=/etc/postgresql/tls,readonly'), '-e', 'POSTGRES_USER=chainlink', '-e', 'POSTGRES_DB=chainlink', '-e', "POSTGRES_PASSWORD=$($secrets['PG_PASSWORD'])", '-e', 'POSTGRES_INITDB_ARGS=--auth-local=scram-sha-256 --auth-host=scram-sha-256 -E UTF8 --locale=C', $postgresRef, 'postgres', '-c', 'ssl=on', '-c', 'ssl_cert_file=/etc/postgresql/tls/tls.crt', '-c', 'ssl_key_file=/etc/postgresql/tls/tls.key')
    if ($postgresResult.Code -ne 0) { throw "启动 Chainlink PostgreSQL 失败: $($postgresResult.Text)" }
    $runtimeResult = Invoke-Docker @('run', '-d', '--pull', 'never', '--name', $runtime, '--network', $network, '--network-alias', 'runtime', '--mount', ('source=' + $runtimeVolume + ',target=/runtime-state'), '-e', 'CHAIMIR_RUNTIME_RPC_PORT=8545', '-e', 'CHAIMIR_CHAIN_STATE_DIR=/runtime-state', '-e', "CHAIMIR_EVM_DEPLOYER_MNEMONIC=$($secrets['CHAIMIR_EVM_DEPLOYER_MNEMONIC'])", $runtimeRef)
    if ($runtimeResult.Code -ne 0) { throw "启动受控 EVM runtime 失败: $($runtimeResult.Text)" }
    Wait-Running $runtime
    Wait-Rpc $network $toolsRef

    $dbReady = $false
    $dbDeadline = [DateTime]::UtcNow.AddSeconds(180)
    while ([DateTime]::UtcNow -lt $dbDeadline) {
        $db = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'psql', '-e', "PGPASSWORD=$($secrets['PG_PASSWORD'])", '-e', 'PGSSLMODE=require', $postgresRef, '-h', 'pg', '-U', 'chainlink', '-d', 'chainlink', '-At', '-c', 'SELECT 1;')
        if ($db.Code -eq 0 -and $db.Text.Trim() -eq '1') { $dbReady = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $dbReady) { throw 'Chainlink PostgreSQL 在限定时间内未就绪' }
    $chainIDText = (Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'chain-id', '--rpc-url', 'http://runtime:8545')).Text.Trim()
    if ($chainIDText -notmatch '^[0-9]+$') { throw "runtime 返回非法 chain id: $chainIDText" }
    $chainID = [uint64]$chainIDText

    $chainlinkResult = Invoke-Docker @('run', '-d', '--pull', 'never', '--name', $chainlink, '--network', $network, '--network-alias', 'chainlink', '--network-alias', 'chainlink-node', '--mount', ('source=' + $chainlinkVolume + ',target=/runtime-state'), '-e', 'ETH_URL=http://runtime:8545', '-e', 'CHAIMIR_CHAINLINK_DATABASE_ADDRESS=pg:5432', '-e', 'CHAIMIR_CHAINLINK_DATABASE_NAME=chainlink', '-e', 'CHAIMIR_CHAINLINK_DATABASE_USER=chainlink', '-e', "CHAINLINK_CHAIN_ID=$chainID", '-e', "CHAINLINK_GAS_PRICE_MAX_WEI=$($config['CHAINLINK_GAS_PRICE_MAX_WEI'])", '-e', "CHAINLINK_VRF_GAS_LIMIT=$($config['CHAINLINK_VRF_GAS_LIMIT'])", '-e', "POSTGRES_PASSWORD=$($secrets['PG_PASSWORD'])", '-e', "CHAINLINK_KEYSTORE_PASSWORD=$($secrets['CHAINLINK_KEYSTORE_PASSWORD'])", '-e', "CHAINLINK_API_EMAIL=$($secrets['CHAINLINK_API_EMAIL'])", '-e', "CHAINLINK_API_PASSWORD=$($secrets['CHAINLINK_API_PASSWORD'])", '-e', "CHAINLINK_TLS_CERT_B64=$($secrets['CHAINLINK_TLS_CERT_B64'])", '-e', "CHAINLINK_TLS_KEY_B64=$($secrets['CHAINLINK_TLS_KEY_B64'])", $chainlinkRef)
    if ($chainlinkResult.Code -ne 0) { throw "启动 Chainlink 节点失败: $($chainlinkResult.Text)" }
    $apiReady = $false
    $apiDeadline = [DateTime]::UtcNow.AddSeconds(180)
    while ([DateTime]::UtcNow -lt $apiDeadline) {
        $chainlinkState = Invoke-Docker @('inspect', '--format', '{{.State.Status}}|{{.State.ExitCode}}', $chainlink)
        if ($chainlinkState.Code -eq 0 -and $chainlinkState.Text -match '^exited\|([1-9][0-9]*)$') {
            throw "Chainlink 节点提前退出: $($chainlinkState.Text)`n$((Invoke-Docker @('logs', '--tail', '120', $chainlink)).Text)"
        }
        $apiProbe = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'sh', '--mount', ('source=' + $chainlinkVolume + ',target=/runtime-state,readonly'), $toolsRef, '-c', 'test -s /runtime-state/chainlink/tls/server.crt && curl --fail --cacert /runtime-state/chainlink/tls/server.crt --connect-timeout 2 --max-time 4 https://chainlink-node:6688/readyz >/dev/null')
        if ($apiProbe.Code -eq 0) { $apiReady = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $apiReady) { throw "Chainlink API 在限定时间内未就绪: $((Invoke-Docker @('logs', '--tail', '120', $chainlink)).Text)" }

    $sessionBody = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((@{ email = $secrets['CHAINLINK_API_EMAIL']; password = $secrets['CHAINLINK_API_PASSWORD'] } | ConvertTo-Json -Compress)))
    $apiSessionPrefix = 'set -eu; cp /runtime-state/chainlink/tls/server.crt /tmp/chainlink-ca.crt; printf "%s" "__SESSION_BODY__" | base64 -d >/tmp/session.json; session_code=$(curl --cacert /tmp/chainlink-ca.crt -sS -o /tmp/session.response -D /tmp/session.headers -c /tmp/session.cookies -w ''%{http_code}'' -H ''content-type: application/json'' --data-binary ''@/tmp/session.json'' https://chainlink-node:6688/sessions); test "$session_code" = 200; cookie_value=$(grep clsession /tmp/session.cookies | cut -f7); test -n "$cookie_value";'.Replace('__SESSION_BODY__', $sessionBody)
    $apiPrefix = $apiSessionPrefix + ' curl --cacert /tmp/chainlink-ca.crt -sS -b clsession=$cookie_value https://chainlink-node:6688'
    $invalidSessionBody = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((@{ email = $secrets['CHAINLINK_API_EMAIL']; password = 'invalid-chainlink-password' } | ConvertTo-Json -Compress)))
    $invalidAuthCommand = 'set -eu; cp /runtime-state/chainlink/tls/server.crt /tmp/chainlink-ca.crt; printf "%s" "__INVALID_SESSION_BODY__" | base64 -d >/tmp/invalid-session.json; code=$(curl --cacert /tmp/chainlink-ca.crt -sS -o /tmp/invalid-session.response -w ''%{http_code}'' --connect-timeout 2 --max-time 4 -H ''content-type: application/json'' --data-binary ''@/tmp/invalid-session.json'' https://chainlink-node:6688/sessions); case "$code" in 401|403) ;; *) printf ''invalid_auth_http_status=%s\n'' "$code"; exit 1 ;; esac; printf ''invalid_auth_rejected\n'''.Replace('__INVALID_SESSION_BODY__', $invalidSessionBody)
    $invalidAuthResult = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'sh', '--mount', ('source=' + $chainlinkVolume + ',target=/runtime-state,readonly'), $toolsRef, '-c', $invalidAuthCommand)
    if ($invalidAuthResult.Code -ne 0 -or $invalidAuthResult.Text -notmatch 'invalid_auth_rejected') { throw "Chainlink 错误鉴权未被拒绝: $($invalidAuthResult.Text)" }
    $vrfKey = (Invoke-ChainlinkApi $network $toolsRef $chainlinkVolume $secrets['CHAINLINK_API_EMAIL'] $secrets['CHAINLINK_API_PASSWORD'] 'create-vrf-key' ($apiSessionPrefix + ' curl --cacert /tmp/chainlink-ca.crt -sS -b clsession=$cookie_value -X POST https://chainlink-node:6688/v2/keys/vrf')) | ConvertFrom-Json
    $ethKeys = (Invoke-ChainlinkApi $network $toolsRef $chainlinkVolume $secrets['CHAINLINK_API_EMAIL'] $secrets['CHAINLINK_API_PASSWORD'] 'read-eth-keys' ($apiPrefix + '/v2/keys/eth')) | ConvertFrom-Json
    $ethKey = @($ethKeys.data | Where-Object { $_.attributes.address -match '^0x[0-9a-fA-F]{40}$' })[0]
    if ($vrfKey.data.attributes.uncompressed -notmatch '^0x[0-9a-fA-F]{128}$' -or $vrfKey.data.attributes.compressed -notmatch '^0x[0-9a-fA-F]{66}$' -or $null -eq $ethKey) {
        $vrfDataCount = @($vrfKey.data).Count
        $vrfUncompressedLength = [string]$vrfKey.data.attributes.uncompressed
        $vrfCompressedLength = [string]$vrfKey.data.attributes.compressed
        $ethDataCount = @($ethKeys.data).Count
        throw "Chainlink 节点未返回合法 VRF/EVM 密钥: vrf_data=$vrfDataCount vrf_uncompressed_length=$($vrfUncompressedLength.Length) vrf_compressed_length=$($vrfCompressedLength.Length) eth_data=$ethDataCount"
    }
    $oracleFunding = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'send', $ethKey.attributes.address, '--value', '5 ether', '--rpc-url', 'http://runtime:8545', '--private-key', $deployerKey, '--json')
    if ($oracleFunding.Code -ne 0) { throw "Chainlink EVM oracle 钱包注资失败: $($oracleFunding.Text)" }
    $oracleBalanceText = (Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'balance', $ethKey.attributes.address, '--rpc-url', 'http://runtime:8545')).Text.Trim()
    if ($oracleBalanceText -notmatch '^[0-9]+$' -or [System.Numerics.BigInteger]::Parse($oracleBalanceText) -lt [System.Numerics.BigInteger]::Parse('300000000000000000')) {
        throw "Chainlink EVM oracle 钱包余额不足: address=$($ethKey.attributes.address)"
    }

    $modulePath = Join-Path $repoRoot 'deploy\scripts\chainlink-material-export'
    $builderCommand = 'set -eu; export GOPATH=/runtime-state/go GOMODCACHE=/runtime-state/go/pkg/mod GOCACHE=/tmp/go-cache; cd /work; /usr/local/go/bin/go build -mod=readonly -o /tmp/chainlink-material-export .; /tmp/chainlink-material-export -rpc http://runtime:8545 -private-key "$DEPLOYER_PRIVATE_KEY" -chain-id "$CHAIN_ID" -vrf-public-key "$VRF_PUBLIC_KEY" -out /out/chainlink-vrf-material.json'
    $buildResult = Invoke-Docker -TimeoutSeconds 1200 @('run', '--rm', '--pull', 'never', '--user', '0:0', '--network', $network, '-v', ((Resolve-Path -LiteralPath $modulePath).Path + ':/work:ro'), '-v', ($official + ':/official/devenv:ro'), '-v', ($workRoot + ':/out'), '--mount', 'source=chaimir-composition-go-mod-cache,target=/runtime-state', '--mount', 'source=chaimir-composition-go-build-cache,target=/tmp/go-cache', '-e', "GOPROXY=$($config['GO_MODULE_PROXY'])", '-e', "DEPLOYER_PRIVATE_KEY=$deployerKey", '-e', "CHAIN_ID=$chainID", '-e', "VRF_PUBLIC_KEY=$($vrfKey.data.attributes.uncompressed)", $builderRef, 'sh', '-c', $builderCommand)
    if ($buildResult.Code -ne 0) { throw "Chainlink 官方 VRF 材料部署失败: $($buildResult.Text)" }
    $materialPath = Join-Path $workRoot 'chainlink-vrf-material.json'
    if (-not (Test-Path -LiteralPath $materialPath -PathType Leaf)) { throw 'Chainlink 官方 VRF 材料未生成' }
    $material = Get-Content -LiteralPath $materialPath -Raw | ConvertFrom-Json
    if ($material.coordinator -notmatch '^0x[0-9a-fA-F]{40}$' -or $material.consumer -notmatch '^0x[0-9a-fA-F]{40}$' -or $material.subscription_id -notmatch '^[1-9][0-9]*$') { throw 'Chainlink VRF 材料地址或 subscription 无效' }

    $observationSource = @'
decode_log [type=ethabidecodelog abi="RandomWordsRequested(bytes32 indexed keyHash,uint256 requestId,uint256 preSeed,uint256 indexed subId,uint16 minimumRequestConfirmations,uint32 callbackGasLimit,uint32 numWords,bytes extraArgs,address indexed sender)" data="$(jobRun.logData)" topics="$(jobRun.logTopics)"]
generate_proof [type=vrfv2plus publicKey="$(jobSpec.publicKey)" requestBlockHash="$(jobRun.logBlockHash)" requestBlockNumber="$(jobRun.logBlockNumber)" topics="$(jobRun.logTopics)"]
estimate_gas [type=estimategaslimit to="__COORDINATOR__" multiplier="1.1" data="$(generate_proof.output)"]
simulate_fulfillment [type=ethcall from="__ETH_ADDRESS__" to="__COORDINATOR__" gas="$(estimate_gas)" gasPrice="$(jobSpec.maxGasPrice)" extractRevertReason=true contract="__COORDINATOR__" data="$(generate_proof.output)"]
decode_log->generate_proof->estimate_gas->simulate_fulfillment
'@
    $observationSource = $observationSource.Replace('__COORDINATOR__', [string]$material.coordinator).Replace('__ETH_ADDRESS__', [string]$ethKey.attributes.address)
    $jobSpec = @'
type = "vrf"
schemaVersion = 1
name = "chaimir-vrf-v2-plus"
coordinatorAddress = "__COORDINATOR__"
batchCoordinatorAddress = "__BATCH_COORDINATOR__"
fromAddresses = ["__ETH_ADDRESS__"]
evmChainID = "__CHAIN_ID__"
minIncomingConfirmations = 1
publicKey = "__PUBLIC_KEY__"
externalJobID = "__JOB_ID__"
batchFulfillmentEnabled = false
batchFulfillmentGasMultiplier = 1.1
backoffInitialDelay = "15s"
backoffMaxDelay = "5m"
pollPeriod = "1s"
requestTimeout = "2m"
observationSource = """
__OBSERVATION_SOURCE__
"""
'@
    $jobSpec = $jobSpec.Replace('__COORDINATOR__', [string]$material.coordinator).Replace('__BATCH_COORDINATOR__', [string]$material.batch_coordinator).Replace('__ETH_ADDRESS__', [string]$ethKey.attributes.address).Replace('__CHAIN_ID__', [string]$chainID).Replace('__PUBLIC_KEY__', [string]$vrfKey.data.attributes.compressed).Replace('__JOB_ID__', [string][Guid]::NewGuid()).Replace('__OBSERVATION_SOURCE__', $observationSource)
    $jobBody = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((@{ toml = $jobSpec } | ConvertTo-Json -Compress)))
    $jobCommand = 'cp /runtime-state/chainlink/tls/server.crt /tmp/chainlink-ca.crt; printf "%s" "__JOB_BODY__" | base64 -d >/tmp/job.json; code=$(curl --cacert /tmp/chainlink-ca.crt -sS -o /tmp/job.response -w ''%{http_code}'' -b clsession=$cookie_value -H ''content-type: application/json'' --data-binary @/tmp/job.json https://chainlink-node:6688/v2/jobs); if test "$code" != 200 && test "$code" != 201; then printf ''job_http_status=%s '' "$code"; jq -c ''.errors // .'' /tmp/job.response; exit 1; fi; jq -r ''.data.id'' /tmp/job.response'
    $jobCommand = $apiSessionPrefix + ' ' + $jobCommand
    $jobID = Invoke-ChainlinkApi $network $toolsRef $chainlinkVolume $secrets['CHAINLINK_API_EMAIL'] $secrets['CHAINLINK_API_PASSWORD'] 'create-vrf-job' $jobCommand.Replace('__JOB_BODY__', $jobBody)
    if ($jobID -notmatch '^(?:[1-9][0-9]*|[0-9a-fA-F-]{20,})$') { throw "Chainlink VRF job 创建结果无效: $jobID" }

    $request = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'send', $material.consumer, 'requestRandomWords(uint256,uint16,bytes32,uint32,bool,uint32,uint16)', $material.subscription_id, '1', $material.vrf_key_hash, '2500000', 'true', '1', '1', '--rpc-url', 'http://runtime:8545', '--private-key', $deployerKey, '--json')
    if ($request.Code -ne 0) { throw "VRF consumer 真实请求失败: $($request.Text)" }
    $deployerAddress = (Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'wallet', 'address', '--private-key', $deployerKey)).Text.Trim()
    if ($deployerAddress -notmatch '^0x[0-9a-fA-F]{40}$') { throw "受控部署账户地址无效: $deployerAddress" }
    $responseReady = $false
    $responseDeadline = [DateTime]::UtcNow.AddSeconds(180)
    $nextHeadAdvance = [DateTime]::UtcNow
    $headAdvanceCount = 0
    while ([DateTime]::UtcNow -lt $responseDeadline) {
        if ([DateTime]::UtcNow -ge $nextHeadAdvance -and $headAdvanceCount -lt 30) {
            $advanceHead = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'send', $deployerAddress, '--value', '0', '--rpc-url', 'http://runtime:8545', '--private-key', $deployerKey, '--json')
            if ($advanceHead.Code -ne 0) { throw "推进受控链头的真实交易失败: $($advanceHead.Text)" }
            $headAdvanceCount++
            $nextHeadAdvance = [DateTime]::UtcNow.AddSeconds(2)
        }
        $count = Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'call', $material.consumer, 's_responseCount()(uint256)', '--rpc-url', 'http://runtime:8545')
        if ($count.Code -eq 0 -and $count.Text.Trim() -match '^[1-9][0-9]*$') { $responseReady = $true; break }
        Start-Sleep -Seconds 2
    }
    if (-not $responseReady) {
        $logText = (Invoke-Docker @('logs', '--tail', '500', $chainlink)).Text
        $diagnostic = (($logText -split "`r?`n") | Where-Object { $_ -match '(?i)latestFinalizedBlock|latestBlock=.*latestFinalized|confirmedAt|Pipeline error|No pending requests|proof reverted|fulfilled=' } | Select-Object -Last 80) -join "`n"
        $latestBlock = (Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'block', 'latest', '--rpc-url', 'http://runtime:8545', '--json')).Text
        $finalizedBlock = (Invoke-Docker @('run', '--rm', '--pull', 'never', '--network', $network, '--entrypoint', 'cast', $foundryRef, 'block', 'finalized', '--rpc-url', 'http://runtime:8545', '--json')).Text
    $configDiagnostic = (Invoke-ChainlinkApi $network $toolsRef $chainlinkVolume $secrets['CHAINLINK_API_EMAIL'] $secrets['CHAINLINK_API_PASSWORD'] 'diagnostic-config' ($apiSessionPrefix + ' curl --cacert /tmp/chainlink-ca.crt -sS -b clsession=$cookie_value https://chainlink-node:6688/v2/config | jq -r ''.data.attributes.config'' | grep -E ''FinalityDepth|FinalityTagEnabled|NewHeadsPollInterval'''))
        throw "Chainlink VRF job 未在限定时间内完成真实回调: latest_block=$latestBlock finalized_block=$finalizedBlock`nconfig=$configDiagnostic`n$diagnostic"
    }
    $material | Add-Member -NotePropertyName chainlink_job_id -NotePropertyValue $jobID.Trim().Trim('"')
    $material | Add-Member -NotePropertyName oracle_address -NotePropertyValue $ethKey.attributes.address
    $material | Add-Member -NotePropertyName consumer_response_count -NotePropertyValue 1
    $material | Add-Member -NotePropertyName source_runtime_digest -NotePropertyValue $lock[$RuntimeImage]
    $material | Add-Member -NotePropertyName chainlink_digest -NotePropertyValue $lock['infra/chainlink-node']
    $material | Add-Member -NotePropertyName postgres_digest -NotePropertyValue $lock['middleware/postgres']
    $generationSucceeded = $true
} finally {
    foreach ($name in @($chainlink, $runtime, $postgres)) { [void](Invoke-Docker @('rm', '-f', '-v', $name)) }
    [void](Invoke-Docker @('network', 'rm', $network))
    foreach ($volume in @($runtimeVolume, $chainlinkVolume, $postgresTLSVolume)) { [void](Invoke-Docker @('volume', 'rm', $volume)) }
    if (Test-Path -LiteralPath $workRoot) {
        try { Remove-Item -LiteralPath $workRoot -Recurse -Force }
        catch { $cleanupErrors.Add("临时目录未清理: $workRoot ($($_.Exception.Message))") }
    }

    $containerList = Invoke-Docker @('ps', '-a', '--format', '{{.Names}}')
    $networkList = Invoke-Docker @('network', 'ls', '--format', '{{.Name}}')
    $volumeList = Invoke-Docker @('volume', 'ls', '--format', '{{.Name}}')
    foreach ($result in @($containerList, $networkList, $volumeList)) {
        if ($result.Code -ne 0) { $cleanupErrors.Add("Docker 清理状态无法核验: $($result.Text)") }
    }
    if ($containerList.Code -eq 0) {
        $remaining = @($containerList.Text -split "`r?`n" | Where-Object { $_ -in @($chainlink, $runtime, $postgres) })
        if ($remaining.Count -gt 0) { $cleanupErrors.Add("容器仍存在: $($remaining -join ', ')") }
    }
    if ($networkList.Code -eq 0 -and $network -in @($networkList.Text -split "`r?`n")) {
        $cleanupErrors.Add("测试网络仍存在: $network")
    }
    if ($volumeList.Code -eq 0) {
        $remaining = @($volumeList.Text -split "`r?`n" | Where-Object { $_ -in @($runtimeVolume, $chainlinkVolume, $postgresTLSVolume) })
        if ($remaining.Count -gt 0) { $cleanupErrors.Add("测试卷仍存在: $($remaining -join ', ')") }
    }
    if ($cleanupErrors.Count -gt 0) { throw "Chainlink VRF 资源清理未通过: $($cleanupErrors -join '; ')" }
}

if (-not $generationSucceeded) { throw 'Chainlink VRF 真实回调未完成，拒绝输出材料' }
$material | Add-Member -NotePropertyName verified -NotePropertyValue ([ordered]@{
    vrf_request_fulfilled = $true
    api_authentication = $true
    candidate_digests_match = $true
    cleanup = $true
}) -Force
[IO.File]::WriteAllText($output, (($material | ConvertTo-Json -Depth 8) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
Write-Output "受控 Chainlink VRF 材料、真实回调和精确清理均已验证: $output"
