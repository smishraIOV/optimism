#!/bin/bash
set -e
cd "$(dirname "$0")/.."
source .env

OP_BATCHER_BIN="../../op-batcher/bin/op-batcher"

$OP_BATCHER_BIN \
  --l1-eth-rpc=$L1_RPC_URL \
  --l2-eth-rpc=$L2_RPC_URL \
  --rollup-rpc=$ROLLUP_RPC_URL \
  --poll-interval=$POLL_INTERVAL \
  --sub-safety-margin=$SUB_SAFETY_MARGIN \
  --num-confirmations=$NUM_CONFIRMATIONS \
  --safe-abort-nonce-too-low-count=$SAFE_ABORT_NONCE_TOO_LOW_COUNT \
  --resubmission-timeout=$RESUBMISSION_TIMEOUT \
  --rpc.addr=0.0.0.0 \
  --rpc.port=$BATCHER_RPC_PORT \
  --rpc.enable-admin \
  --max-channel-duration=$MAX_CHANNEL_DURATION \
  --private-key=$BATCHER_PRIVATE_KEY \
  --data-availability-type=calldata \
  --log.level=info \
  --log.format=json

