package broadcaster

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/holiman/uint256"

	"github.com/ethereum-optimism/optimism/op-service/eth"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	opcrypto "github.com/ethereum-optimism/optimism/op-service/crypto"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum-optimism/optimism/op-service/txmgr/metrics"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/hashicorp/go-multierror"
)

const (
	GasPadFactor = 1.2
)

type KeyedBroadcaster struct {
	lgr    log.Logger
	mgr    txmgr.TxManager
	bcasts []script.Broadcast
	client *ethclient.Client
	mtx    sync.Mutex
}

type KeyedBroadcasterOpts struct {
	Logger  log.Logger
	ChainID *big.Int
	Client  *ethclient.Client
	Signer  opcrypto.SignerFn
	From    common.Address
}

func NewKeyedBroadcaster(cfg KeyedBroadcasterOpts) (*KeyedBroadcaster, error) {
	// Check if chain supports EIP-1559 by checking if BaseFee is present
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	head, err := cfg.Client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block header: %w", err)
	}
	// Auto-detect pre-EIP-1559 chains (like RSKj) by checking if BaseFee is nil.
	// These chains require legacy (Type 0) transactions instead of EIP-1559 (Type 2).
	useLegacyTx := head.BaseFee == nil

	mgrCfg := &txmgr.Config{
		Backend:                   cfg.Client,
		ChainID:                   cfg.ChainID,
		TxSendTimeout:             10 * time.Minute, // Increased for RSKj slow block times (~30s)
		TxNotInMempoolTimeout:     3 * time.Minute,  // Increased for RSKj
		NetworkTimeout:            30 * time.Second, // Increased for RSKj
		ReceiptQueryInterval:      3 * time.Second,  // Increased for RSKj
		NumConfirmations:          1,
		SafeAbortNonceTooLowCount: 20, // Increased for RSKj compatibility - slow block times cause race conditions
		Signer:                    cfg.Signer,
		From:                      cfg.From,
		GasPriceEstimatorFn:       DeployerGasPriceEstimator,
		UseLegacyTx:               useLegacyTx,
	}

	minTipCap, err := eth.GweiToWei(1.0)
	if err != nil {
		panic(err)
	}
	minBaseFee, err := eth.GweiToWei(1.0)
	if err != nil {
		panic(err)
	}

	mgrCfg.RebroadcastInterval.Store(int64(30 * time.Second)) // Increased for RSKj (~30s block time)
	mgrCfg.ResubmissionTimeout.Store(int64(90 * time.Second)) // Increased for RSKj
	mgrCfg.FeeLimitMultiplier.Store(5)
	mgrCfg.FeeLimitThreshold.Store(big.NewInt(100))
	mgrCfg.MinTipCap.Store(minTipCap)
	mgrCfg.MinBaseFee.Store(minBaseFee)

	mgr, err := txmgr.NewSimpleTxManagerFromConfig(
		"transactor",
		cfg.Logger,
		&metrics.NoopTxMetrics{},
		mgrCfg,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx manager: %w", err)
	}

	return &KeyedBroadcaster{
		lgr:    cfg.Logger,
		mgr:    mgr,
		client: cfg.Client,
	}, nil
}

func (t *KeyedBroadcaster) Hook(bcast script.Broadcast) {
	if bcast.Type != script.BroadcastCreate2 && bcast.From != t.mgr.From() {
		panic(fmt.Sprintf("invalid from for broadcast:%v, expected:%v", bcast.From, t.mgr.From()))
	}
	t.mtx.Lock()
	t.bcasts = append(t.bcasts, bcast)
	t.mtx.Unlock()
}

