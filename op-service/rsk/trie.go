package rsk

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	// Import gorsk for RSK trie operations
	"gorsk/rskblocks"
)

// ComputeRSKTxRoot computes the transaction root hash for RSK blocks.
// RSK uses a binary trie instead of Ethereum's hexary MPT.
func ComputeRSKTxRoot(txs types.Transactions) (common.Hash, error) {
	// Convert geth transactions to gorsk transactions
	rskTxs := make([]*rskblocks.Transaction, len(txs))
	for i, tx := range txs {
		rskTx, err := gethTxToRskTx(tx)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to convert tx %d: %w", i, err)
		}
		rskTxs[i] = rskTx
	}

	// Compute the root using gorsk's binary trie
	rootBytes := rskblocks.GetTxTrieRoot(rskTxs)

	var root common.Hash
	copy(root[:], rootBytes)
	return root, nil
}

// ComputeRSKReceiptsRoot computes the receipts root hash for RSK blocks.
// RSK uses a binary trie instead of Ethereum's hexary MPT.
func ComputeRSKReceiptsRoot(receipts types.Receipts) (common.Hash, error) {
	// Convert geth receipts to gorsk receipts
	rskReceipts := make([]*rskblocks.TransactionReceipt, len(receipts))
	for i, receipt := range receipts {
		rskReceipt, err := gethReceiptToRskReceipt(receipt)
		if err != nil {
			return common.Hash{}, fmt.Errorf("failed to convert receipt %d: %w", i, err)
		}
		rskReceipts[i] = rskReceipt
	}

	// Compute the root using gorsk's binary trie
	rootBytes := rskblocks.CalculateReceiptsTrieRoot(rskReceipts)

	var root common.Hash
	copy(root[:], rootBytes)
	return root, nil
}

// VerifyRSKTxRoot verifies that the transaction root in a block header matches
// the computed root from the transactions list.
func VerifyRSKTxRoot(expectedRoot common.Hash, txs types.Transactions) error {
	computed, err := ComputeRSKTxRoot(txs)
	if err != nil {
		return fmt.Errorf("failed to compute RSK tx root: %w", err)
	}

	if !bytes.Equal(computed[:], expectedRoot[:]) {
		return fmt.Errorf("RSK tx root mismatch: computed %s but expected %s", computed.Hex(), expectedRoot.Hex())
	}

	return nil
}

// VerifyRSKReceiptsRoot verifies that the receipts root in a block header matches
// the computed root from the receipts list.
func VerifyRSKReceiptsRoot(expectedRoot common.Hash, receipts types.Receipts) error {
	computed, err := ComputeRSKReceiptsRoot(receipts)
	if err != nil {
		return fmt.Errorf("failed to compute RSK receipts root: %w", err)
	}

	if !bytes.Equal(computed[:], expectedRoot[:]) {
		return fmt.Errorf("RSK receipts root mismatch: computed %s but expected %s", computed.Hex(), expectedRoot.Hex())
	}

	return nil
}

// gethTxToRskTx converts a go-ethereum Transaction to a gorsk Transaction.
// This handles the encoding differences between Ethereum and RSK transactions.
func gethTxToRskTx(tx *types.Transaction) (*rskblocks.Transaction, error) {
	// Use gorsk's NewSignedTransaction to create the RSK transaction
	v, r, s := tx.RawSignatureValues()

	rskTx := rskblocks.NewSignedTransaction(
		tx.Nonce(),
		tx.To(),
		tx.Value(),
		tx.Gas(),
		tx.GasPrice(),
		tx.Data(),
		v, r, s,
	)

	return rskTx, nil
}

// gethReceiptToRskReceipt converts a go-ethereum Receipt to a gorsk TransactionReceipt.
func gethReceiptToRskReceipt(receipt *types.Receipt) (*rskblocks.TransactionReceipt, error) {
	// Convert logs
	rskLogs := make([]*rskblocks.Log, len(receipt.Logs))
	for i, log := range receipt.Logs {
		rskLogs[i] = &rskblocks.Log{
			Address: log.Address,
			Topics:  log.Topics,
			Data:    log.Data,
		}
	}

	rskReceipt := &rskblocks.TransactionReceipt{
		CumulativeGasUsed: receipt.CumulativeGasUsed,
		Bloom:             receipt.Bloom,
		Logs:              rskLogs,
		GasUsed:           receipt.GasUsed,
	}

	// Handle PostState vs Status (EIP-658)
	// In RSK, for new-style receipts (EIP-658), postTxState contains the status byte
	if len(receipt.PostState) > 0 {
		// Pre-Byzantium style: PostState is the state root
		rskReceipt.PostState = receipt.PostState
	} else {
		// EIP-658 style: PostState contains the status byte
		if receipt.Status == types.ReceiptStatusSuccessful {
			rskReceipt.PostState = []byte{1}
		} else {
			rskReceipt.PostState = []byte{}
		}
	}

	// Also set Status field
	if receipt.Status == types.ReceiptStatusSuccessful {
		rskReceipt.Status = []byte{1}
	} else {
		rskReceipt.Status = []byte{}
	}

	return rskReceipt, nil
}
