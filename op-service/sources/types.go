package sources

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/holiman/uint256"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/ethereum-optimism/optimism/op-core/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/rsk"
)

// Note: these types are used, instead of the geth types, to enable:
// - batched calls of many block requests (standard bindings do extra uncle-header fetches, cannot be batched nicely)
// - ignore uncle data (does not even exist anymore post-Merge)
// - use cached block hash, if we trust the RPC.
// - verify transactions list matches tx-root, to ensure consistency with block-hash, if we do not trust the RPC
// - verify block contents are compatible with Post-Merge ExecutionPayload format
//
// Transaction-sender data from the RPC is not cached, since ethclient.setSenderFromServer is private,
// and we only need to compute the sender for transactions into the inbox.
//
// This way we minimize RPC calls, enable batching, and can choose to verify what the RPC gives us.

type RPCHeader struct {
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

	// BaseFee was added by EIP-1559 and is ignored in legacy headers.
	BaseFee *hexutil.Big `json:"baseFeePerGas"`

	// WithdrawalsRoot was added by EIP-4895 and is ignored in legacy headers.
	WithdrawalsRoot *common.Hash `json:"withdrawalsRoot,omitempty"`

	// BlobGasUsed was added by EIP-4844 and is ignored in legacy headers.
	BlobGasUsed *hexutil.Uint64 `json:"blobGasUsed,omitempty"`

	// ExcessBlobGas was added by EIP-4844 and is ignored in legacy headers.
	ExcessBlobGas *hexutil.Uint64 `json:"excessBlobGas,omitempty"`

	// ParentBeaconRoot was added by EIP-4788 and is ignored in legacy headers.
	ParentBeaconRoot *common.Hash `json:"parentBeaconBlockRoot,omitempty"`

	// RequestsHash was added by EIP-7685 and is ignored in legacy headers.
	RequestsHash *common.Hash `json:"requestsHash,omitempty" rlp:"optional"`

	// untrusted info included by RPC, may have to be checked
	Hash common.Hash `json:"hash"`

	// RSK-specific fields (present when connected to RSK L1)
	// These fields are used to detect RSK chains and compute RSK-specific block hashes.
	// See: https://github.com/rsksmart/RSKIPs

	// PaidFees is the total fees paid in this block (RSK-specific)
	PaidFees *hexutil.Big `json:"paidFees,omitempty"`

	// MinimumGasPrice is the minimum gas price for transactions (RSK-specific)
	MinimumGasPrice *hexutil.Big `json:"minimumGasPrice,omitempty"`

	// UncleCount is the number of uncles (RSK uses this differently than Ethereum)
	UncleCount *hexutil.Uint64 `json:"uncleCount,omitempty"`

	// UmmRoot is the Unified Mining Merkle root (RSKIP-UMM)
	UmmRoot *common.Hash `json:"ummRoot,omitempty"`

	// TxExecutionSublistsEdges for parallel transaction execution (RSKIP-144)
	// Note: RSKj returns this as "rskPteEdges" in JSON-RPC responses
	TxExecutionSublistsEdges []hexutil.Uint64 `json:"txExecutionSublistsEdges,omitempty"`

	// RskPteEdges is the RSKj field name for TxExecutionSublistsEdges
	RskPteEdges []hexutil.Uint64 `json:"rskPteEdges,omitempty"`

	// BaseEvent is for V2 headers (RSKIP-535) - pointer to handle null from RPC
	BaseEvent *hexutil.Bytes `json:"baseEvent,omitempty"`

	// Bitcoin merged mining fields
	BitcoinMergedMiningHeader              hexutil.Bytes `json:"bitcoinMergedMiningHeader,omitempty"`
	BitcoinMergedMiningMerkleProof         hexutil.Bytes `json:"bitcoinMergedMiningMerkleProof,omitempty"`
	BitcoinMergedMiningCoinbaseTransaction hexutil.Bytes `json:"bitcoinMergedMiningCoinbaseTransaction,omitempty"`
}

// isRSK returns true if this header appears to be from an RSK chain.
// RSK headers have specific fields like PaidFees and MinimumGasPrice that Ethereum doesn't have.
func (hdr *RPCHeader) isRSK() bool {
	// RSK always includes minimumGasPrice in block headers
	return hdr.MinimumGasPrice != nil
}

