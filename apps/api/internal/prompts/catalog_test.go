package prompts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Broken review links make parts of a prompt effectively invisible. Verify the
// metadata against the checkout rather than asserting particular prose.
func TestCatalogAndAssemblySourcesExist(t *testing.T) {
	root := filepath.Join("..", "..")
	ids := map[string]bool{}
	check := func(path string) {
		t.Helper()
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Errorf("missing source %s: %v", path, err)
		}
	}
	for _, d := range Catalog() {
		if d.ID == "" || ids[d.ID] || strings.TrimSpace(d.Text) == "" {
			t.Errorf("invalid definition %s", d.ID)
		}
		ids[d.ID] = true
		check(d.Source)
		check(d.Consumer)
	}
	ids = map[string]bool{}
	for _, a := range Assemblies() {
		if a.ID == "" || ids[a.ID] || a.Purpose == "" || len(a.Builders) == 0 || len(a.Tests) == 0 {
			t.Errorf("incomplete assembly %s", a.ID)
		}
		ids[a.ID] = true
		for _, paths := range [][]string{a.Builders, a.Context, a.Contracts, a.Tests} {
			for _, p := range paths {
				check(p)
			}
		}
	}
}

// A new fixed prompt must not silently disappear from the review index.
func TestCatalogCoversExportedTextDefinitions(t *testing.T) {
	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, ".", func(f os.FileInfo) bool { return !strings.HasSuffix(f.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	definitions, listed := map[string]bool{}, map[string]bool{}
	for _, f := range pkgs["prompts"].Files {
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.CONST {
				for _, spec := range g.Specs {
					for _, n := range spec.(*ast.ValueSpec).Names {
						if ast.IsExported(n.Name) {
							definitions[n.Name] = true
						}
					}
				}
			}
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "Catalog" {
				ast.Inspect(fn, func(n ast.Node) bool {
					kv, ok := n.(*ast.KeyValueExpr)
					if !ok {
						return true
					}
					key, ok := kv.Key.(*ast.Ident)
					if !ok || key.Name != "Text" {
						return true
					}
					id, ok := kv.Value.(*ast.Ident)
					if !ok {
						t.Error("catalog text must refer to its compiled constant")
						return true
					}
					listed[id.Name] = true
					return true
				})
			}
		}
	}
	for name := range definitions {
		if !listed[name] {
			t.Errorf("unindexed prompt definition: %s", name)
		}
	}
	for name := range listed {
		if !definitions[name] {
			t.Errorf("catalog points outside fixed definitions: %s", name)
		}
	}
}
