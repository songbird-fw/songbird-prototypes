// Package defaults builds the factory-default configuration committed on
// first boot (components.md: "No bootstrap mode"). It is plain v1alpha1
// YAML (factory/), plus the predefined palette and Services, so that every
// page of the PoC has something realistic to show.
package defaults

import (
	"embed"
	"fmt"
	"io/fs"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/predefined"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/validator"
)

//go:embed factory/*.yaml
var factory embed.FS

// Tree returns a fresh copy of the factory configuration.
func Tree() *m.Tree {
	t := &m.Tree{}
	for _, c := range predefined.Palette() {
		_ = t.Put(c)
	}
	for _, s := range predefined.Services() {
		_ = t.Put(s)
	}
	src := validator.Sources{}
	files, _ := fs.Glob(factory, "factory/*.yaml")
	for _, f := range files {
		b, _ := factory.ReadFile(f)
		if errs := validator.ParseDocs(t, f, b, src); len(errs) > 0 {
			panic(fmt.Sprintf("factory default: %v", errs))
		}
	}
	t.Sort()
	return t
}
