#!/usr/bin/env bash
# 本脚本初始化并启动带 TLS 的单组织 Fabric 教学网络,身份和账本只保存在 runtime-state。
set -euo pipefail

if [[ "$#" -gt 0 ]]; then
  exec "$@"
fi

ROOT=/runtime-state/fabric
CRYPTO_ROOT=${ROOT}/crypto
CONFIG_ROOT=${ROOT}/config
LEDGER_ROOT=${ROOT}/ledger
CHANNEL_CREATE_LOG=${ROOT}/channel-create.log
CHANNEL_JOIN_LOG=${ROOT}/channel-join.log
SECURITY_PROFILE=${ROOT}/security-profile
: "${CHAIMIR_FABRIC_CHANNEL:?CHAIMIR_FABRIC_CHANNEL is required}"
CHANNEL_NAME=${CHAIMIR_FABRIC_CHANNEL}
umask 077
mkdir -p "${ROOT}" "${CONFIG_ROOT}" "${LEDGER_ROOT}" "${LEDGER_ROOT}/orderer/etcdraft/wal" "${LEDGER_ROOT}/orderer/etcdraft/snapshot"
cp /etc/hyperledger/fabric/core.yaml /etc/hyperledger/fabric/orderer.yaml "${CONFIG_ROOT}/"
sed -i "s#/var/hyperledger/production#${LEDGER_ROOT}#g" "${CONFIG_ROOT}/core.yaml"

if [[ -f "${ROOT}/.initialized" && ! -f "${SECURITY_PROFILE}" ]]; then
  echo "Fabric runtime-state 缺少安全配置标记,拒绝复用旧状态" >&2
  exit 1
fi
if [[ -f "${SECURITY_PROFILE}" ]] && [[ "$(<"${SECURITY_PROFILE}")" != "tls-etcdraft-v1" ]]; then
  echo "Fabric runtime-state 安全配置版本不受支持,拒绝复用" >&2
  exit 1
fi

if [[ ! -f "${ROOT}/.initialized" ]]; then
  cat >"${ROOT}/crypto-config.yaml" <<'EOF'
OrdererOrgs:
  - Name: Orderer
    Domain: chaimir.local
    Specs:
      - Hostname: orderer
        SANS:
          - "127.0.0.1"
PeerOrgs:
  - Name: Org1
    Domain: org1.chaimir.local
    EnableNodeOUs: true
    Template:
      Count: 1
    Specs:
      - Hostname: peer0
        SANS:
          - "127.0.0.1"
    Users:
      Count: 1
EOF
  cryptogen generate --config="${ROOT}/crypto-config.yaml" --output="${CRYPTO_ROOT}"
  cat >"${CONFIG_ROOT}/configtx.yaml" <<EOF
Organizations:
  - &OrdererOrg
    Name: OrdererOrg
    ID: OrdererMSP
    MSPDir: ${CRYPTO_ROOT}/ordererOrganizations/chaimir.local/msp
    Policies:
      Readers:
        Type: Signature
        Rule: "OR('OrdererMSP.member')"
      Writers:
        Type: Signature
        Rule: "OR('OrdererMSP.member')"
      Admins:
        Type: Signature
        Rule: "OR('OrdererMSP.admin')"
  - &Org1
    Name: Org1MSP
    ID: Org1MSP
    MSPDir: ${CRYPTO_ROOT}/peerOrganizations/org1.chaimir.local/msp
    Policies:
      Readers:
        Type: Signature
        Rule: "OR('Org1MSP.member')"
      Writers:
        Type: Signature
        Rule: "OR('Org1MSP.member')"
      Admins:
        Type: Signature
        Rule: "OR('Org1MSP.admin')"

Capabilities:
  Channel: &ChannelCapabilities
    V2_0: true
  Orderer: &OrdererCapabilities
    V2_0: true
  Application: &ApplicationCapabilities
    V2_0: true

