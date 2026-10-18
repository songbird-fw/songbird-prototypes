package api_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/api"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/audit"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/auth"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/configstore"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/defaults"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/events"
	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/reconciler"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/validator"
)

type client struct {
	t   *testing.T
	srv *httptest.Server
	c   *http.Client
}

type reply struct {
	Status     int
	Error      string            `json:"error"`
	Applied    bool              `json:"applied"`
	Validation []validator.Error `json:"validation"`
	Warnings   []validator.Error `json:"warnings"`
	Revision   string            `json:"revision"`
	Findings   []validator.Error `json:"findings"`
	Objects    map[string][]any  `json:"objects"`
}

func setup(t *testing.T) *client {
	t.Helper()
	dir := t.TempDir()
	store, err := configstore.Open(filepath.Join(dir, "config"), defaults.Tree)
	if err != nil {
		t.Fatal(err)
	}
	log, _ := audit.Open(filepath.Join(dir, "audit.log"))
	rec := reconciler.New(store, filepath.Join(dir, "state"))
	go rec.Run()
	policy := func() auth.Policy { tr, _, _ := store.Read("HEAD"); return auth.PolicyOf(tr) }
	s := &api.Server{Store: store, Rec: rec, Auth: auth.New(policy), Audit: log, Ev: events.NewCollector(),
		Web: fstest.MapFS{"index.html": {Data: []byte("ok")}}, ApplyWait: 10 * time.Second}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	c := &client{t, srv, &http.Client{Jar: jar}}
	if r := c.do("POST", "/api/v1/login", "application/json", `{"username":"admin","password":"songbird"}`, ""); r.Status != 200 {
		t.Fatalf("login: %+v", r)
	}
	return c
}

func (c *client) raw(method, path, ctype, body, rev string) (*http.Response, []byte) {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.srv.URL+path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "songbird")
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if rev != "" {
		req.Header.Set("If-Match", `"`+rev+`"`)
	}
	res, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, b
}

func (c *client) do(method, path, ctype, body, rev string) reply {
	res, b := c.raw(method, path, ctype, body, rev)
	var r reply
	_ = json.Unmarshal(b, &r)
	r.Status = res.StatusCode
	return r
}

func (c *client) rev() string { return c.do("GET", "/api/v1/config", "", "", "").Revision }

func has(l []validator.Error, rule string) *validator.Error {
	for i := range l {
		if l[i].Rule == rule {
			return &l[i]
		}
	}
	return nil
}

func TestSaveIsValidatedInTwoLayers(t *testing.T) {
	c := setup(t)
	if f := c.do("GET", "/api/v1/config", "", "", "").Findings; len(f) != 0 {
		t.Fatalf("factory default must be clean: %v", f)
	}
	rev := c.rev()

	// layer 1: the schema, with the line of the YAML document (R-020.3)
	yaml := "apiVersion: \"config.songbird-fw.com/v1alpha1\"\nkind: \"Address\"\nmetadata:\n  name: \"nas\"\nspec:\n  subnet: \"192.168.20.0/255.255.255.0\"\n"
	r := c.do("PUT", "/api/v1/objects/Address/nas", "application/yaml", yaml, rev)
	if e := has(r.Validation, "schema"); r.Status != 422 || e == nil || e.Line != 6 || e.Path != "spec.subnet" {
		t.Errorf("expected a schema error on line 6, got %+v", r)
	}

	// layer 2: DM-38, a Zone member referenced directly
	r = c.do("PUT", "/api/v1/objects/RuleSet/guest-to-untrusted", "application/json",
		`{"metadata":{"name":"guest-to-untrusted"},"spec":{"from":"guest","to":"wan","rules":[]}}`, rev)
	if e := has(r.Validation, "zone-member"); r.Status != 422 || e == nil || e.Constraint != "DM-38" || len(r.Validation) != 1 {
		t.Errorf("expected only DM-38, got %+v", r)
	}

	// an empty RuleSet is valid and stays "rules: []"
	r = c.do("PUT", "/api/v1/objects/RuleSet/guest-to-dmz?create=1", "application/json",
		`{"metadata":{"name":"guest-to-dmz"},"spec":{"from":"guest","to":"dmz-zone","rules":[]}}`, rev)
	if r.Status != 200 || !r.Applied {
		t.Fatalf("empty RuleSet: %+v", r)
	}
	if _, b := c.raw("GET", "/api/v1/objects/RuleSet/guest-to-dmz/yaml", "", "", ""); !strings.Contains(string(b), "rules: []") || !strings.Contains(string(b), `uuid: "`) {
		t.Errorf("unexpected file:\n%s", b)
	}

	// a warning never blocks a save (DM-40)
	rev = c.rev()
	r = c.do("PUT", "/api/v1/objects/RuleSet/untrusted-to-dmz", "application/json", `{"metadata":{"name":"untrusted-to-dmz"},"spec":{"from":"untrusted","to":"dmz-zone","rules":[
		{"name":"publish-web","source":["any"],"destination":["offsite-backup"],"service":["https"],"action":"accept","nat":{"type":"dnat","translateIpTo":"web-01"}}]}}`, rev)
	if r.Status != 200 || has(r.Warnings, "dnat-destination") == nil {
		t.Errorf("expected a saved change with a DM-40 warning, got %+v", r)
	}
}

