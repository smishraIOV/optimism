#!/bin/bash
set -e
cd "$(dirname "$0")/.."
source .env

OP_GETH_BIN="../op-geth/build/bin/geth"

$OP_GETH_BIN \
  --datadir=./op-geth-data \
  --http \
  --http.addr=0.0.0.0 \
  --http.port=$OP_GETH_HTTP_PORT \
  --http.vhosts="*" \
  --http.corsdomain="*" \
  --http.api=eth,net,web3,debug,txpool,admin,miner,personal \
  --rpc.enabledeprecatedpersonal \
  --ws \
  --ws.addr=0.0.0.0 \
  --ws.port=$OP_GETH_WS_PORT \
  --ws.origins="*" \
  --ws.api=eth,net,web3,debug,txpool,admin,miner,personal \
  --authrpc.addr=0.0.0.0 \
  --authrpc.port=$OP_GETH_AUTH_PORT \
  --authrpc.vhosts="*" \
  --authrpc.jwtsecret=$JWT_SECRET \
  --syncmode=full \
  --gcmode=archive \
  --rollup.disabletxpoolgossip=true
