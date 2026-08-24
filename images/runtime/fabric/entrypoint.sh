#!/usr/bin/env bash
# 本脚本初始化并启动单组织 Fabric 教学网络,身份和账本只保存在 runtime-state。
set -euo pipefail

if [[ "$#" -gt 0 ]]; then
  exec "$@"
fi

ROOT=/runtime-state/fabric
CRYPTO_ROOT=${ROOT}/crypto
CONFIG_ROOT=${ROOT}/config
LEDGER_ROOT=${ROOT}/ledger
CHANNEL_NAME=${CHAIMIR_FABRIC_CHANNEL:-chaimir}
mkdir -p "${ROOT}" "${CONFIG_ROOT}" "${LEDGER_ROOT}" "${LEDGER_ROOT}/orderer/etcdraft/wal" "${LEDGER_ROOT}/orderer/etcdraft/snapshot"
cp /etc/hyperledger/fabric/core.yaml /etc/hyperledger/fabric/orderer.yaml "${CONFIG_ROOT}/"
sed -i "s#/var/hyperledger/production#${LEDGER_ROOT}#g" "${CONFIG_ROOT}/core.yaml"

if [[ ! -f "${ROOT}/.initialized" ]]; then
  cat >"${ROOT}/crypto-config.yaml" <<'EOF'
OrdererOrgs:
  - Name: Orderer
    Domain: chaimir.local
    Specs:
      - Hostname: orderer
PeerOrgs:
  - Name: Org1
    Domain: org1.chaimir.local
    EnableNodeOUs: true
    Template:
      Count: 1
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
  OrdererType: solo
  Addresses:
    - 127.0.0.1:7050
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
  touch "${ROOT}/.initialized"
fi

export FABRIC_CFG_PATH="${CONFIG_ROOT}"
export ORDERER_GENERAL_LISTENADDRESS=0.0.0.0
export ORDERER_GENERAL_LISTENPORT=7050
export ORDERER_GENERAL_GENESISMETHOD=file
export ORDERER_GENERAL_GENESISFILE="${ROOT}/genesis.block"
export ORDERER_GENERAL_LOCALMSPID=OrdererMSP
export ORDERER_GENERAL_LOCALMSPDIR="${CRYPTO_ROOT}/ordererOrganizations/chaimir.local/orderers/orderer.chaimir.local/msp"
export ORDERER_GENERAL_TLS_ENABLED=false
export ORDERER_FILELEDGER_LOCATION="${LEDGER_ROOT}/orderer"
export ORDERER_CONSENSUS_WALDIR="${LEDGER_ROOT}/orderer/etcdraft/wal"
export ORDERER_CONSENSUS_SNAPDIR="${LEDGER_ROOT}/orderer/etcdraft/snapshot"
export CORE_PEER_ID=peer0.org1.chaimir.local
export CORE_PEER_ADDRESS=127.0.0.1:7051
export CORE_PEER_LISTENADDRESS=0.0.0.0:7051
export CORE_PEER_CHAINCODEADDRESS=127.0.0.1:7052
export CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:7052
export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_MSPCONFIGPATH="${CRYPTO_ROOT}/peerOrganizations/org1.chaimir.local/users/Admin@org1.chaimir.local/msp"
export CORE_PEER_FILESYSTEMPATH="${LEDGER_ROOT}/peer"
export CORE_PEER_GOSSIP_EXTERNALENDPOINT=127.0.0.1:7051
export CORE_PEER_TLS_ENABLED=false
export CORE_OPERATIONS_LISTENADDRESS=127.0.0.1:9444

orderer >"${ROOT}/orderer.log" 2>&1 &
orderer_pid=$!
peer node start >"${ROOT}/peer.log" 2>&1 &
peer_pid=$!
trap 'kill "${peer_pid}" "${orderer_pid}" 2>/dev/null || true' EXIT INT TERM

for attempt in $(seq 1 30); do
  if peer channel create -o 127.0.0.1:7050 -c "${CHANNEL_NAME}" -f "${ROOT}/channel.tx" --outputBlock "${ROOT}/channel.block" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "${orderer_pid}" 2>/dev/null || ! kill -0 "${peer_pid}" 2>/dev/null; then
    cat "${ROOT}/orderer.log" "${ROOT}/peer.log" >&2
    exit 1
  fi
  sleep 1
done

if [[ ! -s "${ROOT}/channel.block" ]]; then
  cat "${ROOT}/orderer.log" "${ROOT}/peer.log" >&2
  exit 1
fi
peer channel join -b "${ROOT}/channel.block" >/dev/null
wait "${peer_pid}"
