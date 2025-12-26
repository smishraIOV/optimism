#!/usr/bin/env python3
"""Analyze gas usage of all transactions on L1 (Anvil)"""

import json
import urllib.request
import sys

L1_RPC = "http://localhost:8545"

def rpc(method, params=[]):
    data = json.dumps({"jsonrpc": "2.0", "method": method, "params": params, "id": 1}).encode()
    req = urllib.request.Request(L1_RPC, data=data, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())["result"]

def analyze():
    latest = int(rpc("eth_blockNumber"), 16)
    print(f"Latest block: {latest}")
    print()
    print(f"{'Block':>6} | {'Gas Used':>12} | {'Gas Limit':>12} | {'% Used':>7} | {'Status':>8} | To/Contract")
    print("-" * 90)

    total_gas = 0
    exceeds_4m = []
    all_txs = []

    for block_num in range(1, latest + 1):
        block = rpc("eth_getBlockByNumber", [hex(block_num), True])
        if not block or not block.get("transactions"):
            continue

        for tx in block["transactions"]:
            tx_hash = tx["hash"]
            receipt = rpc("eth_getTransactionReceipt", [tx_hash])

            gas_used = int(receipt["gasUsed"], 16)
            gas_limit = int(tx["gas"], 16)
            pct = (gas_used / gas_limit) * 100 if gas_limit > 0 else 0
            status = "✓" if receipt["status"] == "0x1" else "✗"

            # Get contract created (if any)
            to_addr = tx.get("to") or "CREATE"
            contract_created = receipt.get("contractAddress", "")
            if contract_created:
                to_addr = f"CREATE → {contract_created[:10]}..."
            elif to_addr != "CREATE":
                to_addr = to_addr[:20] + "..."

            total_gas += gas_used

            all_txs.append({
                "block": block_num,
                "gas_used": gas_used,
                "gas_limit": gas_limit,
                "to": to_addr,
                "hash": tx_hash
            })

            # Flag transactions exceeding 4M
            flag = " ⚠️" if gas_used > 4_000_000 else ""
            if gas_used > 4_000_000:
                exceeds_4m.append((block_num, gas_used, gas_limit, to_addr))

            print(f"{block_num:>6} | {gas_used:>12,} | {gas_limit:>12,} | {pct:>6.1f}% | {status:>8} | {to_addr}{flag}")

    print("-" * 90)
    print(f"Total gas used: {total_gas:,}")
    print(f"Total transactions: {len(all_txs)}")
    print()

    if exceeds_4m:
        print(f"⚠️  {len(exceeds_4m)} transactions exceed 4M gas limit:")
        for block, gas_used, gas_limit, addr in exceeds_4m:
            print(f"   Block {block}: {gas_used:,} gas (limit: {gas_limit:,}) - {addr}")
    else:
        print("✓ All transactions are under 4M gas!")

    # Show top 10 by gas usage
    print()
    print("Top 10 transactions by gas used:")
    sorted_txs = sorted(all_txs, key=lambda x: x["gas_used"], reverse=True)[:10]
    for i, tx in enumerate(sorted_txs, 1):
        exceeds = " ⚠️ EXCEEDS 4M" if tx["gas_used"] > 4_000_000 else ""
        print(f"  {i}. Block {tx['block']}: {tx['gas_used']:,} gas - {tx['to']}{exceeds}")

if __name__ == "__main__":
    analyze()

