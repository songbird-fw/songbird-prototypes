package validator_test

import (
	"path/filepath"
	"strings"
	"testing"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/validator"
)

// The corpus is a copy of songbird-fw/schema/examples (commit 99bfafe): it
// is the test corpus of `songbird validate` (schema README, R-020.2).
const corpus = "../../testdata/schema-examples"

func validateDir(t *testing.T, dir string) []validator.Error {
	t.Helper()
	tree, src, errs := validator.LoadDir(dir)
	errs = append(errs, validator.Relocate(validator.ValidateWith(tree, validator.Options{}), tree, src)...)
	return errs
}

// TestValidExample: the 57-object example passes every rule of validate.py
// and of the domain model. Since domain model 0.7 a member Interface may be
// used next to its Zone (DM-38), so untrusted-to-lan1 is valid.
func TestValidExample(t *testing.T) {
	for _, e := range validateDir(t, corpus+"/valid") {
		if e.Severity == validator.SevError {
			t.Error(e)
		} else {
			t.Log(e)
		}
	}
}

// expected rule id of each invalid case, as validate.py reports it (any of
// the listed ids is accepted).
var expected = map[string][]string{
	"01-netmask-notation":                  {"schema"},
	"02-address-two-forms":                 {"schema"},
	"03-range-inverted":                    {"range-order"},
	"04-tcp-no-port":                       {"schema"},
	"05-icmp-with-port":                    {"schema"},
	"06-port-out-of-range":                 {"schema"},
	"07-nested-group":                      {"wrong-kind"},
	"08-dangling-ref":                      {"dangling-ref"},
	"09-ruleset-wildcard-interface":        {"dangling-ref"},
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
	"39-interface-role-removed":            {"schema"},
	"40-predefined-any-modified":           {"predefined-immutable"},
	"41-any-protocol-user-service":         {"any-protocol"},
	"42-fqdn-in-enabled-rule":              {"unsupported-address-kind"},
	"43-system-old-serveto":                {"schema"},
	"45-service-sourceport":                {"schema"},
	"46-dhcp-range-outside-subnet":         {"dhcp-range-subnet"},
	"47-dhcp-advertises-silent-dns":        {"dhcp-advertises-silent-service"},
	"48-dhcp-on-dhcp-client-interface":     {"dhcp-needs-static"},
	"49-dhcp-unknown-interface":            {"dangling-ref"},
}

func TestInvalidExamples(t *testing.T) {
	dirs, _ := filepath.Glob(corpus + "/invalid/*")
	if len(dirs) != len(expected) {
		t.Fatalf("corpus has %d cases, table has %d", len(dirs), len(expected))
	}
	for _, dir := range dirs {
		name := filepath.Base(dir)
		want, ok := expected[name]
		if !ok {
			t.Errorf("%s: not in the table", name)
			continue
		}
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
	if n != 57 {
		t.Errorf("expected 57 objects, got %d", n)
	}
}
