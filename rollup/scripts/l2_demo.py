#!/usr/bin/env python3
"""
L2 Demo Script - Send transactions on the Optimism L2 rollup using cast

Usage:
    python3 l2_demo.py balances              # Show all account balances
    python3 l2_demo.py status                # Show L2 sync status
    python3 l2_demo.py send <from> <to> <eth> # Send ETH between accounts (by index)
    python3 l2_demo.py fund-accounts         # Send 0.1 ETH from account 1 to accounts 0,2,3,4,5

Examples:
    python3 l2_demo.py balances
    python3 l2_demo.py send 1 0 0.1          # Send 0.1 ETH from account 1 to account 0
    python3 l2_demo.py fund-accounts         # Fund other accounts from account 1
"""

import json
import subprocess
import sys
import urllib.request
from typing import Optional, Tuple

# Configuration
L2_RPC = "http://localhost:9545"
L1_RPC = "http://localhost:8545"
OP_NODE_RPC = "http://localhost:8547"

# Anvil seed 2 accounts
ACCOUNTS = [
    {"name": "Account 0", "address": "0x8995E44a22e303A79bdD2E6e41674fb92d620863", "key": "0xd6a036f561e03196779dd34bf3d141dec4737eec5ed0416e413985ca05dad51a"},
    {"name": "Account 1", "address": "0xE9e05C9f02e10FA833D379CB1c7aC3a3f23B247e", "key": "0xbe62250c9db006c67c1595ff1f019bc849e2aa5c092dea0bf00883b39e54e904"},
    {"name": "Account 2", "address": "0x61Da7c7F97EBE53AD7c4E5eCD3d117E7Ab430eA7", "key": "0xc3bae29d211b5523ccdc349e8275cc57a291a03558be6f6ec799c196702ef881"},
    {"name": "Account 3", "address": "0x5b0248e30583CeD4F09726C547935552C469EB24", "key": "0x4bc19d3b0467a84723ad48118ed28884105526458d5a64a716ebea568318e3d0"},
    {"name": "Account 4", "address": "0xcDbc8abb83E01BaE13ECE8853a5Ca84b2Ef6Ca86", "key": "0x07127c605875395527bccbc5c74afa9dc1712d83ba5b12207bab86d3b7fa8be6"},
    {"name": "Account 5", "address": "0xa683a3E33E07fb84ff33FcE753Da1d248298977f", "key": "0x309bc84a97ca76f0c15bc3a6c98d46d8fb2381c8bd63ba6cc7b0d25ff4a13332"},
    {"name": "Account 6", "address": "0x008099bFee75e832e1b93D4c023f646d99d4C90f", "key": "0x3d4471ad1080f3c193b0a1f299ad78e3b9d7119c2bc841b17fa3317a627f2818"},
    {"name": "Account 7", "address": "0x38aDCae107e9aEd4C6dfFA317d651E80CCCE0857", "key": "0xf583a7d3a6cfe88ea61f0d3ab3ef5dc167636defe2b6f8c8f06c8981267ba401"},
]


def rpc_call(url: str, method: str, params: list) -> dict:
    """Make a JSON-RPC call."""
    data = json.dumps({"jsonrpc": "2.0", "method": method, "params": params, "id": 1}).encode()
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.loads(resp.read())
    except Exception as e:
        return {"error": str(e)}


def get_balance(address: str, rpc_url: str = L2_RPC) -> float:
    """Get balance in ETH."""
    result = rpc_call(rpc_url, "eth_getBalance", [address, "latest"])
    if "result" in result:
        return int(result["result"], 16) / 1e18
    return 0.0


def get_nonce(address: str, rpc_url: str = L2_RPC) -> int:
    """Get transaction count (nonce)."""
    result = rpc_call(rpc_url, "eth_getTransactionCount", [address, "latest"])
    if "result" in result:
        return int(result["result"], 16)
    return 0


def show_balances():
    """Show balances of all accounts on L2."""
    print("\n=== L2 Account Balances ===\n")
    print(f"{'Account':<12} {'Address':<44} {'Balance':>14} {'Nonce':>6}")
    print("-" * 80)

    total = 0.0
    for acc in ACCOUNTS:
        bal = get_balance(acc["address"])
        nonce = get_nonce(acc["address"])
        total += bal
        print(f"{acc['name']:<12} {acc['address']:<44} {bal:>12.4f} ETH {nonce:>5}")

    print("-" * 80)
    print(f"{'Total':<12} {'':<44} {total:>12.4f} ETH")
    print()


