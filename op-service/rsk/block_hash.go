package rsk

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	// Import gorsk for RSK block hash computation
	"gorsk/rskblocks"
)

// ComputeRSKBlockHash computes the block hash for an RSK block header.
// This uses RSK's specific encoding rules which differ from Ethereum:
// - RSKIP-92: Excludes merged mining merkle proof and coinbase from hash
// - RSKIP-351: V1 headers use extensionData instead of raw logsBloom
// - Different RLP encoding for certain fields (gasLimit, minimumGasPrice)
func ComputeRSKBlockHash(header *RSKRPCHeader, config RSKNetworkConfig) (common.Hash, error) {
	// Convert RSKRPCHeader to gorsk BlockHeaderInput
	input := rpcHeaderToBlockInput(header)

	// Get block hash config based on block number and network
	blockNum := int64(header.Number)
	hashConfig := getBlockHashConfig(blockNum, config)

	// Compute the hash using gorsk
	hash := rskblocks.ComputeBlockHash(input, hashConfig)

	return hash, nil
}

// VerifyRSKBlockHash verifies that the RPC-provided hash matches the computed hash.
func VerifyRSKBlockHash(header *RSKRPCHeader, config RSKNetworkConfig) error {
	computed, err := ComputeRSKBlockHash(header, config)
	if err != nil {
		return fmt.Errorf("failed to compute RSK block hash: %w", err)
	}

	if computed != header.Hash {
		return fmt.Errorf("RSK block hash mismatch: computed %s but RPC said %s", computed.Hex(), header.Hash.Hex())
	}

	return nil
}

// rpcHeaderToBlockInput converts an RSKRPCHeader to a gorsk BlockHeaderInput.
func rpcHeaderToBlockInput(header *RSKRPCHeader) *rskblocks.BlockHeaderInput {
	input := &rskblocks.BlockHeaderInput{
		ParentHash:      header.ParentHash,
		UnclesHash:      header.UncleHash,
		Coinbase:        header.Coinbase,
		StateRoot:       header.Root,
		TxTrieRoot:      header.TxHash,
		ReceiptTrieRoot: header.ReceiptHash,
		Difficulty:      (*big.Int)(&header.Difficulty),
		Number:          big.NewInt(int64(header.Number)),
		GasLimit:        big.NewInt(int64(header.GasLimit)),
		GasUsed:         big.NewInt(int64(header.GasUsed)),
		Timestamp:       big.NewInt(int64(header.Time)),
		ExtraData:       header.Extra,
		UncleCount:      int(header.UncleCount),

		// Bitcoin merged mining fields
		BitcoinMergedMiningHeader:              header.BitcoinMergedMiningHeader,
		BitcoinMergedMiningMerkleProof:         header.BitcoinMergedMiningMerkleProof,
		BitcoinMergedMiningCoinbaseTransaction: header.BitcoinMergedMiningCoinbaseTransaction,
	}

	// Copy logsBloom
	copy(input.LogsBloom[:], header.Bloom[:])

	// RSK-specific fields
	if header.PaidFees != nil {
		input.PaidFees = (*big.Int)(header.PaidFees)
	} else {
		input.PaidFees = big.NewInt(0)
	}

	if header.MinimumGasPrice != nil {
		input.MinimumGasPrice = (*big.Int)(header.MinimumGasPrice)
	} else {
		input.MinimumGasPrice = big.NewInt(0)
	}

	// Convert TxExecutionSublistsEdges
	if len(header.TxExecutionSublistsEdges) > 0 {
		input.TxExecutionSublistsEdges = make([]int16, len(header.TxExecutionSublistsEdges))
		for i, edge := range header.TxExecutionSublistsEdges {
			input.TxExecutionSublistsEdges[i] = int16(edge)
		}
	}

	// RSKIP-535: BaseEvent for V2 headers
	if header.BaseEvent != nil && len(*header.BaseEvent) > 0 {
		input.BaseEvent = *header.BaseEvent
	}

	// UmmRoot - convert from *common.Hash to *[]byte
	if header.UmmRoot != nil {
		ummBytes := header.UmmRoot.Bytes()
		input.UmmRoot = &ummBytes
	}

	return input
}

// getBlockHashConfig returns the appropriate gorsk BlockHashConfig based on
// block number and network configuration.
func getBlockHashConfig(blockNum int64, config RSKNetworkConfig) rskblocks.BlockHashConfig {
	// Determine which RSKIPs are active at this block height
	// -1 means not activated
	useRskip92 := config.RSKIP92ActivationHeight >= 0 && blockNum >= config.RSKIP92ActivationHeight
	includeUmm := config.UMMActivationHeight >= 0 && blockNum >= config.UMMActivationHeight

	// Determine header version:
	// - V2 if RSKIP-535 is active (Vetiver900)
	// - V1 if RSKIP-351 is active but not RSKIP-535 (Reed810)
	// - V0 otherwise
	var version byte = 0
	if config.RSKIP535ActivationHeight >= 0 && blockNum >= config.RSKIP535ActivationHeight {
		version = 2
	} else if config.RSKIP351ActivationHeight >= 0 && blockNum >= config.RSKIP351ActivationHeight {
		version = 1
	}

	return rskblocks.BlockHashConfig{
		UseRskip92Encoding: useRskip92,
		Version:            version,
		IncludeUmmRoot:     includeUmm,
		Use4ByteGasLimit:   config.Use4ByteGasLimit,
	}
}
