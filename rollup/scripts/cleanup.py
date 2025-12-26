#!/usr/bin/env python3
"""
Rollup Cleanup Script - Stop processes and clean up data

Usage:
    python3 cleanup.py              # Show what would be cleaned (dry run)
    python3 cleanup.py --execute    # Actually perform cleanup
    python3 cleanup.py --full       # Full cleanup including deployment state (requires --execute)
"""

import os
import shutil
import subprocess
import sys

# Base directory (rollup/)
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
ROLLUP_DIR = os.path.dirname(SCRIPT_DIR)

# Process names to kill
PROCESSES = ["anvil", "geth", "op-node", "op-batcher", "op-proposer", "op-challenger"]

# Directories/files to clean (relative to rollup/)
CLEANUP_ITEMS = [
    # Sequencer runtime data
    {"path": "sequencer/op-geth-data", "type": "dir", "desc": "op-geth chain data"},
    {"path": "sequencer/opnode_discovery_db", "type": "dir", "desc": "op-node discovery DB"},
    {"path": "sequencer/opnode_peerstore_db", "type": "dir", "desc": "op-node peerstore DB"},
    {"path": "sequencer/opnode_p2p_priv.txt", "type": "file", "desc": "op-node P2P key"},
    {"path": "sequencer/jwt.txt", "type": "file", "desc": "JWT secret"},

    # Batcher runtime data
    {"path": "batcher/state.json", "type": "file", "desc": "Batcher state copy"},

    # Proposer runtime data
    {"path": "proposer/state.json", "type": "file", "desc": "Proposer state copy"},

    # Challenger runtime data
    {"path": "challenger/data", "type": "dir", "desc": "Challenger data"},
    {"path": "challenger/state.json", "type": "file", "desc": "Challenger state copy"},
]

# Additional items for full cleanup (includes deployment state)
FULL_CLEANUP_ITEMS = [
    {"path": "deployer/.deployer/state.json", "type": "file", "desc": "Deployment state"},
    {"path": "sequencer/genesis.json", "type": "file", "desc": "L2 genesis"},
    {"path": "sequencer/rollup.json", "type": "file", "desc": "Rollup config"},
]


def kill_processes(dry_run: bool = True) -> int:
    """Kill all rollup processes."""
    killed = 0

    for proc_name in PROCESSES:
        try:
            result = subprocess.run(
                ["pgrep", "-f", proc_name],
                capture_output=True,
                text=True
            )
            if result.returncode == 0:
                pids = [p for p in result.stdout.strip().split("\n") if p]
                for pid in pids:
                    # Get process info
                    ps_result = subprocess.run(
                        ["ps", "-p", pid, "-o", "command="],
                        capture_output=True,
                        text=True
                    )
                    cmd = ps_result.stdout.strip() if ps_result.returncode == 0 else ""

                    # Skip this script or editors
                    if "cleanup.py" in cmd or "vim" in cmd or "code" in cmd:
                        continue

                    if dry_run:
                        print(f"  Would kill: {proc_name} (PID {pid})")
                    else:
                        subprocess.run(["kill", pid], capture_output=True)
                        print(f"  ✓ Killed: {proc_name} (PID {pid})")
                    killed += 1
        except Exception as e:
            print(f"  ✗ Error checking {proc_name}: {e}")

    return killed


def clean_items(items: list, dry_run: bool = True) -> tuple:
    """Clean up files and directories."""
    cleaned = 0
    size_freed = 0

    for item in items:
        full_path = os.path.join(ROLLUP_DIR, item["path"])

        if not os.path.exists(full_path):
            continue

        # Calculate size
        if item["type"] == "dir":
            size = sum(
                os.path.getsize(os.path.join(dirpath, filename))
                for dirpath, _, filenames in os.walk(full_path)
                for filename in filenames
            )
        else:
            size = os.path.getsize(full_path)

        size_mb = size / (1024 * 1024)

        if dry_run:
            print(f"  Would remove: {item['path']} ({item['desc']}) - {size_mb:.1f} MB")
        else:
            try:
                if item["type"] == "dir":
                    shutil.rmtree(full_path)
                else:
                    os.remove(full_path)
                print(f"  ✓ Removed: {item['path']} - {size_mb:.1f} MB")
                cleaned += 1
                size_freed += size
            except Exception as e:
                print(f"  ✗ Failed to remove {item['path']}: {e}")

    return cleaned, size_freed


def main():
    dry_run = "--execute" not in sys.argv
    full_cleanup = "--full" in sys.argv

    print("\n" + "=" * 70)
    if dry_run:
        print("              ROLLUP CLEANUP - DRY RUN (no changes)")
        print("         Run with --execute to actually perform cleanup")
    else:
        print("              ROLLUP CLEANUP - EXECUTING")
    print("=" * 70)

    # Step 1: Kill processes
    print("\n📋 Step 1: Stop Processes")
    print("-" * 40)
    killed = kill_processes(dry_run)
    if killed == 0:
        print("  No rollup processes running")

    # Step 2: Clean runtime data
    print("\n📋 Step 2: Clean Runtime Data")
    print("-" * 40)
    cleaned, size_freed = clean_items(CLEANUP_ITEMS, dry_run)
    if cleaned == 0 and dry_run:
        # Check if any items exist
        exists = any(os.path.exists(os.path.join(ROLLUP_DIR, item["path"])) for item in CLEANUP_ITEMS)
        if not exists:
            print("  No runtime data to clean")

    # Step 3: Full cleanup (optional)
    if full_cleanup:
        print("\n📋 Step 3: Clean Deployment State (--full)")
        print("-" * 40)
        if dry_run:
            print("  ⚠️  This would remove deployment state - you'll need to redeploy!")
        cleaned2, size_freed2 = clean_items(FULL_CLEANUP_ITEMS, dry_run)
        cleaned += cleaned2
        size_freed += size_freed2
    else:
        print("\n📋 Step 3: Deployment State")
        print("-" * 40)
        print("  Keeping deployment state (add --full to remove)")

    # Summary
    print("\n" + "=" * 70)
    if dry_run:
        print("DRY RUN COMPLETE - No changes made")
        print("Run with --execute to perform cleanup")
        if full_cleanup:
            print("  python3 rollup/scripts/cleanup.py --execute --full")
        else:
            print("  python3 rollup/scripts/cleanup.py --execute")
    else:
        print(f"CLEANUP COMPLETE")
        print(f"  Processes killed: {killed}")
        print(f"  Items removed: {cleaned}")
        print(f"  Space freed: {size_freed / (1024*1024):.1f} MB")
    print("=" * 70 + "\n")


if __name__ == "__main__":
    main()

