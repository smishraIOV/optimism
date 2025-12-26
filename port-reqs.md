# Next steps

We have to enforce some restrictions on how the rollup works - all of them arise from restrictions related to the L1 chain.

1. We must deploy contracts on L1 in a way so that no deployment transaction consumes more than 4 million gas.
2. We must enforce that all L1 transactions from the rollup must be created in legacy format - i.e. without any eip-1559 gas fields: thus, any rollup component or operator account that generates L1 transactions must be modified to use legacy format

To test the above restrictions, we can run anvil with a low block gas limit of around 6.5M and restrict it to legacy transactions only.

---

## Proposal

### Overview

| Requirement | Affected Components | Complexity | Approach |
|-------------|---------------------|------------|----------|
| 4M gas limit per tx | op-deployer, forge scripts | High | Split contract deployments |
| Legacy transactions | op-batcher, op-proposer, op-challenger | Medium | Modify txmgr |

---

### Requirement 1: Limit Deployment Gas to 4M per Transaction

#### Current State

From our gas analysis, the following deployment transactions exceed 4M gas:

| Block | Gas Used | Contract/Operation |
|-------|----------|-------------------|
| 37 | 8,330,606 | OPContractsManager.deploy (full chain) |
| 14 | 4,480,329 | CREATE2 deployment |
| 15 | 5,373,839 | CREATE2 deployment |

The main issue is `OPContractsManager.deploy()` which deploys the entire chain in a single transaction (~8.3M gas).

#### Required Changes

**Change A: Split OPContractsManager into Phases**

Split `OPContractsManager.deploy()` into multiple phases, passing state via parameters (stateless approach).

**Design: Pass State via Parameters**

```solidity
// Phase 1: Deploy singletons (~500K gas)
function deployPhase1(DeployInput calldata input)
    external returns (Phase1Output memory);

// Phase 2: Deploy proxies batch 1 (~1.5M gas)
function deployPhase2(DeployInput calldata input, Phase1Output calldata phase1)
    external returns (Phase2Output memory);

// Phase 3: Deploy proxies batch 2 (~1.5M gas)
function deployPhase3(DeployInput calldata input, Phase2Output calldata phase2)
    external returns (Phase3Output memory);

// Phase 4: Initialize batch 1 (~2M gas)
function deployPhase4(DeployInput calldata input, Phase3Output calldata phase3)
    external returns (Phase4Output memory);

// Phase 5: Initialize batch 2 + finalize (~2M gas)
function deployPhase5(DeployInput calldata input, Phase4Output calldata phase4)
    external returns (DeployOutput memory);
```

**Why stateless (vs storage-based):**
| Benefit | Explanation |
|---------|-------------|
| Lower gas | No SSTORE (~20K) or SLOAD (~2.1K) per address |
| No cleanup | No need to clear storage after deployment |
| Simpler testing | No storage state to reset between tests |
| Cleaner contract | No storage pollution |

**Output structs:**

```solidity
struct Phase1Output {
    IAddressManager addressManager;
    IProxyAdmin opChainProxyAdmin;
}

struct Phase2Output {
    Phase1Output phase1;
    IL1ERC721Bridge l1ERC721BridgeProxy;
    IOptimismPortal optimismPortalProxy;
    IETHLockbox ethLockboxProxy;
    ISystemConfig systemConfigProxy;
    IOptimismMintableERC20Factory optimismMintableERC20FactoryProxy;
}

struct Phase3Output {
    Phase2Output phase2;
    IDisputeGameFactory disputeGameFactoryProxy;
    IAnchorStateRegistry anchorStateRegistryProxy;
    IL1StandardBridge l1StandardBridgeProxy;
    IL1CrossDomainMessenger l1CrossDomainMessengerProxy;
    IDelayedWETH delayedWETHPermissionedGameProxy;
}

// Phase4Output and Phase5Output follow same pattern
```

**Changes required:**

