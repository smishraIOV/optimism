# L2 Rollup Deployment Log

Following the tutorial: https://docs.optimism.io/chain-operators/tutorials/create-l2-rollup/op-deployer-setup

## Progress Summary

| Step | Status | Notes |
|------|--------|-------|
| 1. Install op-deployer | ✅ Complete | Built from source |
| 2. Create directory structure | ✅ Complete | `rollup/deployer`, `rollup/sequencer` |
| 3. Start Anvil (L1) | ✅ Complete | `--mnemonic-seed-unsafe 2 --hardfork cancun --block-time 15` |
| 4. Initialize and configure intent | ✅ Complete | Custom intent type for local chain |
| 5. Deploy L1 contracts | ✅ Complete | ~82M gas, 37 transactions |
| 6. Generate and modify chain config | ✅ Complete | Disable future hardforks for Anvil |
| 7. Build op-node and op-geth | ✅ Complete | op-geth v1.101511.1 |
| 8. Configure and initialize sequencer | ✅ Complete | JWT, l1-chain-config, scripts |
| 9. Start sequencer | ✅ Complete | op-geth + op-node producing blocks |
| 10. Spin up batcher | ✅ Complete | Posts L2 data to L1 via calldata |
| 11. Spin up proposer | ✅ Complete | Submits state roots to DisputeGameFactory |
| 12. Spin up challenger | ✅ Complete | Monitors dispute games (limited on Anvil) |

---

## Step 1: Install op-deployer (Build from Source)

### 1.1 Clone the Optimism repository

```bash
git clone https://github.com/ethereum-optimism/optimism.git
cd optimism
```

### 1.2 Initialize git submodules for contracts-bedrock

```bash
git submodule update --init packages/contracts-bedrock/lib/
```

This is required because the Solidity dependencies (solady, solmate, openzeppelin-contracts, etc.) are managed as git submodules.

### 1.3 Disable linting during build

⚠️ **REQUIRED CHANGE**: Add the following to `packages/contracts-bedrock/foundry.toml` (before the `[fmt]` section):

```toml
[lint]
lint_on_build = false
```

**Reason:** Forge 1.5.0+ has linting during builds. The codebase has warnings that cause build failure when `deny_warnings = true`.

### 1.4 Build op-deployer

```bash
cd op-deployer
just build
cd ..
```

Binary location: `op-deployer/bin/op-deployer`

### 1.5 Verify installation

```bash
./op-deployer/bin/op-deployer --version
```

---

## Step 2: Create Directory Structure

Create the rollup directory structure (from repo root):

```bash
mkdir -p rollup/deployer
mkdir -p rollup/sequencer/scripts
```

---

## Step 3: Start Local L1 Network (Anvil)

**Terminal 1:**

```bash
anvil --mnemonic-seed-unsafe 2 --hardfork cancun --block-time 15
```

| Flag | Purpose |
|------|---------|
| `--mnemonic-seed-unsafe 2` | Deterministic accounts for reproducibility |
| `--hardfork cancun` | Required for Ecotone/blob support |
| `--block-time 15` | Interval mining - similar to mainnet (~12s), ensures L2 can keep up |

> **Tip:** To pause/resume interval mining (e.g., to let L2 catch up):
> - Pause: `cast rpc evm_setIntervalMining 0 --rpc-url http://localhost:8545`
> - Resume: `cast rpc evm_setIntervalMining 2 --rpc-url http://localhost:8545`

**RPC URL:** `http://localhost:8545`
**Chain ID:** `31337`

### Anvil Seed 2 Accounts

We use these pre-funded accounts for all rollup roles:

| Index | Address | Private Key |
|-------|---------|-------------|
| 0 | `0x8995E44a22e303A79bdD2E6e41674fb92d620863` | `0xd6a036f561e03196779dd34bf3d141dec4737eec5ed0416e413985ca05dad51a` |
| 1 | `0xE9e05C9f02e10FA833D379CB1c7aC3a3f23B247e` | `0xbe62250c9db006c67c1595ff1f019bc849e2aa5c092dea0bf00883b39e54e904` |
| 2 | `0x61Da7c7F97EBE53AD7c4E5eCD3d117E7Ab430eA7` | `0xc3bae29d211b5523ccdc349e8275cc57a291a03558be6f6ec799c196702ef881` |
| 3 | `0x5b0248e30583CeD4F09726C547935552C469EB24` | `0x4bc19d3b0467a84723ad48118ed28884105526458d5a64a716ebea568318e3d0` |
| 4 | `0xcDbc8abb83E01BaE13ECE8853a5Ca84b2Ef6Ca86` | `0x07127c605875395527bccbc5c74afa9dc1712d83ba5b12207bab86d3b7fa8be6` |
| 5 | `0xa683a3E33E07fb84ff33FcE753Da1d248298977f` | `0x309bc84a97ca76f0c15bc3a6c98d46d8fb2381c8bd63ba6cc7b0d25ff4a13332` |
| 6 | `0x008099bFee75e832e1b93D4c023f646d99d4C90f` | `0x3d4471ad1080f3c193b0a1f299ad78e3b9d7119c2bc841b17fa3317a627f2818` |
| 7 | `0x38aDCae107e9aEd4C6dfFA317d651E80CCCE0857` | `0xf583a7d3a6cfe88ea61f0d3ab3ef5dc167636defe2b6f8c8f06c8981267ba401` |

**Mnemonic:** `cabbage measure motor lazy return bind siren again diesel slight bike shock`




---

