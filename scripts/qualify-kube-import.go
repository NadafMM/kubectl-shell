//go:build ignore

// qualify-kube-import adds an import of internal/kube (and optional extras)
// and rewrites exported identifier references via go/ast, skipping struct
// field names and already-qualified selectors.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "usage: %s <file.go> <local-pkg> <CommaExportedKubeSyms>\n", os.Args[0])
		os.Exit(2)
	}
	path, localPkg, exportList := os.Args[1], os.Args[2], os.Args[3]
	kubeExports := setOf(strings.Split(exportList, ","))

	// QUALIFY_EXTRA=alias=import/path:Sym+Sym
	extraPath := map[string]string{}
	extraExports := map[string]map[string]bool{}
	if raw := os.Getenv("QUALIFY_EXTRA"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if part == "" {
				continue
			}
			// alias=path:A+B
			eq := strings.IndexByte(part, '=')
			colon := strings.LastIndexByte(part, ':')
			if eq < 0 || colon < eq {
				fatal(fmt.Errorf("bad QUALIFY_EXTRA entry %q", part))
			}
			alias := part[:eq]
			ipath := part[eq+1 : colon]
			extraPath[alias] = ipath
			extraExports[alias] = setOf(strings.Split(part[colon+1:], "+"))
		}
	}

	src, err := os.ReadFile(path)
	if err != nil {
		fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		fatal(err)
	}

	local := map[string]bool{}
	for _, d := range file.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Recv == nil {
				local[x.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, s := range x.Specs {
				switch sp := s.(type) {
				case *ast.TypeSpec:
					local[sp.Name.Name] = true
				case *ast.ValueSpec:
					for _, n := range sp.Names {
						local[n.Name] = true
					}
				}
			}
		}
	}

	needsKube := false
	needsExtra := map[string]bool{}

	parents := map[ast.Node]ast.Node{}
	stack := []ast.Node{}
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) > 0 {
			parents[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})

	type repl struct {
		id  *ast.Ident
		pkg string
	}
	var repls []repl

	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if local[id.Name] || !ast.IsExported(id.Name) || id.Name == localPkg {
			return true
		}
		parent := parents[id]
		// Skip selector field (X.Sel)
		if sel, ok := parent.(*ast.SelectorExpr); ok && sel.Sel == id {
			return true
		}
		// Skip struct field names
		if f, ok := parent.(*ast.Field); ok {
			for _, name := range f.Names {
				if name == id {
					return true
				}
			}
		}
		// Skip composite lit keys and map keys in KeyValueExpr
		if kv, ok := parent.(*ast.KeyValueExpr); ok && kv.Key == id {
			return true
		}
		pkg := ""
		for alias, set := range extraExports {
			if set[id.Name] {
				pkg = alias
				break
			}
		}
		if pkg == "" && kubeExports[id.Name] {
			pkg = "kube"
		}
		if pkg == "" {
			return true
		}
		repls = append(repls, repl{id: id, pkg: pkg})
		return true
	})

	for _, r := range repls {
		parent := parents[r.id]
		sel := &ast.SelectorExpr{X: ast.NewIdent(r.pkg), Sel: ast.NewIdent(r.id.Name)}
		if !replaceInParent(parent, r.id, sel) {
			fatal(fmt.Errorf("could not replace %s in %T", r.id.Name, parent))
		}
		if r.pkg == "kube" {
			needsKube = true
		} else {
			needsExtra[r.pkg] = true
		}
	}

	if needsKube {
		addImport(file, "github.com/NadafMM/kubectl-shell/internal/kube")
	}
	for alias := range needsExtra {
		addImport(file, extraPath[alias])
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		fatal(err)
	}
}

