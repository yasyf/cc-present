from __future__ import annotations

from captain_hook import (
    Allow,
    BaseHookEvent,
    Event,
    HookResult,
    Input,
    Tool,
    Warn,
    on,
)


@on(
    Event.PreToolUse,
    only_if=[Tool("Artifact")],
    tests={
        Input(tool="Artifact", tool_input={"file_path": "report.html", "icon": "chart"}): Warn(pattern="cc-present"),
        Input(tool="Artifact", tool_input={"action": "open", "url": "https://claude.ai/artifact/x"}): Warn(),
        Input(tool="Read", file="report.html"): Allow(),
    },
)
def suggest_live_board(evt: BaseHookEvent) -> HookResult:
    return evt.context(
        "The built-in Artifact tool publishes a static page the session never hears back from. "
        "Consider the `cc-present:present` skill for a live board whose clicks stream back here."
    )