1. `packages/contracts-bedrock/src/L1/OPContractsManager.sol`
   - Add phase output structs
   - Split `deploy()` into `deployPhase1()` through `deployPhase5()`
   - Each phase takes previous phase output as parameter
   - Keep original `deploy()` as wrapper that calls all phases (for backwards compatibility)

2. `op-deployer/pkg/deployer/opcm/opchain.go`
   - Call each phase separately
   - Store intermediate outputs in `state.json`
   - Pass outputs to subsequent phases

**Estimated changes:** ~600 lines of Solidity, ~150 lines of Go

**Address determinism:** Splitting into phases does NOT affect contract addresses because CREATE2 addresses depend only on deployer address, salt, and bytecode - not on which transaction the deployment occurs in.

**Change B: Reduce Large CREATE2 Deployments**

Split large CREATE2 deployments into smaller chunks using a factory pattern:

```solidity
// Instead of deploying large bytecode directly
// Deploy a minimal factory that deploys the actual contract
contract MinimalFactory {
    function deploy(bytes memory code) external returns (address) {
        address addr;
        assembly {
            addr := create(0, add(code, 0x20), mload(code))
        }
        return addr;
    }
}
```

**Estimated changes:** ~200 lines of Solidity

#### Required Approach: Both Changes A and B

Based on the gas analysis, **both changes are required** to meet the 4M limit:

| Issue | Gas | Solution |
|-------|-----|----------|
| OPContractsManager.deploy (Block 37) | 8.3M | **Change A** - Split into phases |
| Large CREATE2 bytecode (Blocks 14, 15) | 4.5M, 5.4M | **Change B** - Reduce bytecode size |

These are **complementary, not alternatives**:
- Change A alone won't fix the 4.5M/5.4M CREATE2 deployments
- Change B alone won't fix the 8.3M OPContractsManager.deploy

> **Note:** Individual forge deployments are NOT viable because OPContractsManager uses CREATE2 for deterministic addresses. Changing the deployment method would result in different contract addresses.

#### Testing Strategy

```bash
# Run Anvil with 6.5M gas limit (confirmed working)
anvil --mnemonic-seed-unsafe 2 --hardfork cancun --block-time 15 --gas-limit 6500000

# After deployment, analyze gas usage per transaction
python3 rollup/scripts/analyze_gas.py
```

**`analyze_gas.py`** - Analyzes all L1 transactions and reports:
- Gas used vs gas limit per transaction
- Transactions exceeding 4M gas (flagged with ⚠️)
- Top 10 transactions by gas usage

Example output:
```
Block |     Gas Used |    Gas Limit |  % Used | To/Contract
   24 |    5,284,792 |    6,326,692 |   83.5% | 0x4e59b44... ⚠️

⚠️  7 transactions exceed 4M gas limit
```

---

### Requirement 2: Legacy Transaction Format

#### Current State

All L1 transactions from OP Stack components use **EIP-1559 (Type 2)** format:

| Component | Transaction Type | Code Location |
|-----------|------------------|---------------|
| op-batcher | DynamicFeeTx | `op-service/txmgr/txmgr.go:417` |
| op-proposer | DynamicFeeTx | `op-service/txmgr/txmgr.go:417` |
| op-challenger | DynamicFeeTx | `op-service/txmgr/txmgr.go:417` |

The transaction manager (`SimpleTxManager`) always creates `types.DynamicFeeTx`:

```go
// Current code (op-service/txmgr/txmgr.go:417-425)
txMessage = &types.DynamicFeeTx{
    ChainID:   m.chainID,
    To:        candidate.To,
    GasTipCap: gasTipCap,
    GasFeeCap: gasFeeCap,
    Value:     candidate.Value,
    Data:      candidate.TxData,
    Gas:       candidate.GasLimit,
}
```

#### Solution: Add Legacy Transaction Support to TxManager

**Step 1: Add Configuration Flag**

File: `op-service/txmgr/cli.go`