func replaceInParent(parent ast.Node, old, neu ast.Expr) bool {
	switch p := parent.(type) {
	case *ast.Field:
		if p.Type == old {
			p.Type = neu
			return true
		}
	case *ast.ValueSpec:
		if p.Type == old {
			p.Type = neu
			return true
		}
		for i, v := range p.Values {
			if v == old {
				p.Values[i] = neu
				return true
			}
		}
	case *ast.TypeSpec:
		if p.Type == old {
			p.Type = neu
			return true
		}
	case *ast.CallExpr:
		if p.Fun == old {
			p.Fun = neu
			return true
		}
		for i, a := range p.Args {
			if a == old {
				p.Args[i] = neu
				return true
			}
		}
	case *ast.StarExpr:
		if p.X == old {
			p.X = neu
			return true
		}
	case *ast.ArrayType:
		if p.Elt == old {
			p.Elt = neu
			return true
		}
		if p.Len == old {
			p.Len = neu
			return true
		}
	case *ast.MapType:
		if p.Key == old {
			p.Key = neu
			return true
		}
		if p.Value == old {
			p.Value = neu
			return true
		}
	case *ast.ChanType:
		if p.Value == old {
			p.Value = neu
			return true
		}
	case *ast.Ellipsis:
		if p.Elt == old {
			p.Elt = neu
			return true
		}
	case *ast.UnaryExpr:
		if p.X == old {
			p.X = neu
			return true
		}
	case *ast.BinaryExpr:
		if p.X == old {
			p.X = neu
			return true
		}
		if p.Y == old {
			p.Y = neu
			return true
		}
	case *ast.ParenExpr:
		if p.X == old {
			p.X = neu
			return true
		}
	case *ast.IndexExpr:
		if p.X == old {
			p.X = neu
			return true
		}
		if p.Index == old {
			p.Index = neu
			return true
		}
	case *ast.SliceExpr:
		if p.X == old {
			p.X = neu
			return true
		}
		if p.Low == old {
			p.Low = neu
			return true
		}
		if p.High == old {
			p.High = neu
			return true
		}
		if p.Max == old {
			p.Max = neu
			return true
		}
	case *ast.SelectorExpr:
		if p.X == old {
			p.X = neu
			return true
		}
	case *ast.TypeAssertExpr:
		if p.X == old {
			p.X = neu
			return true
		}
		if p.Type == old {
			p.Type = neu
			return true
		}
	case *ast.CompositeLit:
		if p.Type == old {
			p.Type = neu
			return true
		}
		for i, e := range p.Elts {
			if e == old {
				p.Elts[i] = neu
				return true
			}
		}
	case *ast.KeyValueExpr:
		if p.Value == old {
			p.Value = neu
			return true
		}
	case *ast.ReturnStmt:
		for i, e := range p.Results {
			if e == old {
				p.Results[i] = neu
				return true
			}
		}
	case *ast.AssignStmt:
		for i, e := range p.Lhs {
			if e == old {
				p.Lhs[i] = neu
				return true
			}
		}
		for i, e := range p.Rhs {
			if e == old {
				p.Rhs[i] = neu
				return true
			}
		}
	case *ast.ExprStmt:
		if p.X == old {
			p.X = neu
			return true
		}
	case *ast.IfStmt:
		if p.Cond == old {
			p.Cond = neu
			return true
		}
	case *ast.ForStmt:
		if p.Cond == old {
			p.Cond = neu
			return true
		}
	case *ast.RangeStmt:
		if p.Key == old {
			p.Key = neu
			return true
		}
		if p.Value == old {
			p.Value = neu
			return true
		}
		if p.X == old {
			p.X = neu
			return true
		}
	case *ast.SwitchStmt:
		if p.Tag == old {
			p.Tag = neu
			return true
		}
	case *ast.CaseClause:
		for i, e := range p.List {
			if e == old {
				p.List[i] = neu
				return true
			}
		}
	case *ast.SendStmt:
		if p.Chan == old {
			p.Chan = neu
			return true
		}
		if p.Value == old {
			p.Value = neu
			return true
		}
	case *ast.IncDecStmt:
		if p.X == old {
			p.X = neu
			return true
		}
	case *ast.DeferStmt:
		if p.Call == old {
			if c, ok := neu.(*ast.CallExpr); ok {
				p.Call = c
				return true
			}
		}
	case *ast.GoStmt:
		if p.Call == old {
			if c, ok := neu.(*ast.CallExpr); ok {
				p.Call = c
				return true
			}
		}
	}
	return false
}

func addImport(file *ast.File, path string) {
	quoted := strconv.Quote(path)
	for _, i := range file.Imports {
		if i.Path != nil && i.Path.Value == quoted {
			return
		}
	}
	imp := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: quoted}}
	decl := &ast.GenDecl{Tok: token.IMPORT, Lparen: 1, Specs: []ast.Spec{imp}}
	// Merge into existing import decl if present
	for _, d := range file.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			g.Specs = append(g.Specs, imp)
			file.Imports = append(file.Imports, imp)
			return
		}
	}
	file.Decls = append([]ast.Decl{decl}, file.Decls...)
	file.Imports = append(file.Imports, imp)
}

func setOf(items []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range items {
		s = strings.TrimSpace(s)
		if s != "" {
			m[s] = true
		}
	}
	return m
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
