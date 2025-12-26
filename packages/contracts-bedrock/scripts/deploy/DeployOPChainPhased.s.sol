// SPDX-License-Identifier: MIT
pragma solidity 0.8.15;

import { Script } from "forge-std/Script.sol";

import { DeployUtils } from "scripts/libraries/DeployUtils.sol";
import { Types } from "scripts/libraries/Types.sol";

import { IOPContractsManager } from "interfaces/L1/IOPContractsManager.sol";
import { IOPContractsManagerDeployer } from "interfaces/L1/IOPContractsManager.sol";
import { ISuperchainConfig } from "interfaces/L1/ISuperchainConfig.sol";

/// @title DeployOPChainPhased
/// @notice A Forge script that deploys an OP Chain using phased deployment.
///         This splits the deployment into 4 separate transactions, each under 4M gas.
contract DeployOPChainPhased is Script {
    /// @notice Input for Phase 1
    struct Phase1Input {
        Types.DeployOPChainInput deployInput;
        address opcmDeployer;
    }

    /// @notice Output from Phase 1 (matches IOPContractsManagerDeployer.DeployPhase1Output)
    struct Phase1Output {
        address addressManager;
        address opChainProxyAdmin;
    }

    /// @notice Input for Phase 2
    struct Phase2Input {
        Types.DeployOPChainInput deployInput;
        address opcmDeployer;
        Phase1Output phase1;
    }

    /// @notice Output from Phase 2
    struct Phase2Output {
        address addressManager;
        address opChainProxyAdmin;
        address l1ERC721BridgeProxy;
        address optimismPortalProxy;
        address ethLockboxProxy;
        address systemConfigProxy;
        address optimismMintableERC20FactoryProxy;
        address disputeGameFactoryProxy;
        address anchorStateRegistryProxy;
    }

    /// @notice Input for Phase 3
    struct Phase3Input {
        Types.DeployOPChainInput deployInput;
        address opcmDeployer;
        Phase2Output phase2;
    }

    /// @notice Output from Phase 3
    struct Phase3Output {
        address addressManager;
        address opChainProxyAdmin;
        address l1ERC721BridgeProxy;
        address optimismPortalProxy;
        address ethLockboxProxy;
        address systemConfigProxy;
        address optimismMintableERC20FactoryProxy;
        address disputeGameFactoryProxy;
        address anchorStateRegistryProxy;
        address l1StandardBridgeProxy;
        address l1CrossDomainMessengerProxy;
        address delayedWETHPermissionedGameProxy;
    }

    /// @notice Input for Phase 4
    struct Phase4Input {
        Types.DeployOPChainInput deployInput;
        address opcmDeployer;
        Phase3Output phase3;
        address superchainConfig;
    }

    /// @notice Final output (matches DeployOPChain.Output)
    struct FinalOutput {
        address opChainProxyAdmin;
        address addressManager;
        address l1ERC721BridgeProxy;
        address systemConfigProxy;
        address optimismMintableERC20FactoryProxy;
        address l1StandardBridgeProxy;
        address l1CrossDomainMessengerProxy;
        address optimismPortalProxy;
        address ethLockboxProxy;
        address disputeGameFactoryProxy;
        address anchorStateRegistryProxy;
        address faultDisputeGame;
        address permissionedDisputeGame;
        address delayedWETHPermissionedGameProxy;
        address delayedWETHPermissionlessGameProxy;
    }

    // ============================================================
    // Phase 1: Deploy singletons
    // ============================================================

    function runPhase1WithBytes(bytes memory _input) public returns (bytes memory) {
        Phase1Input memory input = abi.decode(_input, (Phase1Input));
        Phase1Output memory output = runPhase1(input);
        return abi.encode(output);
    }

    function runPhase1(Phase1Input memory _input) public returns (Phase1Output memory output_) {
        IOPContractsManagerDeployer opcmDeployer = IOPContractsManagerDeployer(_input.opcmDeployer);
        IOPContractsManager.DeployInput memory deployInput = toOPCMDeployInput(_input.deployInput);

        vm.broadcast(msg.sender);
        IOPContractsManagerDeployer.DeployPhase1Output memory phase1Out = opcmDeployer.deployPhase1(deployInput);

        output_ = Phase1Output({
            addressManager: address(phase1Out.addressManager),
            opChainProxyAdmin: address(phase1Out.opChainProxyAdmin)
        });
    }

    // ============================================================
    // Phase 2: Deploy ERC-1967 proxies
    // ============================================================

    function runPhase2WithBytes(bytes memory _input) public returns (bytes memory) {
        Phase2Input memory input = abi.decode(_input, (Phase2Input));
        Phase2Output memory output = runPhase2(input);
        return abi.encode(output);
    }

    function runPhase2(Phase2Input memory _input) public returns (Phase2Output memory output_) {
        IOPContractsManagerDeployer opcmDeployer = IOPContractsManagerDeployer(_input.opcmDeployer);
        IOPContractsManager.DeployInput memory deployInput = toOPCMDeployInput(_input.deployInput);

        IOPContractsManagerDeployer.DeployPhase1Output memory phase1In = toPhase1Output(_input.phase1);

        vm.broadcast(msg.sender);
        IOPContractsManagerDeployer.DeployPhase2Output memory phase2Out = opcmDeployer.deployPhase2(deployInput, phase1In);

        output_ = Phase2Output({
            addressManager: address(phase2Out.addressManager),
            opChainProxyAdmin: address(phase2Out.opChainProxyAdmin),
            l1ERC721BridgeProxy: address(phase2Out.l1ERC721BridgeProxy),
            optimismPortalProxy: address(phase2Out.optimismPortalProxy),
            ethLockboxProxy: address(phase2Out.ethLockboxProxy),
            systemConfigProxy: address(phase2Out.systemConfigProxy),
            optimismMintableERC20FactoryProxy: address(phase2Out.optimismMintableERC20FactoryProxy),
            disputeGameFactoryProxy: address(phase2Out.disputeGameFactoryProxy),
            anchorStateRegistryProxy: address(phase2Out.anchorStateRegistryProxy)
        });
    }

    // ============================================================
    // Phase 3: Deploy legacy proxies
    // ============================================================

    function runPhase3WithBytes(bytes memory _input) public returns (bytes memory) {
        Phase3Input memory input = abi.decode(_input, (Phase3Input));
        Phase3Output memory output = runPhase3(input);
        return abi.encode(output);
    }

    function runPhase3(Phase3Input memory _input) public returns (Phase3Output memory output_) {
        IOPContractsManagerDeployer opcmDeployer = IOPContractsManagerDeployer(_input.opcmDeployer);
        IOPContractsManager.DeployInput memory deployInput = toOPCMDeployInput(_input.deployInput);

        IOPContractsManagerDeployer.DeployPhase2Output memory phase2In = toPhase2Output(_input.phase2);

        vm.broadcast(msg.sender);
        IOPContractsManagerDeployer.DeployPhase3Output memory phase3Out = opcmDeployer.deployPhase3(deployInput, phase2In);

        output_ = Phase3Output({
            addressManager: address(phase3Out.addressManager),
            opChainProxyAdmin: address(phase3Out.opChainProxyAdmin),
            l1ERC721BridgeProxy: address(phase3Out.l1ERC721BridgeProxy),
            optimismPortalProxy: address(phase3Out.optimismPortalProxy),
            ethLockboxProxy: address(phase3Out.ethLockboxProxy),
            systemConfigProxy: address(phase3Out.systemConfigProxy),
            optimismMintableERC20FactoryProxy: address(phase3Out.optimismMintableERC20FactoryProxy),
            disputeGameFactoryProxy: address(phase3Out.disputeGameFactoryProxy),
            anchorStateRegistryProxy: address(phase3Out.anchorStateRegistryProxy),
            l1StandardBridgeProxy: address(phase3Out.l1StandardBridgeProxy),
            l1CrossDomainMessengerProxy: address(phase3Out.l1CrossDomainMessengerProxy),
            delayedWETHPermissionedGameProxy: address(phase3Out.delayedWETHPermissionedGameProxy)
        });
    }

    // ============================================================
    // Phase 4: Initialize and finalize
    // ============================================================

    function runPhase4WithBytes(bytes memory _input) public returns (bytes memory) {
        Phase4Input memory input = abi.decode(_input, (Phase4Input));
        FinalOutput memory output = runPhase4(input);
        return abi.encode(output);
    }

    function runPhase4(Phase4Input memory _input) public returns (FinalOutput memory output_) {
        IOPContractsManagerDeployer opcmDeployer = IOPContractsManagerDeployer(_input.opcmDeployer);
        IOPContractsManager.DeployInput memory deployInput = toOPCMDeployInput(_input.deployInput);
        ISuperchainConfig superchainConfig = ISuperchainConfig(_input.superchainConfig);

        IOPContractsManagerDeployer.DeployPhase3Output memory phase3In = toPhase3Output(_input.phase3);

        vm.broadcast(msg.sender);
        IOPContractsManager.DeployOutput memory finalOut = opcmDeployer.deployPhase4(
            deployInput,
            phase3In,
            superchainConfig,
            msg.sender
        );

        output_ = FinalOutput({
            opChainProxyAdmin: address(finalOut.opChainProxyAdmin),
            addressManager: address(finalOut.addressManager),
            l1ERC721BridgeProxy: address(finalOut.l1ERC721BridgeProxy),
            systemConfigProxy: address(finalOut.systemConfigProxy),
            optimismMintableERC20FactoryProxy: address(finalOut.optimismMintableERC20FactoryProxy),
            l1StandardBridgeProxy: address(finalOut.l1StandardBridgeProxy),
            l1CrossDomainMessengerProxy: address(finalOut.l1CrossDomainMessengerProxy),
            optimismPortalProxy: address(finalOut.optimismPortalProxy),
            ethLockboxProxy: address(finalOut.ethLockboxProxy),
            disputeGameFactoryProxy: address(finalOut.disputeGameFactoryProxy),
            anchorStateRegistryProxy: address(finalOut.anchorStateRegistryProxy),
            faultDisputeGame: address(finalOut.faultDisputeGame),
            permissionedDisputeGame: address(finalOut.permissionedDisputeGame),
            delayedWETHPermissionedGameProxy: address(finalOut.delayedWETHPermissionedGameProxy),
            delayedWETHPermissionlessGameProxy: address(finalOut.delayedWETHPermissionlessGameProxy)
        });
    }

    // ============================================================
    // Conversion helpers
    // ============================================================

    function toOPCMDeployInput(Types.DeployOPChainInput memory _input)
        internal
        pure
        returns (IOPContractsManager.DeployInput memory)
    {
        IOPContractsManager.Roles memory roles = IOPContractsManager.Roles({
            opChainProxyAdminOwner: _input.opChainProxyAdminOwner,
            systemConfigOwner: _input.systemConfigOwner,
            batcher: _input.batcher,
            unsafeBlockSigner: _input.unsafeBlockSigner,
            proposer: _input.proposer,
            challenger: _input.challenger
        });

        return IOPContractsManager.DeployInput({
            roles: roles,
            basefeeScalar: _input.basefeeScalar,
            blobBasefeeScalar: _input.blobBaseFeeScalar,
            l2ChainId: _input.l2ChainId,
            startingAnchorRoot: startingAnchorRoot(),
            saltMixer: _input.saltMixer,
            gasLimit: _input.gasLimit,
            disputeGameType: _input.disputeGameType,
            disputeAbsolutePrestate: _input.disputeAbsolutePrestate,
            disputeMaxGameDepth: _input.disputeMaxGameDepth,
            disputeSplitDepth: _input.disputeSplitDepth,
            disputeClockExtension: _input.disputeClockExtension,
            disputeMaxClockDuration: _input.disputeMaxClockDuration,
            useCustomGasToken: _input.useCustomGasToken
        });
    }

    function toPhase1Output(Phase1Output memory _output)
        internal
        pure
        returns (IOPContractsManagerDeployer.DeployPhase1Output memory)
    {
        return IOPContractsManagerDeployer.DeployPhase1Output({
            addressManager: IAddressManager(_output.addressManager),
            opChainProxyAdmin: IProxyAdmin(_output.opChainProxyAdmin)
        });
    }

    function toPhase2Output(Phase2Output memory _output)
        internal
        pure
        returns (IOPContractsManagerDeployer.DeployPhase2Output memory)
    {
        return IOPContractsManagerDeployer.DeployPhase2Output({
            addressManager: IAddressManager(_output.addressManager),
            opChainProxyAdmin: IProxyAdmin(_output.opChainProxyAdmin),
            l1ERC721BridgeProxy: IL1ERC721Bridge(_output.l1ERC721BridgeProxy),
            optimismPortalProxy: IOptimismPortal2(payable(_output.optimismPortalProxy)),
            ethLockboxProxy: IETHLockbox(_output.ethLockboxProxy),
            systemConfigProxy: ISystemConfig(_output.systemConfigProxy),
            optimismMintableERC20FactoryProxy: IOptimismMintableERC20Factory(_output.optimismMintableERC20FactoryProxy),
            disputeGameFactoryProxy: IDisputeGameFactory(_output.disputeGameFactoryProxy),
            anchorStateRegistryProxy: IAnchorStateRegistry(_output.anchorStateRegistryProxy)
        });
    }

    function toPhase3Output(Phase3Output memory _output)
        internal
        pure
        returns (IOPContractsManagerDeployer.DeployPhase3Output memory)
    {
        return IOPContractsManagerDeployer.DeployPhase3Output({
            addressManager: IAddressManager(_output.addressManager),
            opChainProxyAdmin: IProxyAdmin(_output.opChainProxyAdmin),
            l1ERC721BridgeProxy: IL1ERC721Bridge(_output.l1ERC721BridgeProxy),
            optimismPortalProxy: IOptimismPortal2(payable(_output.optimismPortalProxy)),
            ethLockboxProxy: IETHLockbox(_output.ethLockboxProxy),
            systemConfigProxy: ISystemConfig(_output.systemConfigProxy),
            optimismMintableERC20FactoryProxy: IOptimismMintableERC20Factory(_output.optimismMintableERC20FactoryProxy),
            disputeGameFactoryProxy: IDisputeGameFactory(_output.disputeGameFactoryProxy),
            anchorStateRegistryProxy: IAnchorStateRegistry(_output.anchorStateRegistryProxy),
            l1StandardBridgeProxy: IL1StandardBridge(payable(_output.l1StandardBridgeProxy)),
            l1CrossDomainMessengerProxy: IL1CrossDomainMessenger(_output.l1CrossDomainMessengerProxy),
            delayedWETHPermissionedGameProxy: IDelayedWETH(payable(_output.delayedWETHPermissionedGameProxy))
        });
    }

    function startingAnchorRoot() internal pure returns (bytes memory) {
        // Same as DeployOPChain.sol
        return abi.encode(
            // Proposal struct: root, l2BlockNumber
            bytes32(0xdead000000000000000000000000000000000000000000000000000000000000),
            uint256(0)
        );
    }
}

// Import interfaces needed for conversion helpers
import { IAddressManager } from "interfaces/legacy/IAddressManager.sol";
import { IProxyAdmin } from "interfaces/universal/IProxyAdmin.sol";
import { IL1ERC721Bridge } from "interfaces/L1/IL1ERC721Bridge.sol";
import { IOptimismPortal2 } from "interfaces/L1/IOptimismPortal2.sol";
import { IETHLockbox } from "interfaces/L1/IETHLockbox.sol";
import { ISystemConfig } from "interfaces/L1/ISystemConfig.sol";
import { IOptimismMintableERC20Factory } from "interfaces/universal/IOptimismMintableERC20Factory.sol";
import { IDisputeGameFactory } from "interfaces/dispute/IDisputeGameFactory.sol";
import { IAnchorStateRegistry } from "interfaces/dispute/IAnchorStateRegistry.sol";
import { IL1StandardBridge } from "interfaces/L1/IL1StandardBridge.sol";
import { IL1CrossDomainMessenger } from "interfaces/L1/IL1CrossDomainMessenger.sol";
import { IDelayedWETH } from "interfaces/dispute/IDelayedWETH.sol";