Application: &ApplicationDefaults
  Organizations:
  Policies:
    Readers:
      Type: ImplicitMeta
      Rule: ANY Readers
    Writers:
      Type: ImplicitMeta
      Rule: ANY Writers
    Admins:
      Type: ImplicitMeta
      Rule: MAJORITY Admins
  Capabilities:
    <<: *ApplicationCapabilities

Orderer: &OrdererDefaults
  OrdererType: etcdraft
  Addresses:
    - 127.0.0.1:7050
  EtcdRaft:
    Consenters:
      - Host: 127.0.0.1
        Port: 7050
        ClientTLSCert: ${CRYPTO_ROOT}/ordererOrganizations/chaimir.local/orderers/orderer.chaimir.local/tls/server.crt
        ServerTLSCert: ${CRYPTO_ROOT}/ordererOrganizations/chaimir.local/orderers/orderer.chaimir.local/tls/server.crt
  BatchTimeout: 2s
  BatchSize:
    MaxMessageCount: 10
    AbsoluteMaxBytes: 99 MB
    PreferredMaxBytes: 512 KB
  Organizations:
  Policies:
    Readers:
      Type: ImplicitMeta
      Rule: ANY Readers
    Writers:
      Type: ImplicitMeta
      Rule: ANY Writers
    Admins:
      Type: ImplicitMeta
      Rule: MAJORITY Admins
    BlockValidation:
      Type: ImplicitMeta
      Rule: ANY Writers
  Capabilities:
    <<: *OrdererCapabilities

Channel: &ChannelDefaults
  Policies:
    Readers:
      Type: ImplicitMeta
      Rule: ANY Readers
    Writers:
      Type: ImplicitMeta
      Rule: ANY Writers
    Admins:
      Type: ImplicitMeta
      Rule: MAJORITY Admins
  Capabilities:
    <<: *ChannelCapabilities

Profiles:
  OneOrgOrdererGenesis:
    <<: *ChannelDefaults
    Orderer:
      <<: *OrdererDefaults
      Organizations:
        - *OrdererOrg
    Consortiums:
      SampleConsortium:
        Organizations:
          - *Org1
  OneOrgChannel:
    Consortium: SampleConsortium
    <<: *ChannelDefaults
    Application:
      <<: *ApplicationDefaults
      Organizations:
        - *Org1
EOF
  export FABRIC_CFG_PATH="${CONFIG_ROOT}"
  configtxgen -profile OneOrgOrdererGenesis -channelID system-channel -outputBlock "${ROOT}/genesis.block"
  configtxgen -profile OneOrgChannel -channelID "${CHANNEL_NAME}" -outputCreateChannelTx "${ROOT}/channel.tx"
  printf '%s' 'tls-etcdraft-v1' >"${SECURITY_PROFILE}"
  touch "${ROOT}/.initialized"
fi

