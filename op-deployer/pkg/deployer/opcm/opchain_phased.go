package opcm

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
	"github.com/ethereum/go-ethereum/common"
)

// DeployOPChainPhasedInput contains the shared input for all phases
type DeployOPChainPhasedInput struct {
	OpChainProxyAdminOwner common.Address
	SystemConfigOwner      common.Address
	Batcher                common.Address
	UnsafeBlockSigner      common.Address
	Proposer               common.Address
	Challenger             common.Address

	BasefeeScalar     uint32
	BlobBaseFeeScalar uint32
	L2ChainId         *big.Int
	Opcm              common.Address
	SaltMixer         string
	GasLimit          uint64

	DisputeGameType              uint32
	DisputeAbsolutePrestate      common.Hash
	DisputeMaxGameDepth          *big.Int
	DisputeSplitDepth            *big.Int
	DisputeClockExtension        uint64
	DisputeMaxClockDuration      uint64
	AllowCustomDisputeParameters bool

	OperatorFeeScalar   uint32
	OperatorFeeConstant uint64
	SuperchainConfig    common.Address

	UseCustomGasToken bool
}

// Phase1Input is the input for phase 1
type Phase1Input struct {
	DeployInput  DeployOPChainPhasedInput
	OpcmDeployer common.Address
}

// Phase1Output is the output from phase 1
type Phase1Output struct {
	AddressManager     common.Address
	OpChainProxyAdmin  common.Address
}

// Phase2Input is the input for phase 2
type Phase2Input struct {
	DeployInput  DeployOPChainPhasedInput
	OpcmDeployer common.Address
	Phase1       Phase1Output
}

// Phase2Output is the output from phase 2
type Phase2Output struct {
	AddressManager                    common.Address
	OpChainProxyAdmin                 common.Address
	L1ERC721BridgeProxy               common.Address
	OptimismPortalProxy               common.Address
	EthLockboxProxy                   common.Address `evm:"ethLockboxProxy"`
	SystemConfigProxy                 common.Address
	OptimismMintableERC20FactoryProxy common.Address
	DisputeGameFactoryProxy           common.Address
	AnchorStateRegistryProxy          common.Address
}

// Phase3Input is the input for phase 3
type Phase3Input struct {
	DeployInput  DeployOPChainPhasedInput
	OpcmDeployer common.Address
	Phase2       Phase2Output
}

// Phase3Output is the output from phase 3
type Phase3Output struct {
	AddressManager                    common.Address
	OpChainProxyAdmin                 common.Address
	L1ERC721BridgeProxy               common.Address
	OptimismPortalProxy               common.Address
	EthLockboxProxy                   common.Address `evm:"ethLockboxProxy"`
	SystemConfigProxy                 common.Address
	OptimismMintableERC20FactoryProxy common.Address
	DisputeGameFactoryProxy           common.Address
	AnchorStateRegistryProxy          common.Address
	L1StandardBridgeProxy             common.Address
	L1CrossDomainMessengerProxy       common.Address
	DelayedWETHPermissionedGameProxy  common.Address
}

// Phase4Input is the input for phase 4
type Phase4Input struct {
	DeployInput      DeployOPChainPhasedInput
	OpcmDeployer     common.Address
	Phase3           Phase3Output
	SuperchainConfig common.Address
}

// Phase4Output is the final output (same as DeployOPChainOutput)
type Phase4Output = DeployOPChainOutput

// Script types for each phase
type DeployPhase1Script script.DeployScriptWithOutput[Phase1Input, Phase1Output]
type DeployPhase2Script script.DeployScriptWithOutput[Phase2Input, Phase2Output]
type DeployPhase3Script script.DeployScriptWithOutput[Phase3Input, Phase3Output]
type DeployPhase4Script script.DeployScriptWithOutput[Phase4Input, Phase4Output]

// NewDeployPhase1Script loads the phase 1 script
func NewDeployPhase1Script(host *script.Host) (DeployPhase1Script, error) {
	forgeScript, err := script.NewForgeScriptFromFile(host, "DeployOPChainPhased.s.sol", "DeployOPChainPhased")
	if err != nil {
		return nil, err
	}
	return script.NewDeployScriptWithOutput[Phase1Input, Phase1Output](forgeScript, "runPhase1")
}

// NewDeployPhase2Script loads the phase 2 script
func NewDeployPhase2Script(host *script.Host) (DeployPhase2Script, error) {
	forgeScript, err := script.NewForgeScriptFromFile(host, "DeployOPChainPhased.s.sol", "DeployOPChainPhased")
	if err != nil {
		return nil, err
	}
	return script.NewDeployScriptWithOutput[Phase2Input, Phase2Output](forgeScript, "runPhase2")
}

// NewDeployPhase3Script loads the phase 3 script
func NewDeployPhase3Script(host *script.Host) (DeployPhase3Script, error) {
	forgeScript, err := script.NewForgeScriptFromFile(host, "DeployOPChainPhased.s.sol", "DeployOPChainPhased")
	if err != nil {
		return nil, err
	}
	return script.NewDeployScriptWithOutput[Phase3Input, Phase3Output](forgeScript, "runPhase3")
}

// NewDeployPhase4Script loads the phase 4 script
func NewDeployPhase4Script(host *script.Host) (DeployPhase4Script, error) {
	forgeScript, err := script.NewForgeScriptFromFile(host, "DeployOPChainPhased.s.sol", "DeployOPChainPhased")
	if err != nil {
		return nil, err
	}
	return script.NewDeployScriptWithOutput[Phase4Input, Phase4Output](forgeScript, "runPhase4")
}

