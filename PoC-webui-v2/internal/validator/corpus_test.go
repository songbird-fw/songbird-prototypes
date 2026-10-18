package validator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/validator"
)

// The corpus is a copy of songbird-yaml-schema/examples (commit 55e63ac):
// it is the test corpus of `songbird validate` (schema README, R-020.2).
const corpus = "../../testdata/schema-examples"

func validateDir(t *testing.T, dir string) []validator.Error {
	t.Helper()
	tree, src, errs := validator.LoadDir(dir)
	errs = append(errs, validator.Relocate(validator.ValidateWith(tree, validator.Options{}), tree, src)...)
	return errs
}

// TestValidExample: the 43-object example of the schema passes every rule
// of validate.py, but breaks DM-38 / R-046.1 of the domain model: the
// RuleSet untrusted-to-lan1 references lan1, a member of Zone internal.
// This is a finding of the PoC, reported in the README.
func TestValidExample(t *testing.T) {
	var errs []validator.Error
	for _, e := range validateDir(t, corpus+"/valid") {
		if e.Severity == validator.SevError {
			errs = append(errs, e)
		} else {
			t.Log(e)
		}
	}
	if len(errs) != 1 || errs[0].Rule != "zone-member" || errs[0].Constraint != "DM-38" ||
		!strings.HasSuffix(errs[0].File, "firewall/rules.yaml") || errs[0].Path != "spec.to" || errs[0].Line == 0 {
		t.Fatalf("expected exactly the DM-38 error on untrusted-to-lan1, got %v", errs)
	}
}

// expected rule id of each invalid file (any of the listed ids is accepted).
var expected = map[string][]string{
	"01-netmask-notation":                  {"schema"},
	"02-address-two-forms":                 {"schema"},
	"03-range-inverted":                    {"range-order"},
	"04-tcp-no-port":                       {"schema"},
	"05-icmp-with-port":                    {"schema"},
	"06-port-out-of-range":                 {"schema"},
	"07-group-cycle":                       {"group-cycle"},
	"08-dangling-ref":                      {"dangling-ref"},
	"09-ruleset-any-interface":             {"schema"},
	"10-drop-with-nat":                     {"schema"},
	"11-duplicate-pair":                    {"duplicate-pair"},
	"12-vlan-without-parent":               {"schema"},
	"13-vlan-id-4095":                      {"schema"},
	"14-zone-overlap":                      {"zone-overlap"},
	"15-dnat-without-translate-ip":         {"schema"},
	"16-dnat-any-destination":              {"dnat-any-destination"},
	"17-unknown-field":                     {"schema"},
	"18-reserved-name":                     {"schema"},
	"19-color-literal-hex":                 {"schema"},
	"20-dup-name-across-ns":                {"unique-name"},
	"21-color-dangling":                    {"dangling-ref"},
	"22-color-bad-hex":                     {"schema"},
	"23-color-duplicate-hex":               {"duplicate-hex"},
	"24-color-repaint-predefined":          {"predefined-hex"},
	"25-color-shadow-predefined":           {"predefined-shadow"},
	"26-color-with-color":                  {"schema"},
	"27-color-meaning-removed":             {"schema"},
	"28-snat-with-translate-service":       {"schema"},
	"29-dnat-translate-to-subnet":          {"dnat-single-host"},
	"30-dnat-service-shape":                {"dnat-service-shape"},
	"31-dnat-protocol-mismatch":            {"dnat-protocol-mismatch"},
	"32-system-wrong-name":                 {"schema"},
	"33-system-duplicate":                  {"unique-name", "system-singleton"},
	"34-system-plaintext-secret":           {"schema"},
	"35-system-dns64-without-nat64":        {"dns64-without-nat64"},
	"36-system-bad-duration":               {"schema"},
	"37-system-git-enabled-without-remote": {"schema"},
	"38-interface-bad-allowaccess":         {"schema"},
}

func TestInvalidExamples(t *testing.T) {
	files, _ := filepath.Glob(corpus + "/invalid/*.yaml")
	if len(files) != len(expected) {
		t.Fatalf("corpus has %d files, table has %d", len(files), len(expected))
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".yaml")
		want, ok := expected[name]
		if !ok {
			t.Errorf("%s: not in the table", name)
			continue
		}
		dir := t.TempDir() // one file per tree: each example stands alone
		b, _ := os.ReadFile(f)
		_ = os.WriteFile(filepath.Join(dir, "x.yaml"), b, 0o600)
		found := false
		var got []string
		for _, e := range validateDir(t, dir) {
			got = append(got, e.Rule)
			for _, w := range want {
				if e.Rule == w && e.Severity == validator.SevError && e.File != "" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s: expected %v, got %v", name, want, got)
		}
	}
}

// TestCanonicalRoundTrip: every example object, written in the canonical
// form and read back, is unchanged and still valid against the schema.
func TestCanonicalRoundTrip(t *testing.T) {
	tree, _, errs := validator.LoadDir(corpus + "/valid")
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	n := 0
	tree.Each(func(o m.Doc) {
		n++
		b, err := m.MarshalObject(o)
		if err != nil {
			t.Fatal(err)
		}
		kind, meta := o.Header()
		back := &m.Tree{}
		if _, err := back.PutYAML(kind, b); err != nil {
			t.Fatalf("%s/%s: %v\n%s", kind, meta.Name, err, b)
		}
		o2, _ := back.Get(kind, meta.Name)
		b2, _ := m.MarshalObject(o2)
		if string(b) != string(b2) {
			t.Errorf("%s/%s changed on round trip:\n%s\n---\n%s", kind, meta.Name, b, b2)
		}
		if strings.Contains(string(b), "kind: Address") || !strings.Contains(string(b), `kind: "`) {
			t.Errorf("%s/%s: string values must be double-quoted:\n%s", kind, meta.Name, b)
		}
	})
	if n != 43 {
		t.Errorf("expected 43 objects, got %d", n)
	}
}
