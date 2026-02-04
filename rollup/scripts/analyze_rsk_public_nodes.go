// Analyze RSK blocks from public nodes to understand structure and test hash computation.
// Run with: go run ./rollup/scripts/analyze_rsk_public_nodes.go
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"

	"gorsk/rskblocks"

	"github.com/ethereum-optimism/optimism/op-service/rsk"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	nodes := []struct {
		name string
		url  string
	}{
		{"RSK Mainnet", "https://public-node.rsk.co"},
		{"RSK Testnet", "https://public-node.testnet.rsk.co"},
	}

	for _, node := range nodes {
		fmt.Printf("\n{'='*60}\n")
		fmt.Printf("=== %s ===\n", node.name)
		fmt.Printf("URL: %s\n", node.url)
		fmt.Println("=" + string(make([]byte, 59)))

		analyzeNode(ctx, node.name, node.url)
	}
}

func analyzeNode(ctx context.Context, name, url string) {
	rpcClient, err := rpc.Dial(url)
	if err != nil {
		fmt.Printf("ERROR: Failed to connect: %v\n", err)
		return
	}
	defer rpcClient.Close()

	// Get latest block number
	var blockNumHex string
	err = rpcClient.CallContext(ctx, &blockNumHex, "eth_blockNumber")
	if err != nil {
		fmt.Printf("ERROR: Failed to get block number: %v\n", err)
		return
	}
	latestBlock, _ := hexutil.DecodeUint64(blockNumHex)
	fmt.Printf("Latest block: %d\n\n", latestBlock)

	// Analyze a recent block
	blockNum := latestBlock - 10 // A few blocks back to ensure it's confirmed
	fmt.Printf("--- Analyzing Block %d ---\n", blockNum)
	analyzeBlock(ctx, rpcClient, name, blockNum)
}

