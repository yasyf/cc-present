// Package examples embeds the reference block pack that `pack init` scaffolds
// from, alongside the sample block documents shipped in this directory.
package examples

import "embed"

// ExamplePack is the reference block pack under packs/example. The allowlist is
// explicit so the on-disk dist/, node_modules/, and bun.lock never leak in.
//
//go:embed packs/example/cc-present.toml
//go:embed packs/example/package.json
//go:embed packs/example/tsconfig.json
//go:embed packs/example/vite.config.ts
//go:embed packs/example/schema/callout.json
//go:embed packs/example/schema/rating.json
//go:embed packs/example/schema/rating.interaction.json
//go:embed packs/example/examples/callout.json
//go:embed packs/example/examples/rating.json
//go:embed packs/example/reference/blocks.md
//go:embed packs/example/scripts/smoke.ts
//go:embed packs/example/src/pack.tsx
//go:embed packs/example/src/Callout.tsx
//go:embed packs/example/src/Rating.tsx
//go:embed packs/example/src/host/global.d.ts
//go:embed packs/example/src/host/jsx-runtime.ts
//go:embed packs/example/src/host/present.ts
//go:embed packs/example/src/host/react.ts
var ExamplePack embed.FS

// ExamplePackRoot is the path within ExamplePack at which the reference pack's
// files are rooted.
const ExamplePackRoot = "packs/example"
