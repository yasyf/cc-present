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
        "BLOCKED: the built-in Artifact tool publishes a static claude.ai page the "
        "session never hears back from. Present through a cc-present live board "
        "instead — invoke the cc-present:present skill and compose typed blocks "
        "(approvals, choices, diffs, cards) at a localhost URL where every click "
        "streams back into this session. When cc-present is unavailable, deliver "
        "the content as plain prose or AskUserQuestion in chat."
    ),
    block=True,
    tests={
        Input(tool="Artifact", tool_input={"file_path": "report.html", "favicon": "📊"}): Block(pattern="cc-present"),
        Input(tool="Artifact", tool_input={"action": "list"}): Block(),
        Input(tool="Read", file="report.html"): Allow(),
    },
)
