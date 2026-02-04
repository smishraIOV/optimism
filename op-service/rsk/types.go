// Package rsk provides RSK-specific types and utilities for op-stack integration.
// RSK (Rootstock) is an EVM-compatible Bitcoin sidechain with different block header
// encoding, trie structure (binary vs hexary), and hash computation rules.
package rsk

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// RSKRPCHeader extends the standard Ethereum RPC header with RSK-specific fields.
// RSK block headers contain additional fields not present in Ethereum:
// - PaidFees: Total fees paid in this block
// - MinimumGasPrice: Minimum gas price for transactions
// - UncleCount: Number of uncles (RSK uses this differently)
// - UmmRoot: Unified Mining Merkle root (RSKIP-UMM)
// - TxExecutionSublistsEdges: Parallel transaction execution edges (RSKIP-144)
// - Bitcoin merged mining fields
type RSKRPCHeader struct {
	// Standard Ethereum header fields
	ParentHash  common.Hash      `json:"parentHash"`
	UncleHash   common.Hash      `json:"sha3Uncles"`
	Coinbase    common.Address   `json:"miner"`
	Root        common.Hash      `json:"stateRoot"`
	TxHash      common.Hash      `json:"transactionsRoot"`
	ReceiptHash common.Hash      `json:"receiptsRoot"`
	Bloom       eth.Bytes256     `json:"logsBloom"`
	Difficulty  hexutil.Big      `json:"difficulty"`
	Number      hexutil.Uint64   `json:"number"`
	GasLimit    hexutil.Uint64   `json:"gasLimit"`
	GasUsed     hexutil.Uint64   `json:"gasUsed"`
	Time        hexutil.Uint64   `json:"timestamp"`
	Extra       hexutil.Bytes    `json:"extraData"`
	MixDigest   common.Hash      `json:"mixHash"`
	Nonce       types.BlockNonce `json:"nonce"`

	// RSK-specific fields
	PaidFees        *hexutil.Big   `json:"paidFees"`
	MinimumGasPrice *hexutil.Big   `json:"minimumGasPrice"`
	UncleCount      hexutil.Uint64 `json:"uncleCount"`

	// UMM root (RSKIP-UMM) - may be null for pre-UMM blocks
	UmmRoot *common.Hash `json:"ummRoot,omitempty"`

	// RSKIP-144: Parallel transaction execution edges
	TxExecutionSublistsEdges []hexutil.Uint64 `json:"txExecutionSublistsEdges,omitempty"`

	// RSKIP-535: Base event for V2 headers - pointer to handle null from RPC
	BaseEvent *hexutil.Bytes `json:"baseEvent,omitempty"`

	// Bitcoin merged mining fields
	BitcoinMergedMiningHeader              hexutil.Bytes `json:"bitcoinMergedMiningHeader,omitempty"`
	BitcoinMergedMiningMerkleProof         hexutil.Bytes `json:"bitcoinMergedMiningMerkleProof,omitempty"`
	BitcoinMergedMiningCoinbaseTransaction hexutil.Bytes `json:"bitcoinMergedMiningCoinbaseTransaction,omitempty"`

	// Hash from RPC (may need verification)
	Hash common.Hash `json:"hash"`
}

// RSKRPCBlock represents an RSK block with header and transactions.
type RSKRPCBlock struct {
	RSKRPCHeader
	Transactions []*types.Transaction `json:"transactions"`
}

// RSKNetworkConfig holds RSK network-specific configuration.
type RSKNetworkConfig struct {
	// Network name: "mainnet", "testnet", or "regtest"
	Network string

	// ChainID for the RSK network
	ChainID *big.Int

	// RSKIP activation heights (0 means active from genesis, -1 means not activated)
	RSKIP92ActivationHeight  int64 // Merged mining encoding change (Orchid)
	RSKIP351ActivationHeight int64 // V1 header format (Reed810)
	RSKIP535ActivationHeight int64 // V2 header format with baseEvent (Vetiver900)
	UMMActivationHeight      int64 // Unified Mining Merkle (Papyrus200)
	RSKIP144ActivationHeight int64 // Parallel tx execution edges (Reed810)

	// Use4ByteGasLimit: true for regtest (4-byte with leading zeros), false for mainnet/testnet (minimal bytes)
	Use4ByteGasLimit bool
}

// DefaultRegtestConfig returns the default RSK regtest configuration.
// In regtest, all RSKIPs are active from block 0, including V2 headers (RSKIP-535).
func DefaultRegtestConfig() RSKNetworkConfig {
	return RSKNetworkConfig{
		Network:                  "regtest",
		ChainID:                  big.NewInt(33), // RSK regtest chain ID
		RSKIP92ActivationHeight:  0,
		RSKIP351ActivationHeight: 0, // V1 active from genesis
		RSKIP535ActivationHeight: 0, // V2 active from genesis
		UMMActivationHeight:      0,
		RSKIP144ActivationHeight: 0,
		Use4ByteGasLimit:         true, // Regtest uses 4-byte gasLimit
	}
}

// MainnetConfig returns the RSK mainnet configuration.
// Note: RSKIP-351 (V1) and RSKIP-535 (V2) are NOT YET ACTIVATED on mainnet.
func MainnetConfig() RSKNetworkConfig {
	return RSKNetworkConfig{
		Network:                  "mainnet",
		ChainID:                  big.NewInt(30), // RSK mainnet chain ID
		RSKIP92ActivationHeight:  729000,         // Orchid activation
		RSKIP351ActivationHeight: -1,             // NOT ACTIVATED (reed810 = -1)
		RSKIP535ActivationHeight: -1,             // NOT ACTIVATED (vetiver900 = -1)
		UMMActivationHeight:      2392700,        // Papyrus200 activation
		RSKIP144ActivationHeight: -1,             // NOT ACTIVATED (reed810 = -1)
		Use4ByteGasLimit:         false,          // Mainnet uses minimal gasLimit
	}
}

// TestnetConfig returns the RSK testnet configuration.
// RSKIP-351 (V1) activated at reed810 = 7139600, RSKIP-535 (V2) NOT YET ACTIVATED.
func TestnetConfig() RSKNetworkConfig {
	return RSKNetworkConfig{
		Network:                  "testnet",
		ChainID:                  big.NewInt(31), // RSK testnet chain ID
		RSKIP92ActivationHeight:  0,              // Orchid active from genesis
		RSKIP351ActivationHeight: 7139600,        // Reed810 activation (V1)
		RSKIP535ActivationHeight: -1,             // NOT ACTIVATED (vetiver900 = -1)
		UMMActivationHeight:      863000,         // Papyrus200 activation
		RSKIP144ActivationHeight: 7139600,        // Reed810 activation
		Use4ByteGasLimit:         false,          // Testnet uses minimal gasLimit
	}
}

// GetConfigForChainID returns the RSK network config for the given chain ID.
func GetConfigForChainID(chainID *big.Int) RSKNetworkConfig {
	if chainID == nil {
		return DefaultRegtestConfig()
	}
	switch chainID.Int64() {
	case 30:
		return MainnetConfig()
	case 31:
		return TestnetConfig()
	case 33:
		return DefaultRegtestConfig()
	default:
		// Default to regtest config for unknown chain IDs
		return DefaultRegtestConfig()
	}
}

// IsRSKChain returns true if the chain ID corresponds to an RSK network.
func IsRSKChain(chainID *big.Int) bool {
	if chainID == nil {
		return false
	}
	id := chainID.Int64()
	return id == 30 || id == 31 || id == 33
}