## Step 4: Initialize and Configure Intent File

**From `rollup/deployer` directory:**

```bash
cd rollup/deployer

../../op-deployer/bin/op-deployer init \
  --intent-type custom \
  --workdir .deployer \
  --l1-chain-id 31337 \
  --l2-chain-ids 42069
```

**Note:** Must use `--intent-type custom` for local Anvil chain (no pre-deployed OPCM on chain ID 31337).

### Configure intent.toml

Edit `.deployer/intent.toml` to set the Anvil seed 2 addresses:

```toml
configType = "custom"
l1ChainID = 31337
fundDevAccounts = false
l1ContractsLocator = "embedded"
l2ContractsLocator = "embedded"

[superchainRoles]
  SuperchainProxyAdminOwner = "0x8995e44a22e303a79bdd2e6e41674fb92d620863"
  SuperchainGuardian = "0x8995e44a22e303a79bdd2e6e41674fb92d620863"
  ProtocolVersionsOwner = "0x8995e44a22e303a79bdd2e6e41674fb92d620863"
  Challenger = "0x8995e44a22e303a79bdd2e6e41674fb92d620863"

[[chains]]
  id = "0x000000000000000000000000000000000000000000000000000000000000a455"
  baseFeeVaultRecipient = "0xe9e05c9f02e10fa833d379cb1c7ac3a3f23b247e"
  l1FeeVaultRecipient = "0xe9e05c9f02e10fa833d379cb1c7ac3a3f23b247e"
  sequencerFeeVaultRecipient = "0xe9e05c9f02e10fa833d379cb1c7ac3a3f23b247e"
  # ... other fields auto-generated ...
  [chains.roles]
    l1ProxyAdminOwner = "0x8995e44a22e303a79bdd2e6e41674fb92d620863"
    l2ProxyAdminOwner = "0x8995e44a22e303a79bdd2e6e41674fb92d620863"
    systemConfigOwner = "0x61da7c7f97ebe53ad7c4e5ecd3d117e7ab430ea7"
    unsafeBlockSigner = "0x5b0248e30583ced4f09726c547935552c469eb24"
    batcher = "0xcdbc8abb83e01bae13ece8853a5ca84b2ef6ca86"
    proposer = "0xa683a3e33e07fb84ff33fce753da1d248298977f"
    challenger = "0x008099bfee75e832e1b93d4c023f646d99d4c90f"
```

⚠️ **Important:** If `init` creates a `[chains.customGasToken]` section, **remove it** (we use standard ETH).

---

## Step 5: Deploy L1 Contracts

**From `rollup/deployer` directory:**

```bash
../../op-deployer/bin/op-deployer apply \
  --workdir .deployer \
  --l1-rpc-url http://localhost:8545 \
  --private-key 0xd6a036f561e03196779dd34bf3d141dec4737eec5ed0416e413985ca05dad51a
```

This deploys all L1 contracts and saves state to `.deployer/state.json`.

---

## Step 6: Generate and Modify Chain Configuration

### 6.1 Generate genesis and rollup config

```bash
../../op-deployer/bin/op-deployer inspect genesis \
  --workdir .deployer \
  --outfile ../sequencer/genesis.json \
  42069

../../op-deployer/bin/op-deployer inspect rollup \
  --workdir .deployer \
  --outfile ../sequencer/rollup.json \
  42069
```

### 6.2 Modify genesis.json for Anvil compatibility

Disable future hardforks that Anvil doesn't fully support:

```bash
cd ../sequencer

# Set future fork times to far future (year 2286)
sed -i '' 's/"holoceneTime": 0/"holoceneTime": 9999999999/' genesis.json
sed -i '' 's/"isthmusTime": 0/"isthmusTime": 9999999999/' genesis.json
sed -i '' 's/"jovianTime": 0/"jovianTime": 9999999999/' genesis.json
sed -i '' 's/"pragueTime": 0/"pragueTime": 9999999999/' genesis.json

# Clear extraData (removes Holocene EIP-1559 encoding that causes errors)
sed -i '' 's/"extraData": "0x[^"]*"/"extraData": "0x"/' genesis.json
```

### 6.3 Modify rollup.json to match genesis

```bash
sed -i '' 's/"holocene_time": 0/"holocene_time": 9999999999/' rollup.json
sed -i '' 's/"isthmus_time": 0/"isthmus_time": 9999999999/' rollup.json
sed -i '' 's/"jovian_time": 0/"jovian_time": 9999999999/' rollup.json
```

---

## Step 7: Build op-node and op-geth

### 7.1 Build op-node

op-node is part of the optimism monorepo:

```bash
cd ../../op-node
just op-node
cd ..
```

Binary: `op-node/bin/op-node`

### 7.2 Clone and build op-geth

op-geth is a **separate repository**:

```bash
cd rollup
git clone https://github.com/ethereum-optimism/op-geth.git
cd op-geth
git checkout v1.101511.1
make geth
cd ..
```

Binary: `rollup/op-geth/build/bin/geth`

---

## Step 8: Configure and Initialize Sequencer

### 8.1 Create JWT secret

```bash
cd sequencer
openssl rand -hex 32 > jwt.txt
```

### 8.2 Create l1-chain-config.json