func (t *KeyedBroadcaster) Broadcast(ctx context.Context) ([]BroadcastResult, error) {
	// Empty the internal broadcast buffer as soon as this method is called.
	t.mtx.Lock()
	bcasts := t.bcasts
	t.bcasts = nil
	t.mtx.Unlock()

	if len(bcasts) == 0 {
		return nil, nil
	}

	results := make([]BroadcastResult, len(bcasts))

	latestBlock, err := t.client.BlockByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %w", err)
	}

	// RSKj compatibility: Send transactions sequentially to avoid mempool conflicts.
	// RSKj has slower block times (~30s) and stricter mempool handling than Ethereum.
	// Parallel tx submission causes "pending transaction with same hash" errors.
	var txErr *multierror.Error
	for i, bcast := range bcasts {
		id := bcast.ID()
		t.lgr.Info(
			"transaction broadcasting",
			"id", id,
			"nonce", bcast.Nonce,
			"progress", fmt.Sprintf("%d/%d", i+1, len(bcasts)),
		)

		// Send and wait for this transaction before sending the next
		fut, _ := t.broadcast(ctx, bcast, latestBlock.GasLimit())
		bcastRes := <-fut

		outRes := BroadcastResult{
			Broadcast: bcasts[i],
		}

		if bcastRes.Err == nil {
			outRes.Receipt = bcastRes.Receipt
			outRes.TxHash = bcastRes.Receipt.TxHash

			if bcastRes.Receipt.Status == 0 {
				failErr := fmt.Errorf("transaction failed: %s", outRes.Receipt.TxHash.String())
				txErr = multierror.Append(txErr, failErr)
				outRes.Err = failErr
				t.lgr.Error(
					"transaction failed on chain",
					"id", id,
					"completed", i+1,
					"total", len(bcasts),
					"hash", outRes.Receipt.TxHash.String(),
					"nonce", outRes.Broadcast.Nonce,
				)
			} else {
				t.lgr.Info(
					"transaction confirmed",
					"id", id,
					"completed", i+1,
					"total", len(bcasts),
					"hash", outRes.Receipt.TxHash.String(),
					"nonce", outRes.Broadcast.Nonce,
					"creation", outRes.Receipt.ContractAddress,
				)
			}
		} else {
			txErr = multierror.Append(txErr, bcastRes.Err)
			outRes.Err = bcastRes.Err
			t.lgr.Error(
				"transaction failed",
				"id", id,
				"completed", i+1,
				"total", len(bcasts),
				"err", bcastRes.Err,
			)
		}

		results[i] = outRes
	}
	return results, txErr.ErrorOrNil()
}

func (t *KeyedBroadcaster) broadcast(ctx context.Context, bcast script.Broadcast, blockGasLimit uint64) (<-chan txmgr.SendResponse, common.Hash) {
	ch := make(chan txmgr.SendResponse, 1)

	id := bcast.ID()
	candidate := asTxCandidate(bcast, blockGasLimit)
	t.mgr.SendAsync(ctx, candidate, ch)
	return ch, id
}

func asTxCandidate(bcast script.Broadcast, blockGasLimit uint64) txmgr.TxCandidate {
	value := ((*uint256.Int)(bcast.Value)).ToBig()
	var candidate txmgr.TxCandidate
	switch bcast.Type {
	case script.BroadcastCall:
		to := &bcast.To
		candidate = txmgr.TxCandidate{
			TxData:   bcast.Input,
			To:       to,
			Value:    value,
			GasLimit: padGasLimit(bcast.Input, bcast.GasUsed, false, blockGasLimit),
		}
	case script.BroadcastCreate:
		candidate = txmgr.TxCandidate{
			TxData:   bcast.Input,
			To:       nil,
			GasLimit: padGasLimit(bcast.Input, bcast.GasUsed, true, blockGasLimit),
		}
	case script.BroadcastCreate2:
		txData := make([]byte, len(bcast.Salt)+len(bcast.Input))
		copy(txData, bcast.Salt[:])
		copy(txData[len(bcast.Salt):], bcast.Input)

		candidate = txmgr.TxCandidate{
			TxData:   txData,
			To:       &script.DeterministicDeployerAddress,
			Value:    value,
			GasLimit: padGasLimit(bcast.Input, bcast.GasUsed, true, blockGasLimit),
		}
	default:
		panic(fmt.Sprintf("unrecognized broadcast type: '%s'", bcast.Type))
	}
	return candidate
}

// padGasLimit calculates the gas limit for a transaction based on the intrinsic gas and the gas used by
// the underlying call. Values are multiplied by a pad factor to account for any discrepancies. The output
// is clamped to the block gas limit since Geth will reject transactions that exceed it before letting them
// into the mempool.
func padGasLimit(data []byte, gasUsed uint64, creation bool, blockGasLimit uint64) uint64 {
	intrinsicGas, err := core.IntrinsicGas(data, nil, nil, creation, true, true, false)
	// This method never errors - we should look into it if it does.
	if err != nil {
		panic(err)
	}

	floorDataGas, err := core.FloorDataGas(data)
	// We should never cause an overflow here.
	if err != nil {
		panic(err)
	}

	gas := intrinsicGas + gasUsed
	if floorDataGas > gas {
		gas = floorDataGas
	}

	limit := uint64(float64(gas) * GasPadFactor)
	if limit > blockGasLimit {
		return blockGasLimit
	}
	return limit
}
