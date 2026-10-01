from __future__ import annotations

from captain_hook import (
    Allow,
    Block,
    Event,
    Input,
    Tool,
    hook,
)

hook(
    Event.PreToolUse,
    only_if=[Tool("Artifact")],
    message=(
        "The built-in Artifact tool publishes a static page the session never hears back from. "
        "Invoke the `cc-present:present` skill to compose a live board instead."
    ),
    block=True,
    tests={
        Input(tool="Artifact", tool_input={"file_path": "report.html", "favicon": "📊"}): Block(pattern="cc-present"),
        Input(tool="Artifact", tool_input={"action": "list"}): Block(),
        Input(tool="Read", file="report.html"): Allow(),
    },
)
