"""Reuse the unchanged named-set revert engine with this ticket's own inputs."""

import json
from pathlib import Path

import red_3738_output_formats as engine

if __name__ == "__main__":
    root = Path(__file__).resolve().parents[2]
    spec = json.loads(Path(__file__).with_name("marker_line_3744.json").read_text())
    engine.ROOT, engine.TEST, engine.PREFIX = root, spec["tests"], spec["prefix"]
    engine.CONTROLS = {engine.PREFIX + name for name in spec["controls"]}
    engine.REVERTS = [
        (row["name"], row["path"], row["before"], row["after"], set(row["failed"]))
        for row in spec["reverts"]
    ]
    engine.main()