// PhasedDeploymentState holds the intermediate state during phased deployment
type PhasedDeploymentState struct {
	Phase1Complete bool         `json:"phase1Complete"`
	Phase2Complete bool         `json:"phase2Complete"`
	Phase3Complete bool         `json:"phase3Complete"`
	Phase4Complete bool         `json:"phase4Complete"`
	Phase1Output   *Phase1Output `json:"phase1Output,omitempty"`
	Phase2Output   *Phase2Output `json:"phase2Output,omitempty"`
	Phase3Output   *Phase3Output `json:"phase3Output,omitempty"`
}

// ToDeployInput converts a phased input to the standard input format
func (p *DeployOPChainPhasedInput) ToDeployInput() DeployOPChainInput {
	return DeployOPChainInput{
		OpChainProxyAdminOwner:       p.OpChainProxyAdminOwner,
		SystemConfigOwner:            p.SystemConfigOwner,
		Batcher:                      p.Batcher,
		UnsafeBlockSigner:            p.UnsafeBlockSigner,
		Proposer:                     p.Proposer,
		Challenger:                   p.Challenger,
		BasefeeScalar:                p.BasefeeScalar,
		BlobBaseFeeScalar:            p.BlobBaseFeeScalar,
		L2ChainId:                    p.L2ChainId,
		Opcm:                         p.Opcm,
		SaltMixer:                    p.SaltMixer,
		GasLimit:                     p.GasLimit,
		DisputeGameType:              p.DisputeGameType,
		DisputeAbsolutePrestate:      p.DisputeAbsolutePrestate,
		DisputeMaxGameDepth:          p.DisputeMaxGameDepth,
		DisputeSplitDepth:            p.DisputeSplitDepth,
		DisputeClockExtension:        p.DisputeClockExtension,
		DisputeMaxClockDuration:      p.DisputeMaxClockDuration,
		AllowCustomDisputeParameters: p.AllowCustomDisputeParameters,
		OperatorFeeScalar:            p.OperatorFeeScalar,
		OperatorFeeConstant:          p.OperatorFeeConstant,
		SuperchainConfig:             p.SuperchainConfig,
		UseCustomGasToken:            p.UseCustomGasToken,
	}
}

// FromDeployInput creates a phased input from the standard input
func FromDeployInput(input DeployOPChainInput) DeployOPChainPhasedInput {
	return DeployOPChainPhasedInput{
		OpChainProxyAdminOwner:       input.OpChainProxyAdminOwner,
		SystemConfigOwner:            input.SystemConfigOwner,
		Batcher:                      input.Batcher,
		UnsafeBlockSigner:            input.UnsafeBlockSigner,
		Proposer:                     input.Proposer,
		Challenger:                   input.Challenger,
		BasefeeScalar:                input.BasefeeScalar,
		BlobBaseFeeScalar:            input.BlobBaseFeeScalar,
		L2ChainId:                    input.L2ChainId,
		Opcm:                         input.Opcm,
		SaltMixer:                    input.SaltMixer,
		GasLimit:                     input.GasLimit,
		DisputeGameType:              input.DisputeGameType,
		DisputeAbsolutePrestate:      input.DisputeAbsolutePrestate,
		DisputeMaxGameDepth:          input.DisputeMaxGameDepth,
		DisputeSplitDepth:            input.DisputeSplitDepth,
		DisputeClockExtension:        input.DisputeClockExtension,
		DisputeMaxClockDuration:      input.DisputeMaxClockDuration,
		AllowCustomDisputeParameters: input.AllowCustomDisputeParameters,
		OperatorFeeScalar:            input.OperatorFeeScalar,
		OperatorFeeConstant:          input.OperatorFeeConstant,
		SuperchainConfig:             input.SuperchainConfig,
		UseCustomGasToken:            input.UseCustomGasToken,
	}
}

// Phase3ToFinalOutput converts phase 3 output to the final DeployOPChainOutput
func Phase3ToFinalOutput(p3 Phase3Output) DeployOPChainOutput {
	return DeployOPChainOutput{
		OpChainProxyAdmin:                 p3.OpChainProxyAdmin,
		AddressManager:                    p3.AddressManager,
		L1ERC721BridgeProxy:               p3.L1ERC721BridgeProxy,
		SystemConfigProxy:                 p3.SystemConfigProxy,
		OptimismMintableERC20FactoryProxy: p3.OptimismMintableERC20FactoryProxy,
		L1StandardBridgeProxy:             p3.L1StandardBridgeProxy,
		L1CrossDomainMessengerProxy:       p3.L1CrossDomainMessengerProxy,
		OptimismPortalProxy:               p3.OptimismPortalProxy,
		EthLockboxProxy:                   p3.EthLockboxProxy,
		DisputeGameFactoryProxy:           p3.DisputeGameFactoryProxy,
		AnchorStateRegistryProxy:          p3.AnchorStateRegistryProxy,
		DelayedWETHPermissionedGameProxy:  p3.DelayedWETHPermissionedGameProxy,
		// These are set during phase 4 initialization
		FaultDisputeGame:                   common.Address{},
		PermissionedDisputeGame:            common.Address{},
		DelayedWETHPermissionlessGameProxy: common.Address{},
	}
}