// checkPostMerge checks that the block header meets all criteria to be a valid ExecutionPayloadHeader,
// see EIP-3675 (block header changes) and EIP-4399 (mixHash usage for prev-randao)
func (hdr *RPCHeader) checkPostMerge() error {
	// RSK is a pre-merge style chain with PoW characteristics - skip post-merge checks
	if hdr.isRSK() {
		return nil
	}

	// TODO: the genesis block has a non-zero difficulty number value.
	// Either this block needs to change, or we special case it. This is not valid w.r.t. EIP-3675.
	if hdr.Number != 0 && (*big.Int)(&hdr.Difficulty).Cmp(common.Big0) != 0 {
		return fmt.Errorf("post-merge block header requires zeroed difficulty field, but got: %s", &hdr.Difficulty)
	}
	if hdr.Nonce != (types.BlockNonce{}) {
		return fmt.Errorf("post-merge block header requires zeroed block nonce field, but got: %s", hdr.Nonce)
	}
	if hdr.BaseFee == nil {
		return fmt.Errorf("post-merge block header requires EIP-1559 base fee field, but got %s", hdr.BaseFee)
	}
	if len(hdr.Extra) > 32 {
		return fmt.Errorf("post-merge block header requires 32 or less bytes of extra data, but got %d", len(hdr.Extra))
	}
	if hdr.UncleHash != types.EmptyUncleHash {
		return fmt.Errorf("post-merge block header requires uncle hash to be of empty uncle list, but got %s", hdr.UncleHash)
	}
	return nil
}

func (hdr *RPCHeader) computeBlockHash() common.Hash {
	// RSK uses different block hash computation rules (RSKIP-92, RSKIP-351)
	if hdr.isRSK() {
		rskHeader := hdr.toRSKRPCHeader()
		// Determine RSK network config based on block characteristics.
		// For blocks at height > 6M, all RSKIPs are active on all RSK networks.
		// For lower blocks, we use the regtest config which has all RSKIPs active from genesis.
		// This works correctly for:
		// - Regtest: all RSKIPs active from genesis
		// - Testnet: all RSKIPs typically active from genesis
		// - Mainnet: for current blocks (> 6M), all RSKIPs are active
		// Note: For historical mainnet blocks before RSKIP activations, the hash computation
		// may be incorrect, but op-node typically only processes recent blocks.
		config := hdr.getRSKNetworkConfig()
		hash, err := rsk.ComputeRSKBlockHash(rskHeader, config)
		if err != nil {
			// Fall back to standard computation if RSK hash fails
			// This shouldn't happen but provides graceful degradation
			gethHeader := hdr.CreateGethHeader()
			return gethHeader.Hash()
		}
		return hash
	}

	gethHeader := hdr.CreateGethHeader()
	return gethHeader.Hash()
}

// getRSKNetworkConfig determines the RSK network configuration based on block characteristics.
// TODO: Ideally we should pass chain ID from EthClient to get accurate network config.
// For now, we use regtest config which has all RSKIPs active (V2 headers).
// This works correctly for regtest. For mainnet/testnet, the chain ID should be passed
// through the verification pipeline for accurate RSKIP activation detection.
func (hdr *RPCHeader) getRSKNetworkConfig() rsk.RSKNetworkConfig {
	// Default to regtest config - all RSKIPs active, V2 headers, 4-byte gasLimit
	// This is the safest default for local development and testing.
	//
	// For production use with mainnet/testnet, the network config should be
	// determined from chain ID rather than inferred from header fields.
	return rsk.DefaultRegtestConfig()
}

