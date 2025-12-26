#!/bin/bash
set -e
cd "$(dirname "$0")/.."
source .env

# Path to the op-proposer binary we built
OP_PROPOSER_BIN="../../op-proposer/bin/op-proposer"

$OP_PROPOSER_BIN \
  --poll-interval=$POLL_INTERVAL \
  --rpc.port=$PROPOSER_RPC_PORT \
  --rpc.enable-admin \
  --rollup-rpc=$ROLLUP_RPC_URL \
  --l1-eth-rpc=$L1_RPC_URL \
  --private-key=$PRIVATE_KEY \
  --game-factory-address=$GAME_FACTORY_ADDRESS \
  --game-type=$GAME_TYPE \
  --proposal-interval=$PROPOSAL_INTERVAL \
  --num-confirmations=1 \
  --resubmission-timeout=30s \
  --wait-node-sync=false \
  --allow-non-finalized=true \
  --log.level=info \
  --log.format=json