```go
// Add new flag
var (
    UseLegacyTxFlag = &cli.BoolFlag{
        Name:    "txmgr.use-legacy-tx",
        Usage:   "Use legacy (Type 0) transactions instead of EIP-1559",
        EnvVars: prefixEnvVars("TXMGR_USE_LEGACY_TX"),
    }
)

// Add to CLIFlagsWithDefaults
func CLIFlagsWithDefaults(...) []cli.Flag {
    // ... existing flags ...
    UseLegacyTxFlag,
}
```

File: `op-service/txmgr/config.go`

```go
type Config struct {
    // ... existing fields ...
    UseLegacyTx bool // Use legacy transactions (Type 0)
}
```

**Step 2: Modify Transaction Creation**

File: `op-service/txmgr/txmgr.go`

```go
func (m *SimpleTxManager) craftTx(ctx context.Context, candidate TxCandidate) (*types.Transaction, error) {
    // ... existing gas estimation code ...

    var txMessage types.TxData
    if sidecar != nil {
        // Blob transactions (unchanged)
        // ...
    } else if m.cfg.UseLegacyTx {
        // NEW: Legacy transaction support
        gasPrice := new(big.Int).Add(baseFee, gasTipCap) // Combine into single gas price
        txMessage = &types.LegacyTx{
            Nonce:    0, // Will be set by signer
            To:       candidate.To,
            GasPrice: gasPrice,
            Value:    candidate.Value,
            Data:     candidate.TxData,
            Gas:      candidate.GasLimit,
        }
    } else {
        // EIP-1559 transaction (existing code)
        txMessage = &types.DynamicFeeTx{
            ChainID:   m.chainID,
            To:        candidate.To,
            GasTipCap: gasTipCap,
            GasFeeCap: gasFeeCap,
            Value:     candidate.Value,
            Data:      candidate.TxData,
            Gas:       candidate.GasLimit,
        }
    }
    return m.signWithNextNonce(ctx, txMessage)
}
```

**Step 3: Update Fee Bumping Logic**

File: `op-service/txmgr/txmgr.go` (in `increaseGasPrice` function)

```go
func (m *SimpleTxManager) increaseGasPrice(ctx context.Context, tx *types.Transaction) (*types.Transaction, error) {
    // ... existing code ...

    var newTx *types.Transaction
    if tx.Type() == types.BlobTxType {
        // Blob transaction (unchanged)
    } else if tx.Type() == types.LegacyTxType {
        // NEW: Handle legacy transaction fee bumping
        bumpedPrice := new(big.Int).Add(bumpedFee, bumpedTip)
        newTx = types.NewTx(&types.LegacyTx{
            Nonce:    tx.Nonce(),
            To:       tx.To(),
            GasPrice: bumpedPrice,
            Value:    tx.Value(),
            Data:     tx.Data(),
            Gas:      gas,
        })
    } else {
        // DynamicFeeTx (existing code)
    }
    // ... sign and return ...
}
```

**Step 4: Apply to All Components**

Each component needs to pass the flag through:

| Component | Config File | Environment Variable |
|-----------|-------------|---------------------|
| op-batcher | `op-batcher/flags/flags.go` | `OP_BATCHER_TXMGR_USE_LEGACY_TX=true` |
| op-proposer | `op-proposer/flags/flags.go` | `OP_PROPOSER_TXMGR_USE_LEGACY_TX=true` |
| op-challenger | `op-challenger/flags/flags.go` | `OP_CHALLENGER_TXMGR_USE_LEGACY_TX=true` |

**Estimated changes:** ~150 lines across txmgr, ~30 lines per component

#### Testing Strategy

```bash
# Run Anvil with Berlin hardfork (pre-EIP-1559) to reject EIP-1559 txs
anvil --hardfork berlin --gas-limit 6500000

# Or use Cancun but verify transaction types
python3 rollup/scripts/verify_tx_types.py --expected-type 0
```

---

### Testing Configuration

