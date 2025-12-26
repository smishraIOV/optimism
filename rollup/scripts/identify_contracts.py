#!/usr/bin/env python3
"""Identify which contracts were deployed in high-gas transactions"""

import json
import urllib.request

L1_RPC = "http://localhost:8545"

def rpc(method, params=[]):
    data = json.dumps({"jsonrpc": "2.0", "method": method, "params": params, "id": 1}).encode()
    req = urllib.request.Request(L1_RPC, data=data, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())["result"]

def main():
    # Load state.json to get deployed addresses
    with open("rollup/deployer/.deployer/state.json") as f:
        state = json.load(f)

    # Build address -> name mapping
    addr_to_name = {}

    # Implementation contracts
    if state.get("implementationsDeployment"):
        for name, addr in state["implementationsDeployment"].items():
            if addr and addr != "0x0000000000000000000000000000000000000000":
                addr_to_name[addr.lower()] = name

    # Superchain contracts
    if state.get("superchainContracts"):
        for name, addr in state["superchainContracts"].items():
            if addr and addr != "0x0000000000000000000000000000000000000000":
                addr_to_name[addr.lower()] = name

    # OP Chain contracts
    if state.get("opChainDeployments") and len(state["opChainDeployments"]) > 0:
        for name, value in state["opChainDeployments"][0].items():
            if isinstance(value, str) and value.startswith("0x") and len(value) == 42:
                addr_to_name[value.lower()] = name

    print(f"Loaded {len(addr_to_name)} contract addresses from state.json")
    print()

    # Get all transactions and find which contracts were created
    latest = int(rpc("eth_blockNumber"), 16)

    print("Transactions exceeding 4M gas with contract identification:")
    print("-" * 100)
    print(f"{'Block':>6} | {'Gas Used':>12} | Contract Created")
    print("-" * 100)

    for block_num in range(1, latest + 1):
        block = rpc("eth_getBlockByNumber", [hex(block_num), True])
        if not block or not block.get("transactions"):
            continue

        for tx in block["transactions"]:
            receipt = rpc("eth_getTransactionReceipt", [tx["hash"]])
            gas_used = int(receipt["gasUsed"], 16)

            if gas_used > 4_000_000:
                # Check if this created a contract
                contract_addr = receipt.get("contractAddress")

                # Also check logs for contract creation events
                contract_name = "Unknown"

                if contract_addr:
                    contract_name = addr_to_name.get(contract_addr.lower(), f"Unknown ({contract_addr[:20]}...)")
                else:
                    # This is a call to CREATE2 deployer - check internal contract creations
                    # We need to trace to find created contracts, but we can check if any known
                    # addresses were created around this block

                    # Check trace for internal creates
                    try:
                        trace = rpc("debug_traceTransaction", [tx["hash"], {"tracer": "callTracer"}])
                        if trace:
                            # Look for CREATE/CREATE2 operations in trace
                            def find_creates(call, depth=0):
                                creates = []
                                if call.get("type") in ["CREATE", "CREATE2"]:
                                    created = call.get("to", "")
                                    if created:
                                        name = addr_to_name.get(created.lower(), f"Unknown ({created[:16]}...)")
                                        creates.append(name)
                                for sub in call.get("calls", []):
                                    creates.extend(find_creates(sub, depth+1))
                                return creates

                            created = find_creates(trace)
                            if created:
                                contract_name = ", ".join(created[:3])
                                if len(created) > 3:
                                    contract_name += f" (+{len(created)-3} more)"
                    except Exception as e:
                        contract_name = "Unknown (trace failed)"

                print(f"{block_num:>6} | {gas_used:>12,} | {contract_name}")

    print("-" * 100)
    print()
    print("Known large contracts from state.json:")

    # Show implementation contracts (likely the large ones)
    if state.get("implementationsDeployment"):
        print("\nImplementation contracts:")
        for name, addr in sorted(state["implementationsDeployment"].items()):
            if addr and addr != "0x0000000000000000000000000000000000000000":
                # Check bytecode size
                code = rpc("eth_getCode", [addr, "latest"])
                code_size = (len(code) - 2) // 2 if code else 0
                if code_size > 10000:  # Show contracts > 10KB
                    print(f"  {name}: {code_size:,} bytes ({addr[:16]}...)")

if __name__ == "__main__":
    main()