def show_status():
    """Show L2 sync status."""
    print("\n=== L2 Sync Status ===\n")

    # L2 block number
    l2_block = rpc_call(L2_RPC, "eth_blockNumber", [])
    if "result" in l2_block:
        print(f"L2 Block: {int(l2_block['result'], 16)}")

    # L1 block number
    l1_block = rpc_call(L1_RPC, "eth_blockNumber", [])
    if "result" in l1_block:
        print(f"L1 Block: {int(l1_block['result'], 16)}")

    # Sync status from op-node
    sync = rpc_call(OP_NODE_RPC, "optimism_syncStatus", [])
    if "result" in sync:
        r = sync["result"]
        print(f"\nUnsafe L2: {r.get('unsafe_l2', {}).get('number', 'N/A')}")
        print(f"Safe L2: {r.get('safe_l2', {}).get('number', 'N/A')}")
        print(f"Finalized L2: {r.get('finalized_l2', {}).get('number', 'N/A')}")
        print(f"L1 Origin: {r.get('unsafe_l2', {}).get('l1origin', {}).get('number', 'N/A')}")
    print()


def run_cast(args: list) -> Tuple[bool, str]:
    """Run a cast command and return (success, output)."""
    import os
    env = os.environ.copy()
    env["NO_PROXY"] = "*"  # Workaround for macOS Foundry bug

    try:
        result = subprocess.run(
            ["cast"] + args,
            capture_output=True,
            text=True,
            timeout=30,
            env=env
        )
        if result.returncode == 0:
            return True, result.stdout
        else:
            return False, result.stderr
    except subprocess.TimeoutExpired:
        return False, "Command timed out"
    except FileNotFoundError:
        return False, "cast not found. Install Foundry: https://getfoundry.sh"
    except Exception as e:
        return False, str(e)


def send_eth(from_index: int, to_index: int, amount_eth: float) -> Optional[str]:
    """Send ETH from one account to another on L2 using cast."""
    sender = ACCOUNTS[from_index]
    recipient = ACCOUNTS[to_index]

    print(f"\n=== Sending {amount_eth} ETH ===")
    print(f"From: {sender['name']} ({sender['address']})")
    print(f"To: {recipient['name']} ({recipient['address']})")

    # Check sender balance
    sender_bal = get_balance(sender["address"])
    if sender_bal < amount_eth:
        print(f"ERROR: Insufficient balance. Have {sender_bal:.4f} ETH, need {amount_eth}")
        return None

    # Build cast command
    args = [
        "send",
        "--rpc-url", L2_RPC,
        "--private-key", sender["key"],
        recipient["address"],
        "--value", f"{amount_eth}ether"
    ]

    print("Sending transaction...")
    success, output = run_cast(args)

    if success:
        # Parse transaction hash from output
        for line in output.split('\n'):
            if 'transactionHash' in line:
                tx_hash = line.split()[-1]
                print(f"✓ Transaction hash: {tx_hash}")
                break

        # Show status
        for line in output.split('\n'):
            if 'status' in line.lower() and 'success' in line.lower():
                print("✓ Status: success")
                break
            elif 'blockNumber' in line:
                block = line.split()[-1]
                print(f"✓ Included in block: {block}")

        return tx_hash if 'tx_hash' in dir() else "success"
    else:
        print(f"ERROR: {output}")
        return None


def fund_accounts():
    """Fund accounts 0, 2, 3, 4, 5 with 0.1 ETH each from account 1."""
    print("\n=== Funding Accounts ===")
    print("Sending 0.1 ETH from Account 1 to Accounts 0, 2, 3, 4, 5\n")

    # Check sender balance
    sender_bal = get_balance(ACCOUNTS[1]["address"])
    print(f"Account 1 balance: {sender_bal:.4f} ETH")

    needed = 0.5  # 0.1 * 5 accounts
    if sender_bal < needed:
        print(f"ERROR: Insufficient balance. Need at least {needed} ETH, have {sender_bal:.4f}")
        return

    recipients = [0, 2, 3, 4, 5]
    successful = 0

    for idx in recipients:
        result = send_eth(1, idx, 0.1)
        if result:
            successful += 1
        else:
            print(f"Failed to send to Account {idx}, stopping.")
            break
        print()

    print(f"\n=== Summary ===")
    print(f"Successful transfers: {successful}/{len(recipients)}")
    print("\n=== Final Balances ===")
    show_balances()


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    cmd = sys.argv[1]

    if cmd == "balances":
        show_balances()

    elif cmd == "status":
        show_status()

    elif cmd == "send":
        if len(sys.argv) != 5:
            print("Usage: python3 l2_demo.py send <from_index> <to_index> <amount_eth>")
            print("Example: python3 l2_demo.py send 1 0 0.1")
            sys.exit(1)
        from_idx = int(sys.argv[2])
        to_idx = int(sys.argv[3])
        amount = float(sys.argv[4])

        if from_idx < 0 or from_idx >= len(ACCOUNTS):
            print(f"ERROR: from_index must be 0-{len(ACCOUNTS)-1}")
            sys.exit(1)
        if to_idx < 0 or to_idx >= len(ACCOUNTS):
            print(f"ERROR: to_index must be 0-{len(ACCOUNTS)-1}")
            sys.exit(1)

        send_eth(from_idx, to_idx, amount)

    elif cmd == "fund-accounts":
        fund_accounts()

    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
