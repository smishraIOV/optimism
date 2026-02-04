package broadcaster

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	// baseFeePadFactor = 50% as a divisor
	baseFeePadFactor = big.NewInt(2)
	// tipMulFactor = 5 as a multiplier
	tipMulFactor = big.NewInt(5)
	// dummyBlobFee is a dummy value for the blob fee. Since this gas estimator will never
	// post blobs, it's just set to 1.
	dummyBlobFee = big.NewInt(1)
	// dummyBlobTipCap is a dummy value for the blob tip cap. Since this gas estimator will never
	// post blobs, it's just set to 0.
	dummyBlobTipCap = big.NewInt(0)
	// maxTip is the maximum tip that can be suggested by this estimator.
	maxTip = big.NewInt(50 * 1e9)
	// minTip is the minimum tip that can be suggested by this estimator.
	minTip = big.NewInt(1 * 1e9)
)

// DeployerGasPriceEstimator is a custom gas price estimator for use with op-deployer.
// It pads the base fee by 50% and multiplies the suggested tip by 5 up to a max of
// 50 gwei.
// For pre-EIP-1559 chains (like RSK), it falls back to legacy gas price estimation.
func DeployerGasPriceEstimator(ctx context.Context, client txmgr.ETHBackend) (*big.Int, *big.Int, *big.Int, *big.Int, error) {
	chainHead, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get block: %w", err)
	}

	// Handle pre-EIP-1559 chains (BaseFee is nil)
	if chainHead.BaseFee == nil {
		// Use legacy gas price for non-EIP-1559 chains
		var gasPrice *big.Int
		// Try to cast to ethclient.Client to access SuggestGasPrice
		if ethClient, ok := client.(*ethclient.Client); ok {
			gasPrice, err = ethClient.SuggestGasPrice(ctx)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to get gas price: %w", err)
			}
		} else {
			// Fallback to a reasonable default gas price (60 Mwei, works for RSK)
			gasPrice = big.NewInt(60000000)
		}
		// Pad the gas price by 50%
		gasPricePad := new(big.Int).Div(gasPrice, baseFeePadFactor)
		paddedGasPrice := new(big.Int).Add(gasPrice, gasPricePad)
		// For legacy transactions, tip and baseFee are both set to the gas price
		return paddedGasPrice, paddedGasPrice, dummyBlobTipCap, dummyBlobFee, nil
	}

	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get gas tip cap: %w", err)
	}

	baseFeePad := new(big.Int).Div(chainHead.BaseFee, baseFeePadFactor)
	paddedBaseFee := new(big.Int).Add(chainHead.BaseFee, baseFeePad)
	paddedTip := new(big.Int).Mul(tip, tipMulFactor)

	if paddedTip.Cmp(minTip) < 0 {
		paddedTip.Set(minTip)
	}

	if paddedTip.Cmp(maxTip) > 0 {
		paddedTip.Set(maxTip)
	}

	return paddedTip, paddedBaseFee, dummyBlobTipCap, dummyBlobFee, nil
}