// toRSKRPCHeader converts this RPCHeader to an RSKRPCHeader for RSK-specific operations.
func (hdr *RPCHeader) toRSKRPCHeader() *rsk.RSKRPCHeader {
	rskHdr := &rsk.RSKRPCHeader{
		ParentHash:  hdr.ParentHash,
		UncleHash:   hdr.UncleHash,
		Coinbase:    hdr.Coinbase,
		Root:        hdr.Root,
		TxHash:      hdr.TxHash,
		ReceiptHash: hdr.ReceiptHash,
		Bloom:       hdr.Bloom,
		Difficulty:  hdr.Difficulty,
		Number:      hdr.Number,
		GasLimit:    hdr.GasLimit,
		GasUsed:     hdr.GasUsed,
		Time:        hdr.Time,
		Extra:       hdr.Extra,
		MixDigest:   hdr.MixDigest,
		Nonce:       hdr.Nonce,
		Hash:        hdr.Hash,

		// RSK-specific fields
		PaidFees:                               hdr.PaidFees,
		MinimumGasPrice:                        hdr.MinimumGasPrice,
		UmmRoot:                                hdr.UmmRoot,
		BaseEvent:                              hdr.BaseEvent,
		BitcoinMergedMiningHeader:              hdr.BitcoinMergedMiningHeader,
		BitcoinMergedMiningMerkleProof:         hdr.BitcoinMergedMiningMerkleProof,
		BitcoinMergedMiningCoinbaseTransaction: hdr.BitcoinMergedMiningCoinbaseTransaction,
	}

	// Handle TxExecutionSublistsEdges - RSKj uses "rskPteEdges" field name
	if len(hdr.TxExecutionSublistsEdges) > 0 {
		rskHdr.TxExecutionSublistsEdges = hdr.TxExecutionSublistsEdges
	} else if len(hdr.RskPteEdges) > 0 {
		rskHdr.TxExecutionSublistsEdges = hdr.RskPteEdges
	} else if hdr.RskPteEdges != nil {
		// RSKj returns empty array [] which we need to preserve (different from nil)
		rskHdr.TxExecutionSublistsEdges = []hexutil.Uint64{}
	}

	// Handle UncleCount conversion - default to 0 if not provided
	if hdr.UncleCount != nil {
		rskHdr.UncleCount = *hdr.UncleCount
	}
	// Note: RSKj doesn't return uncleCount directly, it can be computed from uncles array

	return rskHdr
}

func (hdr *RPCHeader) CreateGethHeader() *types.Header {
	return &types.Header{
		ParentHash:      hdr.ParentHash,
		UncleHash:       hdr.UncleHash,
		Coinbase:        hdr.Coinbase,
		Root:            hdr.Root,
		TxHash:          hdr.TxHash,
		ReceiptHash:     hdr.ReceiptHash,
		Bloom:           types.Bloom(hdr.Bloom),
		Difficulty:      (*big.Int)(&hdr.Difficulty),
		Number:          new(big.Int).SetUint64(uint64(hdr.Number)),
		GasLimit:        uint64(hdr.GasLimit),
		GasUsed:         uint64(hdr.GasUsed),
		Time:            uint64(hdr.Time),
		Extra:           hdr.Extra,
		MixDigest:       hdr.MixDigest,
		Nonce:           hdr.Nonce,
		BaseFee:         (*big.Int)(hdr.BaseFee),
		WithdrawalsHash: hdr.WithdrawalsRoot,
		// Cancun
		BlobGasUsed:      (*uint64)(hdr.BlobGasUsed),
		ExcessBlobGas:    (*uint64)(hdr.ExcessBlobGas),
		ParentBeaconRoot: hdr.ParentBeaconRoot,
		// Prague
		RequestsHash: hdr.RequestsHash,
	}
}

func (hdr *RPCHeader) Info(trustCache bool, mustBePostMerge bool) (eth.BlockInfo, error) {
	if mustBePostMerge {
		if err := hdr.checkPostMerge(); err != nil {
			return nil, err
		}
	}
	if !trustCache {
		if computed := hdr.computeBlockHash(); computed != hdr.Hash {
			return nil, fmt.Errorf("failed to verify block hash: computed %s but RPC said %s", computed, hdr.Hash)
		}
	}
	return eth.HeaderBlockInfoTrusted(hdr.Hash, hdr.CreateGethHeader()), nil
}

func (hdr *RPCHeader) BlockID() eth.BlockID {
	return eth.BlockID{
		Hash:   hdr.Hash,
		Number: uint64(hdr.Number),
	}
}

type RPCBlock struct {
	RPCHeader
	Transactions []*types.Transaction `json:"transactions"`
	Withdrawals  *types.Withdrawals   `json:"withdrawals,omitempty"`

	// OriginalTxHashes stores the transaction hashes as returned by the RPC.
	// For RSK, go-ethereum computes wrong tx hashes due to different RLP encoding.
	// This field preserves the original hashes for use in receipt fetching.
	OriginalTxHashes []common.Hash `json:"-"`
}

// rpcBlockForUnmarshal is used for custom JSON unmarshaling to capture original tx hashes.
type rpcBlockForUnmarshal struct {
	RPCHeader
	Transactions []rpcTxForUnmarshal `json:"transactions"`
	Withdrawals  *types.Withdrawals  `json:"withdrawals,omitempty"`
}

