#!/usr/bin/env python3
"""
Rollup Process Manager - Check and manage rollup component processes

Usage:
    python3 check_processes.py          # Show status of all processes
    python3 check_processes.py kill     # Kill all running rollup processes
    python3 check_processes.py kill -f  # Force kill (SIGKILL)
"""

import subprocess
import sys
import signal

# Process names to check
PROCESSES = [
    {"name": "anvil", "description": "L1 Chain (Anvil)"},
    {"name": "geth", "description": "L2 Execution (op-geth)"},
    {"name": "op-node", "description": "L2 Consensus (op-node)"},
    {"name": "op-batcher", "description": "Batcher"},
    {"name": "op-proposer", "description": "Proposer"},
    {"name": "op-challenger", "description": "Challenger"},
]


def get_pids(process_name: str) -> list:
    """Get PIDs for a process by name."""
    try:
        result = subprocess.run(
            ["pgrep", "-f", process_name],
            capture_output=True,
            text=True
        )
        if result.returncode == 0:
            return [int(pid) for pid in result.stdout.strip().split("\n") if pid]
        return []
    except Exception:
        return []


def get_process_info(pid: int) -> str:
    """Get process command line info."""
    try:
        result = subprocess.run(
            ["ps", "-p", str(pid), "-o", "command="],
            capture_output=True,
            text=True
        )
        if result.returncode == 0:
            cmd = result.stdout.strip()
            # Truncate long commands
            return cmd[:60] + "..." if len(cmd) > 60 else cmd
        return "unknown"
    except Exception:
        return "unknown"


def check_status():
    """Check and display status of all rollup processes."""
    print("\n" + "=" * 70)
    print("                     ROLLUP PROCESS STATUS")
    print("=" * 70 + "\n")

    running_count = 0
    all_pids = []

    for proc in PROCESSES:
        pids = get_pids(proc["name"])

        # Filter out false positives (like this script or editors)
        filtered_pids = []
        for pid in pids:
            info = get_process_info(pid)
            # Skip if it's this script or an editor
            if "check_processes" in info or "vim" in info or "code" in info:
                continue
            filtered_pids.append((pid, info))

        if filtered_pids:
            running_count += 1
            status = "✅ Running"
            for pid, info in filtered_pids:
                all_pids.append(pid)
                print(f"{proc['description']:25} {status}")
                print(f"  └─ PID {pid}: {info}")
        else:
            print(f"{proc['description']:25} ❌ Not running")

    print("\n" + "-" * 70)
    print(f"Total: {running_count}/{len(PROCESSES)} components running")

    if all_pids:
        print(f"PIDs: {', '.join(map(str, all_pids))}")

    print("=" * 70 + "\n")

    return all_pids


def kill_processes(force: bool = False):
    """Kill all rollup processes."""
    print("\n" + "=" * 70)
    print("                     KILLING ROLLUP PROCESSES")
    print("=" * 70 + "\n")

    sig = signal.SIGKILL if force else signal.SIGTERM
    sig_name = "SIGKILL (force)" if force else "SIGTERM (graceful)"

    killed = 0

    for proc in PROCESSES:
        pids = get_pids(proc["name"])

        for pid in pids:
            info = get_process_info(pid)
            # Skip if it's this script or an editor
            if "check_processes" in info or "vim" in info or "code" in info:
                continue

            try:
                subprocess.run(["kill", f"-{sig.value}", str(pid)], check=True)
                print(f"✓ Killed {proc['description']} (PID {pid}) with {sig_name}")
                killed += 1
            except subprocess.CalledProcessError:
                print(f"✗ Failed to kill {proc['description']} (PID {pid})")
            except Exception as e:
                print(f"✗ Error killing {proc['description']}: {e}")

    print("\n" + "-" * 70)
    if killed > 0:
        print(f"Killed {killed} process(es)")
    else:
        print("No rollup processes were running")
    print("=" * 70 + "\n")


def main():
    if len(sys.argv) > 1 and sys.argv[1] == "kill":
        force = "-f" in sys.argv or "--force" in sys.argv
        kill_processes(force)
    else:
        check_status()


if __name__ == "__main__":
    main()