```bash
# Anvil configuration for testing both restrictions
anvil \
  --hardfork berlin \          # Pre-EIP-1559, rejects Type 2 txs
  --gas-limit 6500000 \         # 6.5M block limit
  --block-time 15 \
  --mnemonic-seed-unsafe 2

# Or for Cancun with manual verification
anvil \
  --hardfork cancun \
  --gas-limit 6500000 \
  --block-time 15 \
  --mnemonic-seed-unsafe 2

# Run batcher with legacy mode
OP_BATCHER_TXMGR_USE_LEGACY_TX=true ./scripts/start-batcher.sh

# Verify transactions
cast tx <TX_HASH> --rpc-url http://localhost:8545 | grep "type"
# Should show: type: 0 (legacy)
```

---

### Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Legacy txs may have different gas estimation | Medium | Add buffer to gas estimates |
| Phased deployment may fail mid-way | High | Add rollback/resume logic |
| Nonce management with legacy txs | Medium | Existing txmgr nonce logic should work |
| Breaking changes to upstream OP Stack | Low | Keep changes isolated, easy to merge |

---

### Success Criteria

1. ✅ All deployment transactions use < 4M gas
2. ✅ All L1 transactions from batcher/proposer/challenger are Type 0 (legacy)
3. ✅ Rollup functions correctly with restricted Anvil configuration
4. ✅ All existing tests pass
5. ✅ New tests cover legacy transaction paths

---

## Implementation Progress

### Change A: Split OPContractsManager into Phases ✅ COMPLETED

**Status:** Implemented in `packages/contracts-bedrock/src/L1/OPContractsManager.sol`

#### Changes Made

**1. Added Phase Output Structs (lines 2197-2234)**

Three new structs to hold intermediate deployment results:

```solidity
/// Phase 1 output: Singletons
struct DeployPhase1Output {
    IAddressManager addressManager;
    IProxyAdmin opChainProxyAdmin;
}

/// Phase 2 output: ERC-1967 Proxies (includes Phase 1)
struct DeployPhase2Output {
    IAddressManager addressManager;
    IProxyAdmin opChainProxyAdmin;
    IL1ERC721Bridge l1ERC721BridgeProxy;
    IOptimismPortal optimismPortalProxy;
    IETHLockbox ethLockboxProxy;
    ISystemConfig systemConfigProxy;
    IOptimismMintableERC20Factory optimismMintableERC20FactoryProxy;
    IDisputeGameFactory disputeGameFactoryProxy;
    IAnchorStateRegistry anchorStateRegistryProxy;
}

/// Phase 3 output: Legacy Proxies (includes Phase 2)
struct DeployPhase3Output {
    // All Phase 2 fields...
    IL1StandardBridge l1StandardBridgeProxy;
    IL1CrossDomainMessenger l1CrossDomainMessengerProxy;
    IDelayedWETH delayedWETHPermissionedGameProxy;
}
```

**2. Implemented Phase Functions in OPContractsManagerDeployer**

| Function | Lines | What it does | Est. Gas |
|----------|-------|--------------|----------|
| `deployPhase1()` | 1121-1154 | Deploy AddressManager, ProxyAdmin | ~500K |
| `deployPhase2()` | 1160-1194 | Deploy 7 ERC-1967 proxies | ~1.5M |
| `deployPhase3()` | 1200-1253 | Deploy 3 legacy proxies (ChugSplash, ResolvedDelegate) | ~1M |
| `deployPhase4()` | 1287-1396 | Initialize all proxies, transfer ownership, emit event | ~2-3M |
| `_phase3ToDeployOutput()` | 1257-1278 | Helper to convert Phase3Output → DeployOutput | pure |

**3. Preserved Original deploy() (lines 1402-1608)**

The original single-transaction `deploy()` remains **unchanged** for backwards compatibility with L1s that have high gas limits.

#### Usage

**For standard L1s (high gas limit):**
```solidity
output = opcmDeployer.deploy(input, superchainConfig, deployer);
```

**For gas-limited L1s (<4M per tx):**
```solidity
// Call each phase in separate transactions
phase1 = opcmDeployer.deployPhase1(input);
phase2 = opcmDeployer.deployPhase2(input, phase1);
phase3 = opcmDeployer.deployPhase3(input, phase2);
output = opcmDeployer.deployPhase4(input, phase3, superchainConfig, deployer);
```