// rpcTxForUnmarshal captures the original hash from RPC before go-ethereum recomputes it.
type rpcTxForUnmarshal struct {
	Hash common.Hash `json:"hash"`
	*types.Transaction
}

func (tx *rpcTxForUnmarshal) UnmarshalJSON(data []byte) error {
	// First extract just the hash
	var hashOnly struct {
		Hash common.Hash `json:"hash"`
	}
	if err := json.Unmarshal(data, &hashOnly); err != nil {
		return err
	}
	tx.Hash = hashOnly.Hash

	// Then unmarshal the full transaction
	tx.Transaction = new(types.Transaction)
	return tx.Transaction.UnmarshalJSON(data)
}

func (block *RPCBlock) UnmarshalJSON(data []byte) error {
	var raw rpcBlockForUnmarshal
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	block.RPCHeader = raw.RPCHeader
	block.Withdrawals = raw.Withdrawals

	// Convert transactions and preserve original hashes
	block.Transactions = make([]*types.Transaction, len(raw.Transactions))
	block.OriginalTxHashes = make([]common.Hash, len(raw.Transactions))
	for i, tx := range raw.Transactions {
		block.Transactions[i] = tx.Transaction
		block.OriginalTxHashes[i] = tx.Hash
	}

	return nil
}

// GetTxHashes returns the correct transaction hashes for this block.
// For RSK blocks, returns the original hashes from RPC (since go-ethereum computes wrong hashes).
// For Ethereum blocks, returns the computed hashes from the Transaction objects.
func (block *RPCBlock) GetTxHashes() []common.Hash {
	if block.isRSK() && len(block.OriginalTxHashes) == len(block.Transactions) {
		return block.OriginalTxHashes
	}
	// Fallback to computed hashes
	hashes := make([]common.Hash, len(block.Transactions))
	for i, tx := range block.Transactions {
		hashes[i] = tx.Hash()
	}
	return hashes
}

func (block *RPCBlock) Verify() error {
	if computed := block.computeBlockHash(); computed != block.Hash {
		return fmt.Errorf("failed to verify block hash: computed %s but RPC said %s", computed, block.Hash)
	}
	for i, tx := range block.Transactions {
		if tx == nil {
			return fmt.Errorf("block tx %d is nil", i)
		}
	}

	// RSK uses a binary trie instead of Ethereum's hexary MPT for transaction roots
	if block.isRSK() {
		if err := rsk.VerifyRSKTxRoot(block.TxHash, block.Transactions); err != nil {
			return fmt.Errorf("failed to verify RSK transactions list: %w", err)
		}
	} else {
		if computed := types.DeriveSha(types.Transactions(block.Transactions), trie.NewStackTrie(nil)); block.TxHash != computed {
			return fmt.Errorf("failed to verify transactions list: computed %s but RPC said %s", computed, block.TxHash)
		}
	}

	// RSK doesn't have withdrawals (pre-Shanghai chain) - skip withdrawal validation
	if block.isRSK() {
		return nil
	}

	// Withdrawals validation is different between L1 and L2.
	// It is possible to determine that it is an L2 block if the first transaction is a deposit.
	// The genesis block does not have transactions, but does have a known fee-recipient predeploy address.
	isL2 := (len(block.Transactions) > 0 && block.Transactions[0].IsDepositTx()) ||
		(block.Number == 0 && block.Coinbase == predeploys.SequencerFeeVaultAddr)
	if isL2 {
		if err := block.validateL2Withdrawals(block.Withdrawals, block.WithdrawalsRoot); err != nil {
			return err
		}
	} else {
		if err := block.validateL1Withdrawals(block.Withdrawals, block.WithdrawalsRoot); err != nil {
			return err
		}
	}
	return nil
}

func (block *RPCBlock) validateL1Withdrawals(withdrawals *types.Withdrawals, withdrawalsRoot *common.Hash) error {
	if withdrawalsRoot != nil {
		if withdrawals == nil {
			return errors.New("expected withdrawals")
		}
		for i, w := range *withdrawals {
			if w == nil {
				return fmt.Errorf("block withdrawal %d is null", i)
			}
		}
		if computed := types.DeriveSha(*withdrawals, trie.NewStackTrie(nil)); *withdrawalsRoot != computed {
			return fmt.Errorf("failed to verify withdrawals list: computed %s but RPC said %s", computed, withdrawalsRoot)
		}
	} else {
		if withdrawals != nil {
			return fmt.Errorf("expected no withdrawals due to missing withdrawals-root, but got %d", len(*withdrawals))
		}
	}
	return nil
}

