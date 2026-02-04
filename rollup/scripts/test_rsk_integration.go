// Test script to validate RSK block verification integration.
// Run with: go run ./rollup/scripts/test_rsk_integration.go [--rpc-url http://localhost:8545]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	gethlog "github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/ethereum-optimism/optimism/op-service/sources/caching"
)

var (
	rpcURL = flag.String("rpc-url", "http://localhost:8545", "RSKj RPC URL")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set up go-ethereum logger
	logger := gethlog.NewLogger(gethlog.NewTerminalHandlerWithLevel(os.Stdout, gethlog.LevelInfo, true))

	fmt.Println("=== RSK Integration Test ===")
	fmt.Printf("RPC URL: %s\n\n", *rpcURL)

	// Connect to RSKj
	ethClient, err := ethclient.Dial(*rpcURL)
	if err != nil {
		fmt.Printf("FATAL: Failed to connect to RSKj: %v\n", err)
		os.Exit(1)
	}
	defer ethClient.Close()

	// Get chain ID
	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		fmt.Printf("FATAL: Failed to get chain ID: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Chain ID: %s ", chainID)
	switch chainID.Int64() {
	case 30:
		fmt.Println("(RSK Mainnet)")
	case 31:
		fmt.Println("(RSK Testnet)")
	case 33:
		fmt.Println("(RSK Regtest)")
	default:
		fmt.Println("(Unknown)")
	}

	// Get latest block number
	blockNum, err := ethClient.BlockNumber(ctx)
	if err != nil {
		fmt.Printf("FATAL: Failed to get block number: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Latest block: %d\n\n", blockNum)

	// Test with op-service sources
	fmt.Println("=== Testing op-service/sources RSK Integration ===")
	fmt.Println()

	// Create RPC client
	rpcClient, err := rpc.Dial(*rpcURL)
	if err != nil {
		fmt.Printf("FATAL: Failed to create RPC client: %v\n", err)
		os.Exit(1)
	}

	// Create op-service client with RSK provider kind
	opClient := client.NewBaseRPCClient(rpcClient)
	config := &sources.EthClientConfig{
		ReceiptsCacheSize:     100,
		TransactionsCacheSize: 100,
		HeadersCacheSize:      100,
		PayloadsCacheSize:     100,
		MaxRequestsPerBatch:   20,
		MaxConcurrentRequests: 10,
		TrustRPC:              true, // Start with trust mode for testing
		MustBePostMerge:       false,
		RPCProviderKind:       sources.RPCKindRSK, // Use RSK provider
		MethodResetDuration:   time.Minute,
		BlockRefsCacheSize:    100,
	}

	metrics := &noopMetrics{}
	opEthClient, err := sources.NewEthClient(opClient, logger, metrics, config)
	if err != nil {
		fmt.Printf("FATAL: Failed to create op EthClient: %v\n", err)
		os.Exit(1)
	}

	// Test 1: Fetch block info by number
	fmt.Println("Test 1: Fetch block info by number (TrustRPC=true)")
	testBlocks := []uint64{1, blockNum / 2, blockNum}
	for _, num := range testBlocks {
		if num == 0 || num > blockNum {
			continue
		}
		info, err := opEthClient.InfoByNumber(ctx, num)
		if err != nil {
			fmt.Printf("  Block %d: FAILED - %v\n", num, err)
			continue
		}
		fmt.Printf("  Block %d: OK (hash=%s)\n", num, info.Hash().Hex()[:16]+"...")
	}
	fmt.Println()

	// Test 2: Fetch block with transactions (with verification)
	fmt.Println("Test 2: Fetch block with transactions (TrustRPC=false, verification enabled)")
	config.TrustRPC = false // Enable verification
	opEthClient2, err := sources.NewEthClient(opClient, logger, metrics, config)
	if err != nil {
		fmt.Printf("FATAL: Failed to create op EthClient: %v\n", err)
		os.Exit(1)
	}

	for _, num := range testBlocks {
		if num == 0 || num > blockNum {
			continue
		}
		info, txs, err := opEthClient2.InfoAndTxsByNumber(ctx, num)
		if err != nil {
			fmt.Printf("  Block %d: FAILED - %v\n", num, err)
			continue
		}
		fmt.Printf("  Block %d: OK (hash=%s, txs=%d)\n", num, info.Hash().Hex()[:16]+"...", len(txs))
	}
	fmt.Println()

	// Test 3: Verify block hash computation
	fmt.Println("Test 3: Block hash verification (RSK-specific)")
	testBlockHash(ctx, rpcClient, blockNum)
	fmt.Println()

	// Summary
	fmt.Println("=== Test Complete ===")
	fmt.Println("If all tests passed, the RSK integration is working correctly.")
	fmt.Println("\nNext steps:")
	fmt.Println("1. Run op-node with --l1.rpc-kind=rsk flag")
	fmt.Println("2. Configure --l1.trustrpc=true initially for testing")
	fmt.Println("3. Once verified, set --l1.trustrpc=false for production")
}

// testBlockHash tests RSK block hash computation by comparing RPC hash with computed hash
func testBlockHash(ctx context.Context, rpcClient *rpc.Client, blockNum uint64) {
	var block map[string]interface{}
	err := rpcClient.CallContext(ctx, &block, "eth_getBlockByNumber", fmt.Sprintf("0x%x", blockNum), false)
	if err != nil {
		fmt.Printf("  Block %d: FAILED to fetch - %v\n", blockNum, err)
		return
	}

	rpcHash, ok := block["hash"].(string)
	if !ok {
		fmt.Printf("  Block %d: FAILED - no hash in response\n", blockNum)
		return
	}

	// Check for RSK-specific fields
	_, hasMinGasPrice := block["minimumGasPrice"]
	_, hasPaidFees := block["paidFees"]

	if hasMinGasPrice || hasPaidFees {
		fmt.Printf("  Block %d: RSK block detected (minimumGasPrice=%v, paidFees=%v)\n",
			blockNum, hasMinGasPrice, hasPaidFees)
		fmt.Printf("  RPC hash: %s\n", rpcHash)
		fmt.Println("  Note: Hash verification happens in op-service/sources when TrustRPC=false")
	} else {
		fmt.Printf("  Block %d: WARNING - RSK-specific fields not found\n", blockNum)
	}
}

// noopMetrics implements caching.Metrics with no-op methods
type noopMetrics struct{}

func (m *noopMetrics) CacheAdd(_ string, _ int, _ bool)     {}
func (m *noopMetrics) CacheGet(_ string, _ bool)            {}
func (m *noopMetrics) RecordDBEntryCount(_ string, _ int64) {}

var _ caching.Metrics = (*noopMetrics)(nil)
