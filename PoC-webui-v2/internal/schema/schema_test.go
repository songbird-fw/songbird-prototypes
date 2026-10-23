package schema_test

import (
	"os"
	"path/filepath"
	"testing"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/schema"
)

const corpus = "../../testdata/schema-examples"

func docs(t *testing.T, file string) []m.RawDoc {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	d, err := m.SplitDocs(b)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return d
}

func yamlFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(p) == ".yaml" {
			files = append(files, p)
		}
		return nil
	})
	return files
}

func TestValidExamplesPassTheSchema(t *testing.T) {
	files := yamlFiles(t, corpus+"/valid")
	if len(files) == 0 {
		t.Fatal("no corpus")
	}
	n := 0
	for _, f := range files {
		for _, d := range docs(t, f) {
			n++
			if is := schema.Validate(d.Value); len(is) > 0 {
				t.Errorf("%s:%d %s/%s: %v", f, d.Line, d.Kind, d.Name, is)
			}
		}
	}
	if n != 57 {
		t.Errorf("expected the 57 objects of the schema README, got %d", n)
	}
}

// The invalid corpus cases whose error is structural, as validate.py
// reports them (the others are semantic and are covered by the validator
// tests). Each case is a directory.
var structural = []string{
	"01-netmask-notation", "02-address-two-forms", "04-tcp-no-port", "05-icmp-with-port",
	"06-port-out-of-range", "10-drop-with-nat", "12-vlan-without-parent",
	"13-vlan-id-4095", "15-dnat-without-translate-ip", "17-unknown-field", "18-reserved-name",
	"19-color-literal-hex", "22-color-bad-hex", "26-color-with-color", "27-color-meaning-removed",
	"28-snat-with-translate-service", "32-system-wrong-name", "34-system-plaintext-secret",
	"36-system-bad-duration", "37-system-git-enabled-without-remote", "38-interface-bad-allowaccess",
	"39-interface-role-removed", "43-system-old-serveto", "45-service-sourceport",
}

func TestStructuralInvalidExamplesFail(t *testing.T) {
	for _, name := range structural {
		failed := false
		var all []m.RawDoc
		for _, f := range yamlFiles(t, corpus+"/invalid/"+name) {
			all = append(all, docs(t, f)...)
		}
		for _, d := range all {
			if is := schema.Validate(d.Value); len(is) > 0 {
				failed = true
				t.Logf("%s: %s/%s %v", name, d.Kind, d.Name, is)
			}
		}
		if !failed {
			t.Errorf("%s: expected a schema error", name)
		}
	}
}

func TestExtensionKinds(t *testing.T) {
	ok := map[string]any{"apiVersion": m.APIVersion, "kind": "StaticRoute", "metadata": map[string]any{"name": "default"},
		"spec": map[string]any{"destination": "0.0.0.0/0", "gateway": "203.0.113.1", "interface": "wan"}}
	if is := schema.Validate(ok); len(is) > 0 {
		t.Errorf("valid StaticRoute rejected: %v", is)
	}
	bad := map[string]any{"apiVersion": m.APIVersion, "kind": "StaticRoute", "metadata": map[string]any{"name": "default"},
		"spec": map[string]any{"destination": "0.0.0.0/0", "gateway": "203.0.113.1", "interface": "wan", "distance": 0}}
	if is := schema.Validate(bad); len(is) != 1 || is[0].Path != "spec.distance" {
		t.Errorf("expected spec.distance error, got %v", is)
	}
	unknown := map[string]any{"apiVersion": m.APIVersion, "kind": "VirtualIP", "metadata": map[string]any{"name": "x"}, "spec": map[string]any{}}
	if is := schema.Validate(unknown); len(is) == 0 {
		t.Error("VirtualIP is not a v1alpha1 kind and must be rejected")
	}
}
