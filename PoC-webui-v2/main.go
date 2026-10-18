// Command songbird-webui is a proof of concept of the Songbird Web UI and of
// the Control Plane slice behind it (API Server, Config Manager, Validator,
// Reconciler, Observer, Audit Log), built as one Go binary (C-004).
//
//	songbird-webui [-listen addr] [-data dir]   run the appliance UI
//	songbird-webui validate [-appliance] <dir>  validate a configuration tree (R-052)
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/api"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/audit"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/auth"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/configstore"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/defaults"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/events"
	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/observer"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/reconciler"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/validator"
)

//go:embed web
var webFS embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == "validate" {
		os.Exit(validateCmd(os.Args[2:]))
	}
	listen := flag.String("listen", "127.0.0.1:8080", "address to listen on")
	data := flag.String("data", "./data", "data directory (config repository, runtime state, audit log)")
	cert := flag.String("tls-cert", "", "TLS certificate (enables HTTPS, R-038)")
	key := flag.String("tls-key", "", "TLS key")
	flag.Parse()

	if err := os.MkdirAll(*data, 0o750); err != nil {
		log.Fatal(err)
	}
	store, err := configstore.Open(filepath.Join(*data, "config"), defaults.Tree)
	if err != nil {
		log.Fatalf("config repository: %v", err)
	}
	auditLog, err := audit.Open(filepath.Join(*data, "audit.log"))
	if err != nil {
		log.Fatalf("audit log: %v", err)
	}

	rec := reconciler.New(store, filepath.Join(*data, "state"))
	rec.OnResult = func(a reconciler.Attempt) {
		if a.Result == "no-op" {
			return
		}
		detail := a.Result
		if a.Error != nil {
			detail += ": " + a.Error.Message
		}
		auditLog.Record("reconciler", "local", "apply."+a.Result, "commit "+a.Commit[:min(8, len(a.Commit))], detail, nil, nil)
	}
	go rec.Run()

	ev := events.NewCollector()
	go (&events.Simulator{C: ev, Applied: rec.AppliedTree}).Run()
	obs := observer.New(rec, ev)
	go obs.Run()

	authPolicy := func() auth.Policy {
		t, _, err := store.Read("HEAD")
		if err != nil {
			return auth.PolicyOf(&m.Tree{})
		}
		return auth.PolicyOf(t)
	}
	web, _ := fs.Sub(webFS, "web")
	srv := &api.Server{Store: store, Rec: rec, Auth: auth.New(authPolicy), Audit: auditLog, Ev: ev, Obs: obs, Web: web, ApplyWait: 5 * time.Second}

	hs := &http.Server{Addr: *listen, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	if *cert != "" {
		log.Printf("Songbird Web UI PoC on https://%s (admin/songbird, auditor/auditor)", *listen)
		log.Fatal(hs.ListenAndServeTLS(*cert, *key))
	}
	log.Printf("Songbird Web UI PoC on http://%s (admin/songbird, auditor/auditor)", *listen)
	log.Fatal(hs.ListenAndServe())
}

// validateCmd is the Go prototype of `songbird validate <dir>` (R-052): the
// same two layers and rule ids as songbird-yaml-schema/validate.py, plus the
// domain-model constraints. Exit 0 = valid, 1 = errors, 2 = usage.
func validateCmd(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	appliance := fs.Bool("appliance", false, "also require what a running appliance has: System, the predefined palette, an admin User")
	quiet := fs.Bool("q", false, "do not print warnings")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: songbird-webui validate [-appliance] [-q] <dir>")
		return 2
	}
	tree, src, errs := validator.LoadDir(fs.Arg(0))
	errs = append(errs, validator.Relocate(validator.ValidateWith(tree, validator.Options{Appliance: *appliance}), tree, src)...)
	objects := 0
	tree.Each(func(m.Doc) { objects++ })
	nerr, nwarn := 0, 0
	for _, e := range errs {
		if e.Severity == validator.SevError {
			nerr++
		} else {
			nwarn++
			if *quiet {
				continue
			}
		}
		fmt.Println(e)
	}
	fmt.Printf("%d objects, %d errors, %d warnings\n", objects, nerr, nwarn)
	if nerr > 0 {
		return 1
	}
	return 0
}