ORDERER_TLS_ROOT_CA=${CRYPTO_ROOT}/ordererOrganizations/chaimir.local/orderers/orderer.chaimir.local/tls/ca.crt
ORDERER_TLS_CERT=${CRYPTO_ROOT}/ordererOrganizations/chaimir.local/orderers/orderer.chaimir.local/tls/server.crt
ORDERER_TLS_KEY=${CRYPTO_ROOT}/ordererOrganizations/chaimir.local/orderers/orderer.chaimir.local/tls/server.key
ORDERER_TLS_ORG_ROOT_CA=${CRYPTO_ROOT}/ordererOrganizations/chaimir.local/tlsca/tlsca.chaimir.local-cert.pem
ORDERER_TLS_CLIENT_CERT=${CRYPTO_ROOT}/ordererOrganizations/chaimir.local/users/Admin@chaimir.local/tls/client.crt
ORDERER_TLS_CLIENT_KEY=${CRYPTO_ROOT}/ordererOrganizations/chaimir.local/users/Admin@chaimir.local/tls/client.key
PEER_TLS_ROOT_CA=${CRYPTO_ROOT}/peerOrganizations/org1.chaimir.local/peers/peer0.org1.chaimir.local/tls/ca.crt
PEER_TLS_CERT=${CRYPTO_ROOT}/peerOrganizations/org1.chaimir.local/peers/peer0.org1.chaimir.local/tls/server.crt
PEER_TLS_KEY=${CRYPTO_ROOT}/peerOrganizations/org1.chaimir.local/peers/peer0.org1.chaimir.local/tls/server.key
PEER_TLS_CLIENT_CERT=${CRYPTO_ROOT}/peerOrganizations/org1.chaimir.local/users/Admin@org1.chaimir.local/tls/client.crt
PEER_TLS_CLIENT_KEY=${CRYPTO_ROOT}/peerOrganizations/org1.chaimir.local/users/Admin@org1.chaimir.local/tls/client.key
ORG_TLS_ROOT_CA=${CRYPTO_ROOT}/peerOrganizations/org1.chaimir.local/tlsca/tlsca.org1.chaimir.local-cert.pem
for required_file in "${ORDERER_TLS_ROOT_CA}" "${ORDERER_TLS_ORG_ROOT_CA}" "${ORDERER_TLS_CERT}" "${ORDERER_TLS_KEY}" "${ORDERER_TLS_CLIENT_CERT}" "${ORDERER_TLS_CLIENT_KEY}" "${PEER_TLS_ROOT_CA}" "${PEER_TLS_CERT}" "${PEER_TLS_KEY}" "${PEER_TLS_CLIENT_CERT}" "${PEER_TLS_CLIENT_KEY}" "${ORG_TLS_ROOT_CA}"; do
  [[ -r "${required_file}" ]] || { echo "Fabric TLS material missing: ${required_file}" >&2; exit 1; }
done
find "${CRYPTO_ROOT}" -type f \( -name 'priv_sk' -o -name '*.key' \) -exec chmod 0600 {} +

export FABRIC_CFG_PATH="${CONFIG_ROOT}"
export ORDERER_GENERAL_LISTENADDRESS=0.0.0.0
export ORDERER_GENERAL_LISTENPORT=7050
export ORDERER_GENERAL_GENESISMETHOD=file
export ORDERER_GENERAL_GENESISFILE="${ROOT}/genesis.block"
export ORDERER_GENERAL_LOCALMSPID=OrdererMSP
export ORDERER_GENERAL_LOCALMSPDIR="${CRYPTO_ROOT}/ordererOrganizations/chaimir.local/orderers/orderer.chaimir.local/msp"
export ORDERER_GENERAL_TLS_ENABLED=true
export ORDERER_GENERAL_TLS_PRIVATEKEY="${ORDERER_TLS_KEY}"
export ORDERER_GENERAL_TLS_CERTIFICATE="${ORDERER_TLS_CERT}"
export ORDERER_GENERAL_TLS_ROOTCAS="[${ORDERER_TLS_ORG_ROOT_CA}]"
export ORDERER_GENERAL_TLS_CLIENTAUTHREQUIRED=true
export ORDERER_GENERAL_TLS_CLIENTROOTCAS="[${ORDERER_TLS_ORG_ROOT_CA}]"
export ORDERER_GENERAL_CLUSTER_CLIENTCERTIFICATE="${ORDERER_TLS_CERT}"
export ORDERER_GENERAL_CLUSTER_CLIENTPRIVATEKEY="${ORDERER_TLS_KEY}"
export ORDERER_GENERAL_CLUSTER_ROOTCAS="[${ORDERER_TLS_ORG_ROOT_CA}]"
export ORDERER_FILELEDGER_LOCATION="${LEDGER_ROOT}/orderer"
export ORDERER_CONSENSUS_WALDIR="${LEDGER_ROOT}/orderer/etcdraft/wal"
export ORDERER_CONSENSUS_SNAPDIR="${LEDGER_ROOT}/orderer/etcdraft/snapshot"
export CORE_PEER_ID=peer0.org1.chaimir.local
# peer 对外通过编排器创建的 Service 暴露；进程自身必须监听容器所有地址。
export CORE_PEER_ADDRESS=0.0.0.0:7051
export CORE_PEER_LISTENADDRESS=0.0.0.0:7051
export CORE_PEER_CHAINCODEADDRESS=127.0.0.1:7052
export CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:7052
export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_MSPCONFIGPATH="${CRYPTO_ROOT}/peerOrganizations/org1.chaimir.local/users/Admin@org1.chaimir.local/msp"
export CORE_PEER_FILESYSTEMPATH="${LEDGER_ROOT}/peer"
# 单节点运行时不声明不可达的回环 gossip 地址；跨容器访问由 binding 注入 Service DNS。
unset CORE_PEER_GOSSIP_EXTERNALENDPOINT
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_TLS_CERT_FILE="${PEER_TLS_CERT}"
export CORE_PEER_TLS_KEY_FILE="${PEER_TLS_KEY}"
export CORE_PEER_TLS_ROOTCERT_FILE="${PEER_TLS_ROOT_CA}"
export CORE_PEER_TLS_CLIENTAUTHREQUIRED=true
export CORE_PEER_TLS_CLIENTROOTCAS_FILES="${ORG_TLS_ROOT_CA}"
export CORE_PEER_TLS_CLIENTCERT_FILE="${PEER_TLS_CLIENT_CERT}"
export CORE_PEER_TLS_CLIENTKEY_FILE="${PEER_TLS_CLIENT_KEY}"
export CORE_PEER_TLS_SERVERHOSTOVERRIDE=peer0
export CORE_OPERATIONS_LISTENADDRESS=127.0.0.1:9444

