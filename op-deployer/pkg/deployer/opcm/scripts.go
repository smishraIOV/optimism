package opcm

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
)

// Scripts contains all the deployment scripts for ease of passing them around
type Scripts struct {
	DeployAlphabetVM      DeployAlphabetVMScript
	DeployAltDA           DeployAltDAScript
	DeployAsterisc        DeployAsteriscScript
	DeployDisputeGame     DeployDisputeGameScript
	DeployImplementations DeployImplementationsScript
	DeployMIPS            DeployMIPSScript
	DeploySuperchain      DeploySuperchainScript
	DeployOPChain         DeployOPChainScript
	// Phased deployment scripts for gas-limited L1s
	DeployPhase1 DeployPhase1Script
	DeployPhase2 DeployPhase2Script
	DeployPhase3 DeployPhase3Script
	DeployPhase4 DeployPhase4Script
}

// NewScripts collects all the deployment scripts, raising exceptions if any of them
// are not found or if the Go types don't match the ABI
func NewScripts(host *script.Host) (*Scripts, error) {
	deployImplementations, err := NewDeployImplementationsScript(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeployImplementations script: %w", err)
	}

	deploySuperchain, err := NewDeploySuperchainScript(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeploySuperchain script: %w", err)
	}

	deployAlphabetVM, err := NewDeployAlphabetVMScript(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeployAlphabetVM script: %w", err)
	}

	deployAltDA, err := NewDeployAltDAScript(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeployAltDA script: %w", err)
	}

	deployAsterisc, err := NewDeployAsteriscScript(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeployAsterisc script: %w", err)
	}

	deployDisputeGame, err := NewDeployDisputeGameScript(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeployDisputeGame script: %w", err)
	}

	deployMIPSScript, err := NewDeployMIPSScript(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeployMIPSScript script: %w", err)
	}

	deployOPChain, err := NewDeployOPChainScript(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeployOPChain script: %w", err)
	}

	// Load phased deployment scripts
	deployPhase1, err := NewDeployPhase1Script(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeployPhase1 script: %w", err)
	}

	deployPhase2, err := NewDeployPhase2Script(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeployPhase2 script: %w", err)
	}

	deployPhase3, err := NewDeployPhase3Script(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeployPhase3 script: %w", err)
	}

	deployPhase4, err := NewDeployPhase4Script(host)
	if err != nil {
		return nil, fmt.Errorf("failed to load DeployPhase4 script: %w", err)
	}

	return &Scripts{
		DeployAlphabetVM:      deployAlphabetVM,
		DeployAltDA:           deployAltDA,
		DeployAsterisc:        deployAsterisc,
		DeployDisputeGame:     deployDisputeGame,
		DeployMIPS:            deployMIPSScript,
		DeployImplementations: deployImplementations,
		DeploySuperchain:      deploySuperchain,
		DeployOPChain:         deployOPChain,
		DeployPhase1:          deployPhase1,
		DeployPhase2:          deployPhase2,
		DeployPhase3:          deployPhase3,
		DeployPhase4:          deployPhase4,
	}, nil
}