This file tells op-node about the L1 chain configuration (required for unknown chain IDs like Anvil's 31337):

```bash
cat > l1-chain-config.json << 'EOF'
{
  "config": {
    "chainId": 31337,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "shanghaiTime": 0,
    "cancunTime": 0,
    "terminalTotalDifficulty": 0,
    "blobSchedule": {
      "cancun": {
        "target": 3,
        "max": 6,
        "baseFeeUpdateFraction": 3338477
      }
    }
  }
}
EOF
```

### 8.3 Create .env file

```bash
cat > .env << 'EOF'
L1_RPC_URL=http://localhost:8545
L1_BEACON_URL=
SEQUENCER_ENABLED=true
SEQUENCER_STOPPED=false
PRIVATE_KEY=0x4bc19d3b0467a84723ad48118ed28884105526458d5a64a716ebea568318e3d0
P2P_LISTEN_PORT=9222
P2P_ADVERTISE_IP=127.0.0.1
OP_NODE_RPC_PORT=8547
OP_GETH_HTTP_PORT=9545
OP_GETH_WS_PORT=9546
OP_GETH_AUTH_PORT=9551
JWT_SECRET=./jwt.txt
EOF
```

### 8.4 Initialize op-geth

```bash
# Use --state.scheme=hash for archive mode compatibility
../op-geth/build/bin/geth init --datadir op-geth-data --state.scheme=hash genesis.json
```

**⚠️ Get the L2 genesis hash** (the init output truncates it):

```bash
../op-geth/build/bin/geth --datadir op-geth-data console --exec 'eth.getBlock(0).hash' 2>/dev/null
```

Update `rollup.json` with this hash (replace `genesis.l2.hash` value).

> **Note:** The genesis hash is deterministic - reinitializing with the same `genesis.json` will produce the same hash.

#### Troubleshooting: State Scheme Mismatch

If op-geth fails with:
```
Fatal: Failed to register the Ethereum service: incompatible state scheme, stored: path, provided: hash
```

Fix by cleaning and reinitializing:
```bash
rm -rf op-geth-data
../op-geth/build/bin/geth init --datadir op-geth-data --state.scheme=hash genesis.json
```

### 8.5 Create start scripts

**scripts/start-op-geth.sh:**

```bash
cat > scripts/start-op-geth.sh << 'EOF'
#!/bin/bash
set -e
cd "$(dirname "$0")/.."
source .env

../op-geth/build/bin/geth \
  --datadir=./op-geth-data \
  --http \
  --http.addr=0.0.0.0 \
  --http.port=$OP_GETH_HTTP_PORT \
  --http.api=web3,debug,eth,txpool,net,engine,admin,miner,personal \
  --http.corsdomain="*" \
  --ws \
  --ws.addr=0.0.0.0 \
  --ws.port=$OP_GETH_WS_PORT \
  --ws.api=web3,debug,eth,txpool,net,engine,admin,miner,personal \
  --ws.origins="*" \
  --authrpc.addr=0.0.0.0 \
  --authrpc.port=$OP_GETH_AUTH_PORT \
  --authrpc.vhosts="*" \
  --authrpc.jwtsecret=$JWT_SECRET \
  --syncmode=full \
  --gcmode=archive \
  --nodiscover \
  --maxpeers=0 \
  --networkid=42069 \
  --rollup.disabletxpoolgossip=true \
  --rpc.enabledeprecatedpersonal
EOF
chmod +x scripts/start-op-geth.sh
```

**scripts/start-op-node.sh:**

```bash
cat > scripts/start-op-node.sh << 'EOF'
#!/bin/bash
set -e
cd "$(dirname "$0")/.."
source .env

../../op-node/bin/op-node \
  --l1=$L1_RPC_URL \
  --l1.beacon.ignore=true \
  --l2=http://localhost:$OP_GETH_AUTH_PORT \
  --l2.jwt-secret=$JWT_SECRET \
  --rollup.config=./rollup.json \
  --rollup.l1-chain-config=./l1-chain-config.json \
  --rpc.addr=0.0.0.0 \
  --rpc.port=$OP_NODE_RPC_PORT \
  --p2p.listen.ip=0.0.0.0 \
  --p2p.listen.tcp=$P2P_LISTEN_PORT \
  --p2p.listen.udp=$P2P_LISTEN_PORT \
  --p2p.advertise.ip=$P2P_ADVERTISE_IP \
  --p2p.disable \
  --verifier.l1-confs=0 \
  --sequencer.enabled=$SEQUENCER_ENABLED \
  --sequencer.stopped=$SEQUENCER_STOPPED \
  --sequencer.l1-confs=0 \
  --p2p.sequencer.key=$PRIVATE_KEY
EOF
chmod +x scripts/start-op-node.sh
```

### Directory Structure

```
rollup/
├── deployer/
│   └── .deployer/
│       ├── intent.toml
│       └── state.json
├── op-geth/              # Cloned repository
│   └── build/bin/geth
└── sequencer/
    ├── .env
    ├── genesis.json
    ├── rollup.json
    ├── l1-chain-config.json
    ├── jwt.txt
    ├── op-geth-data/
    └── scripts/
        ├── start-op-geth.sh
        └── start-op-node.sh
```

---

## Step 9: Start the Sequencer

**Terminal 2 - Start op-geth:**

```bash
cd rollup/sequencer
./scripts/start-op-geth.sh
```

**Terminal 3 - Start op-node:**

```bash
cd rollup/sequencer
./scripts/start-op-node.sh
```

### Verify Sequencer is Running

```bash
# Check L2 block number (should be increasing)
cast block-number --rpc-url http://localhost:9545

# Check sequencer status
curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"admin_sequencerActive","params":[],"id":1}' \
  http://localhost:8547 | jq
```

---

## Deviations from Tutorial (Local Anvil)

The tutorial assumes **Sepolia testnet**. For **local Anvil**, these modifications are required:

| Issue | Solution |
|-------|----------|
| No beacon chain | `--l1.beacon.ignore=true` in op-node |
| Port conflict (Anvil on 8545) | op-geth uses ports 9545/9546/9551 |
| L1 confirmation delay | `--verifier.l1-confs=0` |
| Public IP required | `P2P_ADVERTISE_IP=127.0.0.1` |
| Future hardforks fail | Disable Holocene/Isthmus/Jovian/Prague in genesis |
| Blob gas division by zero | Add `baseFeeUpdateFraction` to l1-chain-config.json |

**Active forks:** Regolith, Canyon, Delta, Ecotone, Fjord, Granite
**Disabled forks:** Holocene, Isthmus, Jovian, Prague (set to year 2286)

---

## Bridging ETH (Deposits & Withdrawals)

### Key Contract Addresses

Get addresses from the deployment state (these change each deployment):

```bash
cd rollup/deployer
cat .deployer/state.json | grep -E "L1StandardBridgeProxy|OptimismPortalProxy"
```

- **L2StandardBridge (predeploy)**: `0x4200000000000000000000000000000000000010` (fixed address)

### L1 → L2 Deposit (Example)

Deposit 1 ETH from L1 (Anvil) to L2:

```bash
# Get the L1StandardBridgeProxy address from state.json
L1_BRIDGE=$(cat rollup/deployer/.deployer/state.json | jq -r '.opChainDeployments[0].L1StandardBridgeProxy')

# Using account #1 from Anvil
ACCOUNT="0xE9e05C9f02e10FA833D379CB1c7aC3a3f23B247e"
PRIVATE_KEY="0xbe62250c9db006c67c1595ff1f019bc849e2aa5c092dea0bf00883b39e54e904"

# Deposit ETH via L1StandardBridge
cast send $L1_BRIDGE "depositETH(uint32,bytes)" 100000 "0x" \
  --value 1ether \
  --private-key $PRIVATE_KEY \
  --rpc-url http://localhost:8545
# Args: 100000 = L2 gas limit for deposit tx, "0x" = empty extra data

# Mine L1 blocks to trigger L2 processing (Anvil only mines on-demand)
for i in {1..20}; do
  cast rpc anvil_mine 1 --rpc-url http://localhost:8545 > /dev/null
  sleep 0.5
done

# Check L2 balance
cast balance $ACCOUNT --rpc-url http://localhost:9545 --ether
```

> **Note:** The deposit won't appear on L2 immediately. The L2 sequencer must process the L1 block containing the deposit. Check sync status:
> ```bash
> curl -s -X POST -H "Content-Type: application/json" \
>   --data '{"jsonrpc":"2.0","method":"optimism_syncStatus","params":[],"id":1}' \
>   http://localhost:8547 | jq '.result.unsafe_l2.l1origin.number'
> ```
> Wait until this number is >= the L1 block where your deposit was made.

### L2 → L1 Withdrawal (Example)

**Note:** To submit L2 transactions, op-geth must be started **WITHOUT** `--rollup.sequencerhttp`. When this flag is absent, transaction admission is enabled by default.

> ⚠️ **Security Note:** Removing `--rollup.sequencerhttp` is safe for single-node dev setups. In production with multiple nodes:
> - **Sequencer node**: No `--rollup.sequencerhttp` (accepts transactions)
> - **Replica nodes**: Use `--rollup.sequencerhttp=<sequencer-url>` (forwards transactions)
>
> Without this, replica nodes would accept transactions locally that never get sequenced, leading to **transaction loss** and **inconsistent state**.

#### Step 1: Initiate Withdrawal on L2

```bash
ACCOUNT="0xE9e05C9f02e10FA833D379CB1c7aC3a3f23B247e"
PRIVATE_KEY="0xbe62250c9db006c67c1595ff1f019bc849e2aa5c092dea0bf00883b39e54e904"
L2_BRIDGE="0x4200000000000000000000000000000000000010"

# Withdraw 0.1 ETH via L2StandardBridge
cast send $L2_BRIDGE "withdraw(address,uint256,uint32,bytes)" \
  "0xDeadDeAddeAddEAddeadDEaDDEAdDeaDDeAD0000" \
  "100000000000000000" \
  100000 \
  "0x" \
  --value 0.1ether \
  --private-key $PRIVATE_KEY \
  --rpc-url http://localhost:9545
```

**Parameters explained:**
- `0xDeadDeAddeAddEAddeadDEaDDEAdDeaDDeAD0000` - Native ETH token address (special L2 address)
- `100000000000000000` - Amount in wei (0.1 ETH)
- `100000` - L1 gas limit for finalization
- `0x` - Extra data (empty)
- `--value 0.1ether` - Must match the withdrawal amount

**Successful example output:**
```
blockNumber          6724
status               1 (success)
transactionHash      0x25ce27ca7d7d3ecc718e529aebbb2d00933653e5ea4e18d3ed2cb4f581ebcb85
```

#### Full Withdrawal Flow

| Step | Description | Time |
|------|-------------|------|
| 1 | Initiate on L2 | Instant |
| 2 | Wait for batcher to submit batch | ~minutes |
| 3 | Wait for proposer to submit state root | ~2 min (proposal interval) |
| 4 | Prove withdrawal on L1 | Requires SDK |
| 5 | Wait for challenge period | 7 days (mainnet) |
| 6 | Finalize on L1 | Requires SDK |

> **Note:** Steps 4-6 require the Optimism SDK or specialized tooling to generate proofs and call the OptimismPortal contract. For local testing, you can verify the withdrawal was initiated by checking:
> - L2 balance decreased
> - L2ToL1MessagePasser logs emitted (address `0x4200000000000000000000000000000000000016`)
> - Proposer submitting state roots to DisputeGameFactory

---

## Deployment Gas Analysis

Gas usage breakdown for deploying all L1 rollup contracts.

**Note:** this was done with automine off, so each block is really a single transaction on anvil.

### Summary

| Metric | Value |
|--------|-------|
| **Total Transactions** | 37 |
| **Total Gas Used** | 82,310,606 |
| **Blocks Used** | 1-37 |

### ETH Cost at Various Gas Prices

| Gas Price | ETH Cost |
|-----------|----------|
| 1 gwei | 0.082 ETH |
| 10 gwei | 0.82 ETH |
| 50 gwei | 4.12 ETH |
| 100 gwei | 8.23 ETH |

### Per-Transaction Breakdown

| Block | Gas Used | To Address |
|-------|----------|------------|
| 1 | 1,483,961 | Contract Creation |
| 2 | 754,616 | CREATE2 Deployer |
| 3 | 612,135 | CREATE2 Deployer |
| 4 | 524,880 | Contract Creation |
| 5 | 87,426 | Proxy Admin |
| 6 | 524,880 | Contract Creation |
| 7 | 161,246 | Proxy Admin |
| 8 | 25,762 | Proxy Admin |
| 9 | 2,926,457 | CREATE2 Deployer |
| 10 | 2,145,981 | CREATE2 Deployer |
| 11 | 1,409,950 | CREATE2 Deployer |
| 12 | 2,791,251 | CREATE2 Deployer |
| 13 | 2,412,797 | CREATE2 Deployer |
| 14 | 4,480,329 | CREATE2 Deployer |
| 15 | 5,284,792 | CREATE2 Deployer |
| 16 | 1,138,891 | CREATE2 Deployer |
| 17 | 1,355,699 | CREATE2 Deployer |
| 18 | 3,475,288 | CREATE2 Deployer |
| 19 | 4,711,079 | CREATE2 Deployer |
| 20 | 1,816,094 | CREATE2 Deployer |
| 21 | 1,654,555 | CREATE2 Deployer |
| 22 | 5,142,762 | CREATE2 Deployer |
| 23 | 5,245,824 | CREATE2 Deployer |
| 24 | 297,294 | CREATE2 Deployer |
| 25 | 408,243 | CREATE2 Deployer |
| 26 | 558,249 | CREATE2 Deployer |
| 27 | 1,507,291 | CREATE2 Deployer |
| 28 | 620,079 | CREATE2 Deployer |
| 29 | 381,929 | CREATE2 Deployer |
| 30 | 860,088 | CREATE2 Deployer |
| 31 | 3,020,045 | CREATE2 Deployer |
| 32 | 3,179,873 | CREATE2 Deployer |
| 33 | 3,041,643 | CREATE2 Deployer |
| 34 | 2,561,175 | CREATE2 Deployer |
| 35 | 5,031,793 | CREATE2 Deployer |
| 36 | 2,376,481 | CREATE2 Deployer |
| 37 | 8,299,768 | Initialization |

> **Note:** Most deployments use the CREATE2 Deployer (`0x4e59b44847b379578588920ca78fbf26c0b4956c`) for deterministic addresses.

### Block 37 Deep Dive (8.3M Gas)

The largest transaction (8,299,768 gas) calls the **OPContractsManager (OpcmImpl)** which deploys an entire OP Chain's L1 infrastructure in a single transaction.

**Contract:** `0xabc4c5cbb04422bbfc231bcc0c38ad6c9efb6dc8` (OpcmImpl)
**Function:** `deploy(DeployInput)`
**Events Emitted:** 44

#### Contracts Deployed/Initialized

| Contract | Purpose |
|----------|---------|
| opChainProxyAdmin | Proxy admin for the chain |
| addressManager | Address registry |
| l1ERC721BridgeProxy | NFT bridge |
| systemConfigProxy | Chain configuration |
| optimismMintableERC20FactoryProxy | Token factory |
| l1StandardBridgeProxy | ETH/ERC20 bridge |
| l1CrossDomainMessengerProxy | Cross-chain messaging |
| ethLockboxProxy | ETH lockup |
| optimismPortalProxy | Main entry point for deposits |
| disputeGameFactoryProxy | Fault proof games |
| anchorStateRegistryProxy | State anchoring |
| faultDisputeGame | Fault dispute logic |
| permissionedDisputeGame | Permissioned disputes |
| delayedWETHPermissionedGameProxy | Delayed WETH (permissioned) |
| delayedWETHPermissionlessGameProxy | Delayed WETH (permissionless) |

#### Why So Much Gas?

- **44 events emitted** - Each contract initialization emits events
- **Multiple proxy deployments** - Each proxy + implementation setup
- **Cross-contract calls** - OpcmImpl orchestrates many internal calls
- **State initialization** - Setting up initial state for all contracts

This single transaction deploys the **entire L1 infrastructure** for an OP Chain, including the bridge, portal, dispute game system, and all proxies.

---

## Restarting the Rollup (Anvil In-Memory)

Since Anvil runs in-memory by default, **all L1 state is lost on restart**. This means every restart is effectively a **full redeployment**.

### What's Reusable Across Restarts

| Component | Notes |
|-----------|-------|
| Built binaries | `op-deployer`, `op-node`, `op-geth` |
| Shell scripts | `start-op-geth.sh`, `start-op-node.sh` |
| `l1-chain-config.json` | Static Anvil config |
| `.env` files | Environment variables |
| `intent.toml` structure | Only need to update if roles change |

### What Must Be Redone After Anvil Restart

| Step | Reason |
|------|--------|
| `op-deployer init` | Creates fresh `state.json` (overwrites `intent.toml`!) |
| Update `intent.toml` | Restore seed 2 addresses after init |
| `op-deployer apply` | Deploys new L1 contracts (new addresses) |
| `op-deployer inspect genesis/rollup` | Generates configs with new contract addresses |
| Modify `genesis.json` & `rollup.json` | Disable future hardforks for Anvil compatibility |
| `op-geth init` | Initialize L2 with new genesis |
| Update `rollup.json` L2 hash | Match the new L2 genesis hash from geth init output |

### Alternative: Persist Anvil State

To avoid full redeployment, use Anvil's state persistence:
- Start: `anvil --mnemonic-seed-unsafe 2 --hardfork cancun --block-time 15 --state anvil-state.json`
- Restart: `anvil --mnemonic-seed-unsafe 2 --hardfork cancun --block-time 15 --load-state anvil-state.json`

---

## Step 10: Spin Up Batcher

Tutorial: https://docs.optimism.io/chain-operators/tutorials/create-l2-rollup/op-batcher-setup

The batcher (`op-batcher`) submits L2 transaction batches to L1 for data availability.

### 10.1 Build op-batcher

```bash
cd op-batcher
just op-batcher
cd ..
```

Binary: `op-batcher/bin/op-batcher`

### 10.2 Create batcher directory

```bash
cd rollup
mkdir -p batcher/scripts
cd batcher

# Copy state.json (for reference)
cp ../deployer/.deployer/state.json .

# Get BatchInbox address
cat ../sequencer/rollup.json | jq -r '.batch_inbox_address'
```

### 10.3 Create .env file

```bash
cat > .env << 'EOF'
# L1 Configuration (Anvil)
L1_RPC_URL=http://localhost:8545

# L2 Configuration
L2_RPC_URL=http://localhost:9545
ROLLUP_RPC_URL=http://localhost:8547

# Contract addresses (from rollup.json batch_inbox_address)
BATCH_INBOX_ADDRESS=<YOUR_BATCH_INBOX_ADDRESS>

# Private key - Anvil account #4 (batcher role)
BATCHER_PRIVATE_KEY=0x07127c605875395527bccbc5c74afa9dc1712d83ba5b12207bab86d3b7fa8be6

# Batcher configuration
POLL_INTERVAL=1s
SUB_SAFETY_MARGIN=6
NUM_CONFIRMATIONS=1
SAFE_ABORT_NONCE_TOO_LOW_COUNT=3
RESUBMISSION_TIMEOUT=30s
MAX_CHANNEL_DURATION=25
BATCHER_RPC_PORT=8548
EOF
```

### 10.4 Create start script

Create `scripts/start-batcher.sh`:

```bash
#!/bin/bash
set -e
cd "$(dirname "$0")/.."
source .env

../../op-batcher/bin/op-batcher \
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
```

> **Note:** Using `--data-availability-type=calldata` because Anvil doesn't support EIP-4844 blobs. On mainnet/Sepolia, use `blobs` for lower costs.

Make executable: `chmod +x scripts/start-batcher.sh`

### 10.5 Start the batcher

**Terminal 4:**

```bash
cd rollup/batcher
./scripts/start-batcher.sh
```

### Directory Structure

```
rollup/
├── deployer/
├── op-geth/
├── sequencer/
└── batcher/
    ├── state.json      # Copied from deployer
    ├── .env            # Environment variables
    └── scripts/
        └── start-batcher.sh
```

---

## 11. Spin up op-proposer

The proposer submits L2 state roots to L1 via the DisputeGameFactory contract. This is required for L2→L1 withdrawals to be proven and finalized.

### 11.1 Build op-proposer

```bash
cd optimism/op-proposer
mkdir -p bin
go build -o bin/op-proposer ./cmd
```

### 11.2 Create proposer directory

```bash
cd rollup
mkdir -p proposer/scripts
cd proposer

# Copy state.json from deployer
cp ../deployer/.deployer/state.json .

# Extract DisputeGameFactory address
cat state.json | jq -r '.opChainDeployments[0].DisputeGameFactoryProxy'
```

### 11.3 Set up environment variables

Create `.env`:

```bash
cat > .env << 'EOF'
# L1 Configuration (Anvil)
L1_RPC_URL=http://localhost:8545

# L2 Configuration
L2_RPC_URL=http://localhost:9545
ROLLUP_RPC_URL=http://localhost:8547

# DisputeGameFactory address from deployment
GAME_FACTORY_ADDRESS=<ADDRESS_FROM_STEP_11.2>

# Proposer private key (Anvil seed 2, Account 5)
PRIVATE_KEY=0x309bc84a97ca76f0c15bc3a6c98d46d8fb2381c8bd63ba6cc7b0d25ff4a13332

# Proposer configuration
PROPOSAL_INTERVAL=120s
GAME_TYPE=1
POLL_INTERVAL=6s
PROPOSER_RPC_PORT=8560
EOF
```

### 11.4 Create start script

Create `scripts/start-proposer.sh`:

```bash
#!/bin/bash
set -e
cd "$(dirname "$0")/.."
source .env

../../op-proposer/bin/op-proposer \
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
```

> **Note:** Using `--allow-non-finalized=true` and `--wait-node-sync=false` for Anvil since it doesn't have finality like a real L1.

Make executable: `chmod +x scripts/start-proposer.sh`

### 11.5 Start the proposer

**Terminal 6:**

```bash
cd rollup/proposer
./scripts/start-proposer.sh
```

### Directory Structure

```
rollup/
├── deployer/
├── op-geth/
├── sequencer/
├── batcher/
└── proposer/
    ├── state.json      # Copied from deployer
    ├── .env            # Environment variables
    └── scripts/
        └── start-proposer.sh
```

### 11.6 Monitor Proposer Activity

A Python script is provided to monitor proposer transactions on L1.

**Location:** `rollup/scripts/proposer_status.py`

**Usage:**

```bash
# One-time status check
python3 rollup/scripts/proposer_status.py

# Continuous monitoring (every 30 seconds)
python3 rollup/scripts/proposer_status.py watch

# Custom interval (every 10 seconds)
python3 rollup/scripts/proposer_status.py watch 10
```

**Output includes:**
- Proposer account balance and nonce
- L2 sync status (unsafe, safe, finalized blocks)
- Recent proposer transactions to DisputeGameFactory
- L1/L2 block numbers

---

## 12. Spin up op-challenger

The challenger monitors dispute games and challenges invalid claims.

### 12.1 Generate Prestate (Requires Docker)

```bash
cd optimism

# Copy chain configs for prestate generation
mkdir -p op-program/chainconfig/configs
cp rollup/sequencer/rollup.json op-program/chainconfig/configs/42069-rollup.json
cp rollup/sequencer/genesis.json op-program/chainconfig/configs/42069-genesis-l2.json

# Create L1 genesis config for Anvil
cat > op-program/chainconfig/configs/31337-genesis-l1.json << 'EOF'
{
  "config": {
    "chainId": 31337,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "shanghaiTime": 0,
    "cancunTime": 0,
    "terminalTotalDifficulty": 0,
    "blobSchedule": {
      "cancun": { "target": 3, "max": 6, "baseFeeUpdateFraction": 3338477 }
    }
  },
  "alloc": {},
  "difficulty": "0x0",
  "gasLimit": "0x1c9c380"
}
EOF

# Generate prestate (requires Docker)
make reproducible-prestate
```

Save the **Cannon64 Absolute prestate hash** from the output (e.g., `0x03d2a051e4d2b2e9bfa365ec6a562ead2ba245108c47d6b52ae79bebd97828ec`).

### 12.2 Build op-challenger and cannon

```bash
cd optimism/op-challenger && just
cd ../.. && make cannon
```

### 12.3 Create challenger directory

```bash
cd rollup
mkdir -p challenger/scripts
cd challenger

# Copy state for contract addresses
cp ../deployer/.deployer/state.json .

# Get DisputeGameFactory address
cat state.json | jq -r '.opChainDeployments[0].DisputeGameFactoryProxy'
```

### 12.4 Environment variables

```bash
cat > .env << 'EOF'
L1_RPC_URL=http://localhost:8545
L2_RPC_URL=http://localhost:9545
ROLLUP_RPC_URL=http://localhost:8547
GAME_FACTORY_ADDRESS=0xc2f53f7e5ae6180682e9353b34ec544053784a91

# Challenger private key (Anvil seed 2, Account 6)
PRIVATE_KEY=0x4e84476ad472f4a1ccf16e2f96a70df8f8c4a9bceb06e0a4c1bdca6fddce9d1c

# Paths (relative to rollup/challenger/)
DATADIR=./data
CANNON_BIN=../../cannon/bin/cannon
CANNON_SERVER=../../op-program/bin/op-program
CANNON_PRESTATE=../../op-program/bin/prestate-proof-mt64.json
CANNON_ROLLUP_CONFIG=../sequencer/rollup.json
CANNON_L2_GENESIS=../sequencer/genesis.json
EOF
```

### 12.5 Start script

```bash
cat > scripts/start-challenger.sh << 'EOF'
#!/bin/bash
source .env
mkdir -p $DATADIR

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
EOF
chmod +x scripts/start-challenger.sh
```

### 12.6 Start the challenger

**Terminal 7:**
```bash
cd rollup/challenger
./scripts/start-challenger.sh
```

### Challenger Limitations on Anvil

| Feature | Status | Notes |
|---------|--------|-------|
| Startup | ✅ Works | Connects to L1/L2 |
| Game monitoring | ✅ Works | Finds dispute games |
| Move calculation | ⚠️ Limited | Needs archive node with SafeDB |
| Full dispute participation | ❌ | Requires real beacon endpoint |

> **Note:** For local Anvil testing, the challenger can monitor games but cannot fully participate in disputes due to missing beacon endpoint and SafeDB. This is expected for development setups.

---

## Tutorial Complete! 🎉

All components of the Optimism L2 rollup are now running:

1. ✅ **op-deployer** - L1 contracts deployed
2. ✅ **Sequencer** (op-geth + op-node) - Processing transactions
3. ✅ **op-batcher** - Publishing data to L1
4. ✅ **op-proposer** - Submitting state roots
5. ✅ **op-challenger** - Monitoring disputes

---

## Process Management

A script is provided to check and manage all rollup processes.

**Location:** `rollup/scripts/check_processes.py`

### Check Status (safe - doesn't kill anything)

```bash
python3 rollup/scripts/check_processes.py
```

Output shows which components are running and their PIDs:
```
L1 Chain (Anvil)          ✅ Running
  └─ PID 12345: anvil --hardfork cancun --block-time 15...
L2 Execution (op-geth)    ✅ Running
  └─ PID 12346: geth --datadir op-geth-data...
...
```

### Kill All Processes

```bash
# Graceful shutdown (SIGTERM)
python3 rollup/scripts/check_processes.py kill

# Force kill (SIGKILL) - use if graceful fails
python3 rollup/scripts/check_processes.py kill -f
```

### Manual Process Check

```bash
# Check individual processes
pgrep -f anvil
pgrep -f "geth.*op-geth-data"
pgrep -f op-node
pgrep -f op-batcher
pgrep -f op-proposer
pgrep -f op-challenger
```

---

## Cleanup

A cleanup script is provided to stop processes and remove generated data.

**Location:** `rollup/scripts/cleanup.py`

### Dry Run (see what would be cleaned)

```bash
python3 rollup/scripts/cleanup.py
```

### Standard Cleanup (stop processes + remove runtime data)

```bash
python3 rollup/scripts/cleanup.py --execute
```

This removes:
- `sequencer/op-geth-data/` - L2 chain data
- `sequencer/opnode_*` - op-node databases
- `sequencer/jwt.txt` - JWT secret
- `*/state.json` copies in batcher/proposer/challenger
- `challenger/data/` - Challenger cache

### Full Cleanup (also remove deployment state)

```bash
python3 rollup/scripts/cleanup.py --execute --full
```

Additionally removes:
- `deployer/.deployer/state.json` - Deployment state
- `sequencer/genesis.json` - L2 genesis
- `sequencer/rollup.json` - Rollup config

> ⚠️ **After full cleanup**, you must redeploy contracts with `op-deployer apply` before starting the rollup again.

### What's Preserved

Even after cleanup, these are kept:
- `deployer/.deployer/intent.toml` - Deployment configuration
- `*/scripts/*.sh` - Start scripts
- `scripts/*.py` - Utility scripts
- `sequencer/l1-chain-config.json` - L1 config for op-node
- `sequencer/.env` - Environment variables (but may need updating)

---

## Quick Reference

### Ports

| Service | Port | URL |
|---------|------|-----|
| Anvil (L1) | 8545 | http://localhost:8545 |
| op-geth (L2 HTTP) | 9545 | http://localhost:9545 |
| op-geth (L2 WS) | 9546 | ws://localhost:9546 |
| op-geth (Auth RPC) | 9551 | http://localhost:9551 |
| op-node (RPC) | 8547 | http://localhost:8547 |
| op-batcher (RPC) | 8548 | http://localhost:8548 |
| op-proposer (RPC) | 8560 | http://localhost:8560 |

### Key Paths

| File | Path |
|------|------|
| Deployment state | `rollup/deployer/.deployer/state.json` |
| Intent config | `rollup/deployer/.deployer/intent.toml` |
| Genesis | `rollup/sequencer/genesis.json` |
| Rollup config | `rollup/sequencer/rollup.json` |
| L1 chain config | `rollup/sequencer/l1-chain-config.json` |
| JWT secret | `rollup/sequencer/jwt.txt` |
| op-geth data | `rollup/sequencer/op-geth-data/` |

### Chain IDs

- **L1 (Anvil)**: 31337
- **L2 (Rollup)**: 42069

---

## Demo: L2 Transactions

A Python script is provided to generate demo transactions on L2 using the Anvil seed 2 accounts.

**Location:** `rollup/scripts/l2_demo.py`

### Usage

```bash
# Show all L2 account balances
python3 rollup/scripts/l2_demo.py balances

# Show L2 sync status
python3 rollup/scripts/l2_demo.py status

# Send ETH between accounts (by index 0-7)
python3 rollup/scripts/l2_demo.py send <from_index> <to_index> <amount_eth>
# Example: Send 0.1 ETH from Account 1 to Account 0
python3 rollup/scripts/l2_demo.py send 1 0 0.1

# Fund multiple accounts (sends 0.1 ETH from Account 1 to Accounts 0,2,3,4,5)
python3 rollup/scripts/l2_demo.py fund-accounts
```

### Prerequisites

**L2 accounts start with 0 ETH.** Before using this script, you must first bridge ETH from L1 to L2:

```bash
# Bridge 1 ETH from L1 to L2 (to Account 1)
cast send --rpc-url http://localhost:8545 \
  --private-key 0xbe62250c9db006c67c1595ff1f019bc849e2aa5c092dea0bf00883b39e54e904 \
  <L1StandardBridgeProxy_ADDRESS> \
  "depositETH(uint32,bytes)" 100000 "0x" \
  --value 1ether
```

After the deposit is processed (wait for L2 to sync with L1), Account 1 will have 1 ETH on L2.

### Notes

- The script uses `cast` (Foundry) under the hood for transaction signing
- All accounts are derived from Anvil's `--mnemonic-seed-unsafe 2`
- Get the L1StandardBridgeProxy address from: `cat rollup/batcher/state.json | jq -r '.opChainDeployments[0].L1StandardBridgeProxy'`

---

## Environment Info

- **OS:** macOS (darwin 25.2.0)
- **Forge version:** 1.5.0-stable
- **Go version:** (used for building op-deployer)
- **Just:** (used as build tool)

