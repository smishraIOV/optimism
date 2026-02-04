#!/bin/bash
set -e
cd "$(dirname "$0")/.."
source .env

OP_NODE_BIN="../../bin/op-node"

$OP_NODE_BIN \
  --l1=$L1_RPC_URL \
  --l1.rpckind=rsk \
  --l1.trustrpc=true \
  --l1.beacon.ignore=true \
  --l2=http://localhost:$OP_GETH_AUTH_PORT \
  --l2.jwt-secret=$JWT_SECRET \
  --rollup.config=./rollup.json \
  --rollup.l1-chain-config=./l1-chain-config.json \
  --sequencer.enabled=$SEQUENCER_ENABLED \
  --sequencer.stopped=$SEQUENCER_STOPPED \
  --sequencer.max-safe-lag=3600 \
  --verifier.l1-confs=0 \
  --p2p.listen.ip=0.0.0.0 \
  --p2p.listen.tcp=$P2P_LISTEN_PORT \
  --p2p.listen.udp=$P2P_LISTEN_PORT \
  --p2p.advertise.ip=$P2P_ADVERTISE_IP \
  --p2p.advertise.tcp=$P2P_LISTEN_PORT \
  --p2p.advertise.udp=$P2P_LISTEN_PORT \
  --p2p.sequencer.key=$PRIVATE_KEY \
  --rpc.addr=0.0.0.0 \
  --rpc.port=$OP_NODE_RPC_PORT \
  --rpc.enable-admin \
  --log.level=info \
  --log.format=json
