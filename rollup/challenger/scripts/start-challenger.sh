#!/bin/bash

source .env

# Create data directory if it doesn't exist
mkdir -p $DATADIR

# Path to the challenger binary (from rollup/challenger/)
# Note: Using L1 RPC as beacon endpoint since Anvil doesn't have a real beacon
../../op-challenger/bin/op-challenger \
  --trace-type permissioned,cannon \
  --l1-eth-rpc=$L1_RPC_URL \
  --l1-beacon=$L1_RPC_URL \
  --l2-eth-rpc=$L2_RPC_URL \
  --rollup-rpc=$ROLLUP_RPC_URL \
  --game-factory-address=$GAME_FACTORY_ADDRESS \
  --datadir=$DATADIR \
  --cannon-bin=$CANNON_BIN \
  --cannon-rollup-config=$CANNON_ROLLUP_CONFIG \
  --cannon-l2-genesis=$CANNON_L2_GENESIS \
  --cannon-server=$CANNON_SERVER \
  --cannon-prestate=$CANNON_PRESTATE \
  --private-key=$PRIVATE_KEY \
  --log.level=info
