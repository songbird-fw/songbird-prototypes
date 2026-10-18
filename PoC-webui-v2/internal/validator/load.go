package validator

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/schema"
)

// Source records where an object was read, to report errors against the
// original file and line rather than the canonical repository path.
type Source struct {
	File string
	Doc  m.RawDoc
}

type Sources map[string]Source // "Kind/name" -> source

// ParseDocs types the documents of one file. Documents that fail the schema
// are reported and skipped, like validate.py: the semantic layer only sees
// structurally valid objects.
func ParseDocs(t *m.Tree, file string, data []byte, src Sources) []Error {
	docs, err := m.SplitDocs(data)
	if err != nil {
		return []Error{{File: file, Reason: err.Error(), Rule: "yaml", Severity: SevError}}
	}
	var out []Error
	for _, d := range docs {
		where := Error{File: file, Line: d.Line, Rule: "schema", Constraint: "C-005", Severity: SevError}
		if issues := schema.Validate(d.Value); len(issues) > 0 {
			for _, is := range issues {
				e := where
				e.Path, e.Reason, e.Line = is.Path, is.Reason, m.LineOf(d.Node, is.Path)
				out = append(out, e)
			}
			continue
		}
		key := d.Kind + "/" + d.Name
		if prev, dup := src[key]; dup {
			e := where
			e.Path, e.Rule, e.Constraint = "metadata.name", "unique-name", "DM-01"
			e.Reason = fmt.Sprintf("%s is already defined in %s:%d", key, prev.File, prev.Doc.Line)
			out = append(out, e)
			continue
		}
		if _, err := t.PutYAML(d.Kind, d.Data); err != nil {
			e := where
			e.Reason = err.Error()
			out = append(out, e)
			continue
		}
		src[key] = Source{file, d}
	}
	return out
}

// LoadDir reads every *.yaml / *.yml file under dir, in any layout.
func LoadDir(dir string) (*m.Tree, Sources, []Error) {
	t, src := &m.Tree{}, Sources{}
	var errs []Error
	var files []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml")) {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			errs = append(errs, Error{File: f, Reason: err.Error(), Rule: "io", Severity: SevError})
			continue
		}
		errs = append(errs, ParseDocs(t, f, b, src)...)
	}
	t.Sort()
	return t, src, errs
}

// Relocate rewrites the canonical file of each finding to the source file
// and line it was read from.
func Relocate(errs []Error, t *m.Tree, src Sources) []Error {
	byFile := map[string]Source{}
	t.Each(func(o m.Doc) {
		kind, meta := o.Header()
		if s, ok := src[kind+"/"+meta.Name]; ok {
			byFile[m.FilePath(kind, meta.Name)] = s
		}
	})
	out := make([]Error, len(errs))
	for i, e := range errs {
		if s, ok := byFile[e.File]; ok {
			e.File = s.File
			e.Line = m.LineOf(s.Doc.Node, e.Path)
		}
		out[i] = e
	}
	return out
}