func (block *RPCBlock) validateL2Withdrawals(withdrawals *types.Withdrawals, withdrawalsRoot *common.Hash) error {
	if withdrawalsRoot != nil {
		if !(withdrawals != nil && len(*withdrawals) == 0) {
			return fmt.Errorf("expected empty withdrawals, but got %d", len(*withdrawals))
		}
	}
	return nil
}

func (block *RPCBlock) Info(trustCache bool, mustBePostMerge bool) (eth.BlockInfo, types.Transactions, error) {
	if mustBePostMerge {
		if err := block.checkPostMerge(); err != nil {
			return nil, nil, err
		}
	}
	if !trustCache {
		if err := block.Verify(); err != nil {
			return nil, nil, err
		}
	}

	// verify the header data
	info, err := block.RPCHeader.Info(trustCache, mustBePostMerge)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to verify block from RPC: %w", err)
	}

	return info, block.Transactions, nil
}

func (block *RPCBlock) ExecutionPayloadEnvelope(trustCache bool) (*eth.ExecutionPayloadEnvelope, error) {
	if err := block.checkPostMerge(); err != nil {
		return nil, err
	}
	if !trustCache {
		if err := block.Verify(); err != nil {
			return nil, err
		}
	}
	var baseFee uint256.Int
	baseFee.SetFromBig((*big.Int)(block.BaseFee))

	// Unfortunately eth_getBlockByNumber either returns full transactions, or only tx-hashes.
	// There is no option for encoded transactions.
	opaqueTxs := make([]hexutil.Bytes, len(block.Transactions))
	for i, tx := range block.Transactions {
		data, err := tx.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("failed to encode tx %d from RPC: %w", i, err)
		}
		opaqueTxs[i] = data
	}

	payload := &eth.ExecutionPayload{
		ParentHash:    block.ParentHash,
		FeeRecipient:  block.Coinbase,
		StateRoot:     eth.Bytes32(block.Root),
		ReceiptsRoot:  eth.Bytes32(block.ReceiptHash),
		LogsBloom:     block.Bloom,
		PrevRandao:    eth.Bytes32(block.MixDigest), // mix-digest field is used for prevRandao post-merge
		BlockNumber:   block.Number,
		GasLimit:      block.GasLimit,
		GasUsed:       block.GasUsed,
		Timestamp:     block.Time,
		ExtraData:     eth.BytesMax32(block.Extra),
		BaseFeePerGas: eth.Uint256Quantity(baseFee),
		BlockHash:     block.Hash,
		Transactions:  opaqueTxs,
		Withdrawals:   block.Withdrawals,
		BlobGasUsed:   block.BlobGasUsed,
		ExcessBlobGas: block.ExcessBlobGas,
	}

	// Only Isthmus execution payloads must set the withdrawals root.
	// They are guaranteed to not be the empty withdrawals hash, which is set pre-Isthmus (post-Canyon).
	if wr := block.WithdrawalsRoot; wr != nil && *wr != types.EmptyWithdrawalsHash {
		wr := *wr
		payload.WithdrawalsRoot = &wr
	}

	return &eth.ExecutionPayloadEnvelope{
		ParentBeaconBlockRoot: block.ParentBeaconRoot,
		ExecutionPayload:      payload,
	}, nil
}

// blockHashParameter is used as "block parameter":
// Some Nethermind and Alchemy RPC endpoints require an object to identify a block, instead of a string.
type blockHashParameter struct {
	BlockHash common.Hash `json:"blockHash"`
}

// unusableMethod identifies if an error indicates that the RPC method cannot be used as expected:
// if it's an unknown method, or if parameters were invalid.
func unusableMethod(err error) bool {
	if rpcErr, ok := err.(rpc.Error); ok {
		code := rpcErr.ErrorCode()
		// invalid request, method not found, or invalid params
		if code == -32600 || code == -32601 || code == -32602 {
			return true
		}
	}
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "unsupported method") || // alchemy -32600 message
		strings.Contains(errText, "unknown method") ||
		strings.Contains(errText, "invalid param") ||
		strings.Contains(errText, "is not available") ||
		strings.Contains(errText, "rpc method is not whitelisted") // proxyd -32001 error code
}
