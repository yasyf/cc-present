package examples

import (
	"io/fs"
	"reflect"
	"sort"
	"testing"
)

func TestExamplePackEmbedComplete(t *testing.T) {
	want := []string{
		"packs/example/cc-present.toml",
		"packs/example/examples/callout.json",
		"packs/example/examples/rating.json",
		"packs/example/package.json",
		"packs/example/reference/blocks.md",
		"packs/example/schema/callout.json",
		"packs/example/schema/rating.interaction.json",
		"packs/example/schema/rating.json",
		"packs/example/scripts/smoke.ts",
		"packs/example/src/Callout.tsx",
		"packs/example/src/Rating.tsx",
		"packs/example/src/host/global.d.ts",
		"packs/example/src/host/jsx-runtime.ts",
		"packs/example/src/host/present.ts",
		"packs/example/src/host/react.ts",
		"packs/example/src/pack.tsx",
		"packs/example/tsconfig.json",
		"packs/example/vite.config.ts",
	}

	var got []string
	err := fs.WalkDir(ExamplePack, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			got = append(got, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embed FS: %v", err)
	}
	sort.Strings(got)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("embedded files = %v, want %v", got, want)
	}
}