func TestPredefinedObjectsAreProtected(t *testing.T) {
	c := setup(t)
	rev := c.rev()
	if r := c.do("DELETE", "/api/v1/objects/Color/green", "", "", rev); r.Status != 422 {
		t.Errorf("predefined Color deleted: %+v", r)
	}
	if r := c.do("DELETE", "/api/v1/objects/Service/https", "", "", rev); r.Status != 422 {
		t.Errorf("predefined Service deleted: %+v", r)
	}
	r := c.do("PUT", "/api/v1/objects/Color/green", "application/json", `{"metadata":{"name":"green"},"spec":{"hex":"#00FF00","predefined":true}}`, rev)
	if r.Status != 422 || has(r.Validation, "predefined-hex") == nil {
		t.Errorf("predefined hex changed: %+v", r)
	}
	r = c.do("PUT", "/api/v1/objects/Color/green", "application/json", `{"metadata":{"name":"green","comment":"Veeam"},"spec":{"hex":"#2E7D32","predefined":true}}`, rev)
	if r.Status != 200 {
		t.Errorf("the meaning of a predefined Color must be editable (R-055.1): %+v", r)
	}
}

func readCorpus(t *testing.T) string {
	files, _ := filepath.Glob("../../testdata/schema-examples/valid/*/*.yaml")
	var parts []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, string(b))
	}
	return strings.Join(parts, "\n---\n")
}

// The example of songbird-yaml-schema is refused as is (DM-38), accepted
// once untrusted-to-lan1 targets the Zone, and the export reads back clean.
func TestImportSchemaExampleAndExport(t *testing.T) {
	c := setup(t)
	corpus := readCorpus(t)
	r := c.do("POST", "/api/v1/import", "application/yaml", corpus, c.rev())
	if e := has(r.Validation, "zone-member"); r.Status != 422 || e == nil || e.File != "import.yaml" || e.Line == 0 {
		t.Fatalf("expected the DM-38 error in import.yaml, got %+v", r)
	}
	fixed := strings.Replace(corpus, "  to: \"lan1\"", "  to: \"internal\"", 1)
	r = c.do("POST", "/api/v1/import", "application/yaml", fixed, c.rev())
	if r.Status != 200 || !r.Applied || len(r.Warnings) != 5 {
		t.Fatalf("fixed example: %+v", r)
	}
	cfg := c.do("GET", "/api/v1/config", "", "", "")
	if n := len(cfg.Objects["Address"]); n != 10 || len(cfg.Objects["User"]) != 2 {
		t.Errorf("expected the 10 Addresses of the example and the 2 Users kept, got %d / %d", n, len(cfg.Objects["User"]))
	}

	res, b := c.raw("GET", "/api/v1/export", "", "", "")
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "application/gzip" {
		t.Fatalf("export: %d %s", res.StatusCode, b)
	}
	dir := t.TempDir()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(zr)
	files := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		p := filepath.Join(dir, filepath.FromSlash(h.Name))
		_ = os.MkdirAll(filepath.Dir(p), 0o750)
		data, _ := io.ReadAll(tr)
		_ = os.WriteFile(p, data, 0o600)
		files++
	}
	tree, _, errs := validator.LoadDir(dir)
	errs = append(errs, validator.Errors(validator.Validate(tree))...)
	if files != 45 || len(validator.Errors(errs)) > 0 {
		t.Errorf("export: %d files, errors %v", files, errs)
	}
	n := 0
	tree.Each(func(m.Doc) { n++ })
	if n != 45 {
		t.Errorf("export reads back %d objects, want 45", n)
	}
}