#### Key Design Decisions

1. **Stateless approach:** Each phase takes the previous phase's output as a parameter, rather than storing state in contract storage. This saves gas (no SSTORE/SLOAD) and keeps the contract simpler.

2. **Cumulative outputs:** Each phase struct includes all fields from previous phases, so callers only need to track one struct at a time.

3. **Same salts:** All phase functions use the same `computeSalt()` logic as the original `deploy()`, ensuring **identical deterministic addresses** regardless of which approach is used.

4. **Backwards compatible:** The original `deploy()` function is preserved, so existing tooling continues to work.

#### Verification

```bash
# Compile to verify syntax
cd packages/contracts-bedrock
forge build src/L1/OPContractsManager.sol --skip test --skip script
```

#### Go Code Changes (op-deployer)

The following files were modified to support phased deployment:

1. **`op-deployer/pkg/deployer/opcm/opchain_phased.go`** (new file)
   - Go types for phase inputs/outputs
   - Conversion functions between phased and standard formats

2. **`op-deployer/pkg/deployer/opcm/scripts.go`**
   - Added `DeployPhase1` through `DeployPhase4` scripts

3. **`op-deployer/pkg/deployer/flags.go`**
   - Added `--phased-deployment` CLI flag

4. **`op-deployer/pkg/deployer/apply.go`**
   - Passes `PhasedDeployment` flag through pipeline

5. **`op-deployer/pkg/deployer/pipeline/env.go`**
   - Added `PhasedDeployment` field to `Env` struct

6. **`op-deployer/pkg/deployer/pipeline/opchain.go`**
   - Added `deployOPChainPhased()` function
   - Modified `DeployOPChain()` to use phased deployment when flag is set

7. **`packages/contracts-bedrock/scripts/deploy/DeployOPChainPhased.s.sol`** (new file)
   - Forge script with `runPhase1` through `runPhase4` functions

8. **`packages/contracts-bedrock/interfaces/L1/IOPContractsManager.sol`**
   - Added phase output structs and function signatures to `IOPContractsManagerDeployer` interface

#### Building and Embedding Modified Contracts

When you modify Solidity contracts in `packages/contracts-bedrock`, you must update the **embedded artifacts** in `op-deployer`. This is because `op-deployer` bundles pre-compiled contract artifacts inside its binary.

**What are embedded artifacts?**

The `op-deployer` binary contains a compressed tarball (`artifacts.tzst`) of pre-compiled Forge artifacts. When `intent.toml` uses `l1ContractsLocator = "embedded"`, the deployer extracts these bundled artifacts at runtime—no external files needed.

**Why rebuilding is required:**

If you modify contracts (like adding the phased deployment functions to `OPContractsManager.sol`), the embedded artifacts won't include your changes. The deployer will fail when trying to load scripts or contracts that don't exist in the embedded bundle.

**How to rebuild embedded artifacts:**

```bash
# Step 1: Build the modified contracts
cd packages/contracts-bedrock
forge build

# Step 2: Create new embedded tarball with your changes
cd ../../op-deployer
just copy-contract-artifacts

# Step 3: Rebuild op-deployer with new embedded artifacts
just build-go

# Verify the new tarball was created (size should change if contracts changed)
ls -la pkg/deployer/artifacts/forge-artifacts/artifacts.tzst
```

**Alternative: Use file:// locator (development only)**

During development, you can skip rebuilding embedded artifacts by pointing directly to the local forge-artifacts directory. Note that `file://` URLs require **absolute paths**:

```toml
# intent.toml - for local development only
l1ContractsLocator = "file:///absolute/path/to/packages/contracts-bedrock/forge-artifacts"
l2ContractsLocator = "file:///absolute/path/to/packages/contracts-bedrock/forge-artifacts"
```

