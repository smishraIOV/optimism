#!/usr/bin/env python3
"""
Proposer Status Script - Monitor op-proposer activity on L1

Usage:
    python3 proposer_status.py           # Show proposer status and recent transactions
    python3 proposer_status.py watch     # Continuously monitor (every 30 seconds)
"""

import json
import sys
import time
import urllib.request

# Configuration
L1_RPC = "http://localhost:8545"
L2_RPC = "http://localhost:9545"
OP_NODE_RPC = "http://localhost:8547"

# Proposer address (Anvil seed 2, Account 5)
PROPOSER_ADDRESS = "0xa683a3E33E07fb84ff33FcE753Da1d248298977f"

# DisputeGameFactory address from deployment
GAME_FACTORY_ADDRESS = "0xc2f53f7e5ae6180682e9353b34ec544053784a91"


def rpc_call(url: str, method: str, params: list) -> dict:
    """Make a JSON-RPC call."""
    data = json.dumps({"jsonrpc": "2.0", "method": method, "params": params, "id": 1}).encode()
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.loads(resp.read())
    except Exception as e:
        return {"error": str(e)}


def get_proposer_stats():
    """Get proposer account statistics."""
    stats = {}

    # Nonce (number of transactions submitted)
    nonce_result = rpc_call(L1_RPC, "eth_getTransactionCount", [PROPOSER_ADDRESS, "latest"])
    if "result" in nonce_result:
        stats["nonce"] = int(nonce_result["result"], 16)
    else:
        stats["nonce"] = 0

    # Balance
    bal_result = rpc_call(L1_RPC, "eth_getBalance", [PROPOSER_ADDRESS, "latest"])
    if "result" in bal_result:
        stats["balance_eth"] = int(bal_result["result"], 16) / 1e18
    else:
        stats["balance_eth"] = 0

    # Gas spent (assuming started with 10000 ETH)
    stats["gas_spent_eth"] = 10000 - stats["balance_eth"]

    return stats


def get_recent_proposer_transactions(limit: int = 5):
    """Find recent transactions from the proposer on L1."""
    transactions = []

    # Get latest block
    latest_result = rpc_call(L1_RPC, "eth_blockNumber", [])
    if "result" not in latest_result:
        return transactions

    latest_block = int(latest_result["result"], 16)

    # Search recent blocks for proposer transactions
    blocks_to_search = min(200, latest_block)

    for i in range(blocks_to_search):
        block_num = latest_block - i
        block = rpc_call(L1_RPC, "eth_getBlockByNumber", [hex(block_num), True])

        if block.get("result") and block["result"].get("transactions"):
            for tx in block["result"]["transactions"]:
                if tx.get("from", "").lower() == PROPOSER_ADDRESS.lower():
                    tx_info = {
                        "block": block_num,
                        "hash": tx["hash"],
                        "to": tx.get("to", "Contract Creation"),
                        "gas": int(tx["gas"], 16),
                        "value": int(tx["value"], 16) / 1e18 if tx.get("value") else 0
                    }
                    transactions.append(tx_info)

                    if len(transactions) >= limit:
                        return transactions

    return transactions


def get_l2_sync_status():
    """Get L2 sync status from op-node."""
    sync_result = rpc_call(OP_NODE_RPC, "optimism_syncStatus", [])
    if "result" in sync_result:
        r = sync_result["result"]
        return {
            "unsafe_l2": r.get("unsafe_l2", {}).get("number", "N/A"),
            "safe_l2": r.get("safe_l2", {}).get("number", "N/A"),
            "finalized_l2": r.get("finalized_l2", {}).get("number", "N/A"),
            "l1_origin": r.get("unsafe_l2", {}).get("l1origin", {}).get("number", "N/A")
        }
    return None


def show_status():
    """Display proposer status."""
    print("\n" + "=" * 60)
    print("                    PROPOSER STATUS")
    print("=" * 60 + "\n")

    # Proposer account stats
    print("📊 Proposer Account")
    print("-" * 40)
    stats = get_proposer_stats()
    print(f"  Address: {PROPOSER_ADDRESS}")
    print(f"  Balance: {stats['balance_eth']:.6f} ETH")
    print(f"  Nonce:   {stats['nonce']} transactions submitted")
    print(f"  Gas spent: {stats['gas_spent_eth']:.6f} ETH")

    # L2 Sync Status
    print("\n📈 L2 Sync Status")
    print("-" * 40)
    sync = get_l2_sync_status()
    if sync:
        print(f"  Unsafe L2:    {sync['unsafe_l2']}")
        print(f"  Safe L2:      {sync['safe_l2']}")
        print(f"  Finalized L2: {sync['finalized_l2']}")
        print(f"  L1 Origin:    {sync['l1_origin']}")
    else:
        print("  Could not get sync status")

    # Recent transactions
    print("\n📝 Recent Proposer Transactions (L1)")
    print("-" * 40)
    txs = get_recent_proposer_transactions(5)
    if txs:
        for tx in txs:
            print(f"  Block {tx['block']}:")
            print(f"    Hash: {tx['hash'][:22]}...")
            print(f"    To:   {tx['to'][:22]}..." if len(tx['to']) > 22 else f"    To:   {tx['to']}")
            print(f"    Gas:  {tx['gas']}")
    else:
        print("  No recent transactions found")

    # L1 block info
    print("\n🔗 Chain Status")
    print("-" * 40)
    l1_block = rpc_call(L1_RPC, "eth_blockNumber", [])
    l2_block = rpc_call(L2_RPC, "eth_blockNumber", [])
    if "result" in l1_block:
        print(f"  L1 Block: {int(l1_block['result'], 16)}")
    if "result" in l2_block:
        print(f"  L2 Block: {int(l2_block['result'], 16)}")

    print("\n" + "=" * 60 + "\n")


def watch_status(interval: int = 30):
    """Continuously monitor proposer status."""
    print(f"Watching proposer status (refresh every {interval}s, Ctrl+C to stop)...")

    last_nonce = 0

    try:
        while True:
            # Clear screen (optional)
            print("\033[2J\033[H", end="")

            stats = get_proposer_stats()
            current_nonce = stats["nonce"]

            show_status()

            if current_nonce > last_nonce:
                print(f"🆕 New proposal submitted! (nonce: {last_nonce} → {current_nonce})")
                last_nonce = current_nonce
            elif last_nonce == 0:
                last_nonce = current_nonce

            print(f"Next refresh in {interval} seconds...")
            time.sleep(interval)

    except KeyboardInterrupt:
        print("\nStopped watching.")


def main():
    if len(sys.argv) > 1 and sys.argv[1] == "watch":
        interval = int(sys.argv[2]) if len(sys.argv) > 2 else 30
        watch_status(interval)
    else:
        show_status()


if __name__ == "__main__":
    main()

