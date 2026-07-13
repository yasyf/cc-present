# example pack blocks

Two block types under the `example` pack. Reference them by dotted wire type
inside any `Doc.blocks` array or a card's `children`.

## example.callout

A toned admonition that renders Markdown. Content only, no interaction.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `id` | string | yes | Unique block id. |
| `type` | `"example.callout"` | yes | The dotted wire type. |
| `tone` | `"info"` \| `"warn"` \| `"success"` | no | Visual tone; defaults to `info`. |
| `md` | string | yes | Markdown body, rendered as sanitized HTML. |

```json
{
  "id": "note",
  "type": "example.callout",
  "tone": "warn",
  "md": "Runs **before** the deploy — double-check the target."
}
```

## example.rating

An N-point rating. The human picks a value; the pick streams back as a
`pack.interaction` with payload `{"value": <n>}`.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `id` | string | yes | Unique block id. |
| `type` | `"example.rating"` | yes | The dotted wire type. |
| `label` | string | yes | Prompt shown above the buttons. |
| `scale` | integer, 2 to 10 | no | Number of points; defaults to `5`. |

```json
{
  "id": "confidence",
  "type": "example.rating",
  "label": "How confident are you in this plan?",
  "scale": 5
}
```

The interaction payload is `{"value": <n>}`, where `n` is the chosen point, `1`
through `scale`.

## example.survey

A two-step wizard built on the hostApi 2 surface. `ui.usePackState` holds the
step index and per-step drafts, which survive remounts and board↔focus
navigation. `ui.tokens` styles every element, and the last step raises a
`ui.toast` and submits one merged payload.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `id` | string | yes | Unique block id. |
| `type` | `"example.survey"` | yes | The dotted wire type. |
| `title` | string | yes | Heading shown above the wizard. |
| `steps` | array of 2 | yes | Each step is `{ "prompt": string, "placeholder"?: string }`. |

```json
{
  "id": "pulse",
  "type": "example.survey",
  "title": "Quick pulse check",
  "steps": [
    { "prompt": "In one line, how did this go?", "placeholder": "Summary" },
    { "prompt": "Anything to expand on?", "placeholder": "Detail (optional)" }
  ]
}
```

The interaction payload is one merged object with optional `summary` and `detail`
strings. The component submits `{...(value ?? {}), summary, detail}`. In this
merge idiom, each control spreads the prior value and overwrites the fields it
owns, so one interaction schema backs a multi-control block.
