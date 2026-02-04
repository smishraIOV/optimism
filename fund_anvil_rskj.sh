#!/bin/bash

# Start rskj
# java -Drpc.providers.web.http.port=8545 \
#   -Drpc.providers.web.http.bind_address=0.0.0.0 \
#   -cp rskj-core/build/libs/rskj-core-8.2.0-SNAPSHOT-all.jar \
#   co.rsk.Start --regtest --reset



# Default "cow" account details (from rsk-dev.json)
SENDER_PRIV_KEY="c85ef7d79691fe79573b1a7064c19c1a9819ebdbd1faaab1a8ec92344438aaf4"
RPC_URL="http://localhost:8545"
AMOUNT="100000000000000000000" # 100 ether in wei (hex or dec)

# anvil seed 2 accounts
ADDRESSES=(
    "0x8995E44a22e303A79bdD2E6e41674fb92d620863"
    "0xE9e05C9f02e10FA833D379CB1c7aC3a3f23B247e"
    "0x61Da7c7F97EBE53AD7c4E5eCD3d117E7Ab430eA7"
    "0x5b0248e30583CeD4F09726C547935552C469EB24"
    "0xcDbc8abb83E01BaE13ECE8853a5Ca84b2Ef6Ca86"
    "0xa683a3E33E07fb84ff33FcE753Da1d248298977f"
    "0x008099bFee75e832e1b93D4c023f646d99d4C90f"
    "0x38aDCae107e9aEd4C6dfFA317d651E80CCCE0857"
)

echo "Funding ${#ADDRESSES[@]} accounts with $AMOUNT wei each..."

for ADDR in "${ADDRESSES[@]}"; do
    echo "Sending to $ADDR..."
    # --legacy: Use legacy gas price (RSK doesn't support EIP-1559)
    # --gas-price 60000000: Manual gas price to avoid estimation if it's failing
    cast send --private-key "$SENDER_PRIV_KEY" --rpc-url "$RPC_URL" "$ADDR" --value "$AMOUNT" --legacy --gas-price 60000000
done

echo "All accounts funded!"

echo "Deploying Deterministic Deployer for create2"

cast send --private-key "c85ef7d79691fe79573b1a7064c19c1a9819ebdbd1faaab1a8ec92344438aaf4" \
  --rpc-url "http://localhost:8545" \
  0x3fab184622dc19b6109349b94811493bf2a45362 \
  --value 100000000000000000 \
  --legacy \
  --gas-price 60000000

echo "Broadcasting transaction to deploy Deterministic Deployer for create2"

cast publish --rpc-url http://localhost:8545 \
0xf8a58085174876e800830186a08080b853604580600e600039806000f350fe7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf31ba02222222222222222222222222222222222222222222222222222222222222222a02222222222222222222222222222222222222222222222222222222222222222

echo "verify deployer code"

cast code --rpc-url http://localhost:8545 0x4e59b44847b379578588920cA78FbF26c0B4956C