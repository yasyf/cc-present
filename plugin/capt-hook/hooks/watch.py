from __future__ import annotations

from itertools import pairwise
from pathlib import PurePosixPath

from captain_hook import Allow, Annotated, BaseHookEvent, Block, Cmd, CustomCondition, Event, Input, Tool, hook


class PipesWatch(CustomCondition):
    def check(self, evt: BaseHookEvent) -> bool:
        if (cmd := Cmd.parse(evt.input.raw.get("command", ""))) is None:
            return False
        stages = [call for call in cmd.calls() if call.occurrence.nesting == 0]
        return any(
            PurePosixPath(stage.name).name == "cc-present"
            and stage.args[:1] == ("watch",)
            and after.occurrence.prev_op == "|"
            for stage, after in pairwise(stages)
        )


hook(
    Event.PreToolUse,
    only_if=[Tool("Monitor", "Bash"), PipesWatch()],
    skip_if=[Annotated("raw")],
    message=(
        "A pipe after `cc-present watch` holds each board event until the next one arrives. "
        'Run `cc-present watch --session "$CLAUDE_CODE_SESSION_ID"` bare; `CAPT_HOOK_CCX_RAW=1` skips this check.'
    ),
    block=True,
    tests={
        Input(
            tool="Monitor",
            command="cc-present watch --session \"$CLAUDE_CODE_SESSION_ID\" 2>&1 | grep --line-buffered -v 'dropped pack'",
        ): Block(pattern="bare"),
        Input(
            tool="Bash",
            command='"${CLAUDE_PLUGIN_ROOT}/bin/cc-present" watch --session s --once | jq .type',
        ): Block(pattern="bare"),
        Input(tool="Monitor", command='cc-present watch --session "$CLAUDE_CODE_SESSION_ID"'): Allow(),
        Input(tool="Monitor", command='cc-present watch --session "$CLAUDE_CODE_SESSION_ID" 2>/dev/null'): Allow(),
        Input(tool="Bash", command="cc-present outcomes --new | jq ."): Allow(),
        Input(tool="Bash", command="cc-present watch --session s --once | grep -v x  # ccx:raw"): Allow(),
    },
)
