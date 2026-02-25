#!/usr/bin/env python3
"""Sprint ledger — track sprint status.

Usage:
    python3 docs/sprints/ledger.py stats
    python3 docs/sprints/ledger.py start NNN
    python3 docs/sprints/ledger.py complete NNN
"""

import json
import os
import sys
from datetime import datetime, timezone

_HERE = os.path.dirname(os.path.abspath(__file__))
_LEDGER = os.path.join(_HERE, "ledger.json")


def _load() -> dict:
    if not os.path.exists(_LEDGER):
        return {"sprints": {}}
    with open(_LEDGER) as f:
        return json.load(f)


def _save(data: dict) -> None:
    with open(_LEDGER, "w") as f:
        json.dump(data, f, indent=2)
        f.write("\n")


def _now() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _discover_sprints() -> list[str]:
    """Return sorted list of SPRINT-NNN numbers found as .md files."""
    nums = []
    for name in os.listdir(_HERE):
        if name.startswith("SPRINT-") and name.endswith(".md"):
            try:
                n = int(name[len("SPRINT-"):-len(".md")])
                nums.append(n)
            except ValueError:
                pass
    return sorted(nums)


def cmd_stats() -> None:
    data = _load()
    sprints_data = data.get("sprints", {})
    nums = _discover_sprints()
    if not nums:
        print("No SPRINT-NNN.md files found in", _HERE)
        return

    col_w = 8
    print(f"{'Sprint':<{col_w}}  {'Status':<12}  {'Started':<22}  {'Completed':<22}")
    print("-" * 72)
    for n in nums:
        key = f"{n:03d}"
        entry = sprints_data.get(key, {})
        status = entry.get("status", "pending")
        started = entry.get("started_at", "-")
        completed = entry.get("completed_at", "-")
        print(f"SPRINT-{key}  {status:<12}  {started:<22}  {completed:<22}")


def cmd_start(num: str) -> None:
    key = f"{int(num):03d}"
    data = _load()
    entry = data["sprints"].setdefault(key, {})
    if entry.get("status") == "completed":
        print(f"Sprint {key} is already completed — cannot reopen.")
        sys.exit(1)
    entry["status"] = "in_progress"
    entry.setdefault("started_at", _now())
    _save(data)
    print(f"Sprint {key} marked in_progress.")


def cmd_complete(num: str) -> None:
    key = f"{int(num):03d}"
    data = _load()
    entry = data["sprints"].setdefault(key, {})
    entry["status"] = "completed"
    entry.setdefault("started_at", _now())
    entry["completed_at"] = _now()
    _save(data)
    print(f"Sprint {key} marked completed.")


def main() -> None:
    args = sys.argv[1:]
    if not args:
        print(__doc__)
        sys.exit(1)

    cmd = args[0]
    if cmd == "stats":
        cmd_stats()
    elif cmd == "start":
        if len(args) < 2:
            print("Usage: ledger.py start NNN")
            sys.exit(1)
        cmd_start(args[1])
    elif cmd == "complete":
        if len(args) < 2:
            print("Usage: ledger.py complete NNN")
            sys.exit(1)
        cmd_complete(args[1])
    else:
        print(f"Unknown command: {cmd}")
        print(__doc__)
        sys.exit(1)


if __name__ == "__main__":
    main()