func analyzeBlock(ctx context.Context, rpcClient *rpc.Client, network string, blockNum uint64) {
	var block map[string]interface{}
	err := rpcClient.CallContext(ctx, &block, "eth_getBlockByNumber", fmt.Sprintf("0x%x", blockNum), false)
	if err != nil {
		fmt.Printf("ERROR: Failed to fetch block: %v\n", err)
		return
	}

	// Print key fields
	rpcHash := block["hash"].(string)
	fmt.Printf("RPC Hash: %s\n\n", rpcHash)

	// Check for RSK-specific fields
	fields := []string{
		"parentHash", "sha3Uncles", "miner", "stateRoot",
		"transactionsRoot", "receiptsRoot", "logsBloom",
		"difficulty", "number", "gasLimit", "gasUsed",
		"timestamp", "extraData", "mixHash", "nonce",
		"paidFees", "minimumGasPrice", "uncleCount",
		"ummRoot", "txExecutionSublistsEdges", "rskPteEdges",
		"bitcoinMergedMiningHeader", "bitcoinMergedMiningMerkleProof",
		"bitcoinMergedMiningCoinbaseTransaction",
	}

	fmt.Println("=== Field Analysis ===")
	for _, field := range fields {
		val, ok := block[field]
		if !ok || val == nil {
			fmt.Printf("  %-40s: <not present>\n", field)
		} else {
			valStr := fmt.Sprintf("%v", val)
			if len(valStr) > 60 {
				valStr = valStr[:60] + "..."
			}
			fmt.Printf("  %-40s: %s\n", field, valStr)
		}
	}

	// Try to compute hash
	fmt.Println("\n=== Hash Computation Test ===")

	// Parse values
	parentHash := getString(block, "parentHash")
	sha3Uncles := getString(block, "sha3Uncles")
	miner := getString(block, "miner")
	stateRoot := getString(block, "stateRoot")
	txRoot := getString(block, "transactionsRoot")
	receiptsRoot := getString(block, "receiptsRoot")
	logsBloom := getString(block, "logsBloom")
	difficulty := getString(block, "difficulty")
	number := getString(block, "number")
	gasLimit := getString(block, "gasLimit")
	gasUsed := getString(block, "gasUsed")
	timestamp := getString(block, "timestamp")
	extraData := getString(block, "extraData")
	paidFees := getString(block, "paidFees")
	minGasPrice := getString(block, "minimumGasPrice")
	btcHeader := getString(block, "bitcoinMergedMiningHeader")
	btcProof := getString(block, "bitcoinMergedMiningMerkleProof")
	btcCoinbase := getString(block, "bitcoinMergedMiningCoinbaseTransaction")

	// Parse values
	difficultyBig, _ := hexutil.DecodeBig(difficulty)
	numberBig, _ := hexutil.DecodeBig(number)
	gasLimitBig, _ := hexutil.DecodeBig(gasLimit)
	gasUsedBig, _ := hexutil.DecodeBig(gasUsed)
	timestampBig, _ := hexutil.DecodeBig(timestamp)
	paidFeesBig, _ := hexutil.DecodeBig(paidFees)
	minGasPriceBig, _ := hexutil.DecodeBig(minGasPrice)
	extraDataBytes, _ := hexutil.Decode(extraData)
	logsBloomBytes, _ := hexutil.Decode(logsBloom)
	btcHeaderBytes, _ := hexutil.Decode(btcHeader)
	btcProofBytes, _ := hexutil.Decode(btcProof)
	btcCoinbaseBytes, _ := hexutil.Decode(btcCoinbase)

	// Get uncle count
	uncleCount := 0
	if uncles, ok := block["uncles"].([]interface{}); ok {
		uncleCount = len(uncles)
	}

	// Check for ummRoot and edges
	// Note: use explicit nil check, not just key existence
	ummRootVal, hasUmmRoot := block["ummRoot"]
	hasUmmRoot = hasUmmRoot && ummRootVal != nil

	edgesVal, hasEdges := block["rskPteEdges"]
	hasEdges = hasEdges && edgesVal != nil

	edges2Val, hasEdges2 := block["txExecutionSublistsEdges"]
	hasEdges2 = hasEdges2 && edges2Val != nil

	fmt.Printf("ummRoot present: %v\n", hasUmmRoot)
	fmt.Printf("rskPteEdges present: %v (value: %v)\n", hasEdges, edgesVal)
	fmt.Printf("txExecutionSublistsEdges present: %v\n", hasEdges2)

	// Create BlockHeaderInput
	var bloom [256]byte
	copy(bloom[:], logsBloomBytes)

	input := &rskblocks.BlockHeaderInput{
		ParentHash:      common.HexToHash(parentHash),
		UnclesHash:      common.HexToHash(sha3Uncles),
		Coinbase:        common.HexToAddress(miner),
		StateRoot:       common.HexToHash(stateRoot),
		TxTrieRoot:      common.HexToHash(txRoot),
		ReceiptTrieRoot: common.HexToHash(receiptsRoot),
		LogsBloom:       bloom,
		Difficulty:      difficultyBig,
		Number:          numberBig,
		GasLimit:        gasLimitBig,
		GasUsed:         gasUsedBig,
		Timestamp:       timestampBig,
		ExtraData:       extraDataBytes,
		PaidFees:        paidFeesBig,
		MinimumGasPrice: minGasPriceBig,
		UncleCount:      uncleCount,

		BitcoinMergedMiningHeader:              btcHeaderBytes,
		BitcoinMergedMiningMerkleProof:         btcProofBytes,
		BitcoinMergedMiningCoinbaseTransaction: btcCoinbaseBytes,

		// TxExecutionSublistsEdges: nil means "don't include edges"
		// Only set if RPC returns the field (even if empty array)
	}

	// Parse edges if present - check both field names
	if edges, ok := block["rskPteEdges"].([]interface{}); ok {
		// Field exists - include even if empty
		input.TxExecutionSublistsEdges = make([]int16, len(edges))
		for i, e := range edges {
			if num, ok := e.(float64); ok {
				input.TxExecutionSublistsEdges[i] = int16(num)
			}
		}
	} else if edges, ok := block["txExecutionSublistsEdges"].([]interface{}); ok {
		// Check alternative field name
		input.TxExecutionSublistsEdges = make([]int16, len(edges))
		for i, e := range edges {
			if num, ok := e.(float64); ok {
				input.TxExecutionSublistsEdges[i] = int16(num)
			}
		}
	}
	// If neither field present, leave as nil (edges not included in encoding)

	fmt.Println("\n=== Testing Hash Configs ===")

	// Determine network config
	var networkName string
	if network == "RSK Mainnet" {
		networkName = "mainnet"
	} else {
		networkName = "testnet"
	}

	// Get config for this block
	gorskConfig := rskblocks.ConfigForBlockNumber(int64(blockNum), networkName)
	fmt.Printf("gorsk config for block %d on %s:\n", blockNum, networkName)
	fmt.Printf("  UseRskip92Encoding: %v\n", gorskConfig.UseRskip92Encoding)
	fmt.Printf("  Version: %d\n", gorskConfig.Version)
	fmt.Printf("  IncludeUmmRoot: %v\n", gorskConfig.IncludeUmmRoot)

	// Test with gorsk config
	hash := rskblocks.ComputeBlockHash(input, gorskConfig)
	match := ""
	if hash.Hex() == rpcHash {
		match = " *** MATCH! ***"
	}
	fmt.Printf("\ngorsk computed hash: %s%s\n", hash.Hex(), match)
	fmt.Printf("Expected (RPC):      %s\n", rpcHash)

	// Test other configs
	configs := []struct {
		name   string
		config rskblocks.BlockHashConfig
	}{
		{"V0 + RSKIP92 + no UMM", rskblocks.BlockHashConfig{UseRskip92Encoding: true, Version: 0, IncludeUmmRoot: false}},
		{"V0 + RSKIP92 + UMM", rskblocks.BlockHashConfig{UseRskip92Encoding: true, Version: 0, IncludeUmmRoot: true}},
		{"V1 + RSKIP92 + UMM", rskblocks.BlockHashConfig{UseRskip92Encoding: true, Version: 1, IncludeUmmRoot: true}},
		{"V1 + RSKIP92 + no UMM", rskblocks.BlockHashConfig{UseRskip92Encoding: true, Version: 1, IncludeUmmRoot: false}},
	}

	fmt.Println("\n=== All Config Tests ===")
	for _, tc := range configs {
		hash := rskblocks.ComputeBlockHash(input, tc.config)
		match := ""
		if hash.Hex() == rpcHash {
			match = " *** MATCH! ***"
		}
		fmt.Printf("  %-25s: %s%s\n", tc.name, hash.Hex()[:20]+"...", match)
	}

	// Print encoded header for V1 + UMM (what we expect for recent blocks)
	fmt.Println("\n=== Encoded Header (V1 + RSKIP92 + UMM) ===")
	encoded := rskblocks.GetEncodedBlockHeader(input, rskblocks.BlockHashConfig{UseRskip92Encoding: true, Version: 1, IncludeUmmRoot: true})
	fmt.Printf("Length: %d bytes\n", len(encoded))
	fmt.Printf("First 100 bytes: %s\n", hex.EncodeToString(encoded[:min(100, len(encoded))]))

	// Also test with op-service/rsk wrapper
	fmt.Println("\n=== Using op-service/rsk wrapper ===")
	rskHeader := &rsk.RSKRPCHeader{
		ParentHash:      common.HexToHash(parentHash),
		UncleHash:       common.HexToHash(sha3Uncles),
		Coinbase:        common.HexToAddress(miner),
		Root:            common.HexToHash(stateRoot),
		TxHash:          common.HexToHash(txRoot),
		ReceiptHash:     common.HexToHash(receiptsRoot),
		Difficulty:      hexutil.Big(*difficultyBig),
		Number:          hexutil.Uint64(numberBig.Uint64()),
		GasLimit:        hexutil.Uint64(gasLimitBig.Uint64()),
		GasUsed:         hexutil.Uint64(gasUsedBig.Uint64()),
		Time:            hexutil.Uint64(timestampBig.Uint64()),
		Extra:           extraDataBytes,
		PaidFees:        (*hexutil.Big)(paidFeesBig),
		MinimumGasPrice: (*hexutil.Big)(minGasPriceBig),
		UncleCount:      hexutil.Uint64(uncleCount),
		Hash:            common.HexToHash(rpcHash),

		BitcoinMergedMiningHeader:              btcHeaderBytes,
		BitcoinMergedMiningMerkleProof:         btcProofBytes,
		BitcoinMergedMiningCoinbaseTransaction: btcCoinbaseBytes,
	}
	copy(rskHeader.Bloom[:], logsBloomBytes)

	// Handle ummRoot
	if hasUmmRoot {
		ummRootStr := getString(block, "ummRoot")
		if ummRootStr != "" && ummRootStr != "0x" {
			ummHash := common.HexToHash(ummRootStr)
			rskHeader.UmmRoot = &ummHash
		}
	}

	// Use mainnet config for high block numbers
	var rskConfig rsk.RSKNetworkConfig
	if blockNum > 5000000 {
		rskConfig = rsk.MainnetConfig()
	} else {
		rskConfig = rsk.DefaultRegtestConfig()
	}

	computedHash, err := rsk.ComputeRSKBlockHash(rskHeader, rskConfig)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		match := ""
		if computedHash.Hex() == rpcHash {
			match = " *** MATCH! ***"
		}
		fmt.Printf("Computed hash: %s%s\n", computedHash.Hex(), match)
	}
}

func getString(block map[string]interface{}, key string) string {
	if val, ok := block[key]; ok && val != nil {
		return val.(string)
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