> ⚠️ **Note:** Relative paths don't work with `file://` URLs due to URL parsing rules. The relative portion is interpreted as a hostname, not a path.

For production deployments, always use `"embedded"` after rebuilding the artifacts—this ensures the binary is self-contained and portable.

#### Usage

```bash
# Deploy with phased deployment (4 transactions, <4M gas each)
op-deployer apply --phased-deployment --l1-rpc-url http://localhost:8545 --private-key <KEY>

# Environment variable
OP_DEPLOYER_PHASED_DEPLOYMENT=true op-deployer apply ...
```

## Deploying Contracts with Phased Deployment

This section provides step-by-step instructions to deploy the L1 rollup contracts using the phased deployment approach.

### Prerequisites

Ensure you have:
- Initialized git submodules: `git submodule update --init --recursive`
- Built the contracts: `cd packages/contracts-bedrock && forge build`
- Rebuilt embedded artifacts: `cd op-deployer && just copy-contract-artifacts && just build-go`
- A valid `intent.toml` in `rollup/deployer/.deployer/` (see below)

> **Note:** If `forge build` fails with errors like `Source "lib/solady/..." not found`, you need to initialize git submodules first.

**Creating intent.toml:**

```bash
# Create the deployer directory
mkdir -p rollup/deployer
cd rollup/deployer

# Initialize with custom intent type (for local Anvil)
../../op-deployer/bin/op-deployer init --intent-type custom --l2-chain-ids 42069

# Edit .deployer/intent.toml to configure:
# 1. Set l1ChainID = 31337 (Anvil's chain ID)
# 2. Update addresses to match your Anvil seed accounts
# 3. Remove [chains.customGasToken] section if present
```

See `rollup/deployer/.deployer/intent.toml` for a working example configured for Anvil seed 2.

### Step 1: Start Anvil

Start a local Ethereum chain with Anvil. Use the same mnemonic seed for reproducible addresses:

```bash
anvil --mnemonic-seed-unsafe 2 --hardfork cancun --block-time 15
```

For testing gas limits (after Change B is implemented):
```bash
anvil --mnemonic-seed-unsafe 2 --hardfork cancun --block-time 15 --gas-limit 6500000
```

### Step 2: Clean Previous Deployment State

If you've run a previous deployment, you **must** remove the old state file. The deployer will not overwrite an existing deployment.

```bash
cd rollup/deployer

# Remove previous deployment state and recreate empty state
rm -f .deployer/state.json

# Create fresh state.json (required for apply to work)
cat > .deployer/state.json << 'EOF'
{
  "version": 1,
  "opDeployerVersion": "v0.0.0-dev"
}
EOF

# Optionally remove address artifacts too
rm -rf address/
```

> ⚠️ **Important:** You must both remove the old `state.json` AND create a fresh one. The deployer won't work without `state.json`, but an old one will cause it to skip already-deployed stages.

### Step 3: Verify intent.toml Configuration

Ensure your `intent.toml` uses the correct locator:

```toml
# For production (after rebuilding embedded artifacts)
l1ContractsLocator = "embedded"
l2ContractsLocator = "embedded"
```

Also verify the addresses match your Anvil seed. For `--mnemonic-seed-unsafe 2`:

| Account | Address | Role |
|---------|---------|------|
| 0 | `0x8D00bd3191EA20E6Ef3E859155f4FA56f5c28b52` | ProxyAdminOwner, FeeRecipients |
| 1 | `0x425A9E418efBE8F55e70E7D2BE0B02f8d2A5c08f` | Guardian, SystemConfigOwner |
| 2 | `0x8995E44a22e303A79bdD2E6e41674fb92d620863` | ProtocolVersionsOwner, UnsafeBlockSigner |
| 3 | `0xdEBD3CFCd414E09D5c97b020dE7cE74d6c4e6DC9` | Batcher |
| 4 | `0xa683a3E33E07fb84ff33FcE753Da1d248298977f` | Proposer, Challenger |

### Step 4: Run Phased Deployment