orderer >"${ROOT}/orderer.log" 2>&1 &
orderer_pid=$!
peer node start >"${ROOT}/peer.log" 2>&1 &
peer_pid=$!
cleanup_processes() {
  local cleanup_status=0
  for pid in "${peer_pid}" "${orderer_pid}"; do
    if kill -0 "${pid}" 2>/dev/null; then
      if ! kill "${pid}" 2>/dev/null; then
        echo "Fabric 子进程终止失败: pid=${pid}" >&2
        cleanup_status=1
      fi
    fi
  done
  return "${cleanup_status}"
}
trap cleanup_processes EXIT INT TERM

for attempt in $(seq 1 30); do
  # orderer 就绪后 channel 创建可能立即完成,而 peer 仍在打开账本。
  # 创建和加入必须处于同一个有界状态机中,只重试尚未就绪的操作。
  if [[ ! -s "${ROOT}/channel.block" ]]; then
    if ! CORE_PEER_ADDRESS=127.0.0.1:7051 peer channel create -o 127.0.0.1:7050 -c "${CHANNEL_NAME}" -f "${ROOT}/channel.tx" --outputBlock "${ROOT}/channel.block" --tls --cafile "${ORDERER_TLS_ROOT_CA}" --clientauth --certfile "${ORDERER_TLS_CLIENT_CERT}" --keyfile "${ORDERER_TLS_CLIENT_KEY}" --ordererTLSHostnameOverride orderer >>"${CHANNEL_CREATE_LOG}" 2>&1; then
      echo "channel create 尚未就绪,将在有界重试中继续" >>"${CHANNEL_CREATE_LOG}"
    fi
  fi
  if [[ -s "${ROOT}/channel.block" ]] && CORE_PEER_ADDRESS=127.0.0.1:7051 peer channel join -b "${ROOT}/channel.block" >>"${CHANNEL_JOIN_LOG}" 2>&1; then
    break
  fi
  sleep 1
done

if [[ ! -s "${ROOT}/channel.block" ]] || ! kill -0 "${orderer_pid}" 2>/dev/null || ! kill -0 "${peer_pid}" 2>/dev/null; then
  cat "${CHANNEL_CREATE_LOG}" "${CHANNEL_JOIN_LOG}" "${ROOT}/orderer.log" "${ROOT}/peer.log" >&2
  exit 1
fi
wait "${peer_pid}"
