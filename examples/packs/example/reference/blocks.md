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