Deploy using the phased approach (4 separate transactions for OPContractsManager):

```bash
cd rollup/deployer

# Get the deployer private key (Anvil Account 0)
PRIVATE_KEY=0xbe62250c9db006c67c1595ff1f019bc849e2aa5c092dea0bf00883b39e54e904

# Run phased deployment
# Note: --workdir .deployer because intent.toml is in .deployer/
../../op-deployer/bin/op-deployer apply \
  --workdir .deployer \
  --l1-rpc-url http://localhost:8545 \
  --private-key $PRIVATE_KEY \
  --phased-deployment
```

You should see output indicating each deployment stage completing successfully.

### Step 5: Verify Deployment

After deployment, verify the contracts were deployed correctly by inspecting `state.json`:

```bash
cd rollup/deployer

# Check that state.json was populated with deployment data
jq 'keys' .deployer/state.json
# Should show: ["version", "opDeployerVersion", "superchainContracts", "implementationsDeployment", "opChainDeployments", ...]

# View superchain contract addresses
jq '.superchainContracts' .deployer/state.json

# View implementation contract addresses (OPCM)
jq '{OpcmImpl: .implementationsDeployment.OpcmImpl, OpcmDeployerImpl: .implementationsDeployment.OpcmDeployerImpl}' .deployer/state.json

# View OP Chain contract addresses (proxies)
jq '.opChainDeployments[0] | {OptimismPortalProxy, SystemConfigProxy, L1StandardBridgeProxy, L1CrossDomainMessengerProxy, DisputeGameFactoryProxy, AddressManagerImpl, OpChainProxyAdminImpl}' .deployer/state.json
```

#### Key Addresses to Verify

**Superchain Contracts:**
```bash
jq '{
  SuperchainConfigProxy: .superchainContracts.SuperchainConfigProxy,
  ProtocolVersionsProxy: .superchainContracts.ProtocolVersionsProxy
}' .deployer/state.json
```

**OPCM (Implementation):**
```bash
jq '{
  OpcmImpl: .implementationsDeployment.OpcmImpl,
  OpcmDeployerImpl: .implementationsDeployment.OpcmDeployerImpl
}' .deployer/state.json
```

**OP Chain Proxies (your L2's L1 contracts):**
```bash
jq '.opChainDeployments[0] | {
  OptimismPortalProxy,
  SystemConfigProxy,
  L1StandardBridgeProxy,
  L1CrossDomainMessengerProxy,
  DisputeGameFactoryProxy,
  AddressManagerImpl,
  OpChainProxyAdminImpl
}' .deployer/state.json
```

### Step 6: Verify On-Chain (Optional)

You can also verify the contracts exist on-chain:

```bash
# Check if a contract has code
cast code 0x<ADDRESS> --rpc-url http://localhost:8545

# Example: Check OptimismPortalProxy
PORTAL=$(jq -r '.opChainDeployments[0].OptimismPortalProxy' .deployer/state.json)
cast code $PORTAL --rpc-url http://localhost:8545 | head -c 100
# Should output bytecode, not "0x"
```

### Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| "state file already exists" | Previous deployment not cleaned | `rm -rf .deployer/state.json` |
| "script not found" | Embedded artifacts out of date | Rebuild: `just copy-contract-artifacts && just build-go` |
| "nonce too low" | Transaction already pending | Wait or restart Anvil |
| "gas exceeds limit" | L1 gas limit too low | Use higher `--gas-limit` for Anvil |
| Phase transaction reverts | Contract error | Check Anvil logs for revert reason |

---

### Change B: Reduce Large CREATE2 Deployments ⏳ PENDING

**Status:** Not yet implemented

The CREATE2 deployments in blocks 14 and 15 (4.5M and 5.4M gas) still exceed 4M. This requires implementing a factory pattern to deploy large bytecode in chunks.

---

### Requirement 2: Legacy Transactions ⏳ PENDING

**Status:** Not yet implemented

The txmgr modifications to support `--txmgr.use-legacy-tx` flag have not been implemented yet.

