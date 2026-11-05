package sdktc3

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestInterfaceSurfaceHasNoConsoleSessionEntry enumerates every exported symbol
// the SDK publishes and checks that none of them reads a console session: no
// console cookie, no kubeconfig, no console-session reader. The app interface
// itself is a closed list, so an app reaches exactly the reviewed entries.
func TestInterfaceSurfaceHasNoConsoleSessionEntry(t *testing.T) {
	root := moduleRoot(t)
	surface := exportedSurface(t, root)

	var inventory []string
	for pkg, names := range surface {
		for _, name := range names {
			inventory = append(inventory, pkg+"."+name)
		}
	}
	slices.Sort(inventory)
	t.Logf("SDK exported interface (%d entries):\n%s", len(inventory), strings.Join(inventory, "\n"))

	for _, required := range []string{
		"app.NewHost",
		"app.NewWindow",
		"app.Session.Allows",
		"app.Window.Accept",
		"manifest.ValidateAppManifest",
		"validate.Issue",
	} {
		if !slices.Contains(inventory, required) {
			t.Fatalf("%s is missing from the enumerated surface", required)
		}
	}

	// A console session is a console cookie or a kubeconfig (KOSD-C-75); no
	// exported name may be a reader of either.
	forbidden := []string{
		"consolesession", "consolecookie", "sessioncookie",
		"kubeconfig", "serviceaccount", "cookiejar",
	}
	for _, name := range inventory {
		lower := strings.ToLower(name)
		for _, bad := range forbidden {
			if strings.Contains(lower, bad) {
				t.Errorf("%s reads a console session", name)
			}
		}
	}

	// The app interface is closed: an app reaches exactly these entries.
	wantApp := []string{
		"Delivery",
		"Event",
		"Host",
		"Host.Receive",
		"HostConfig",
		"Init",
		"MessageType",
		"NewHost",
		"NewWindow",
		"Ready",
		"Session",
		"Session.Allows",
		"Source",
		"Token",
		"TypeInit",
		"TypeReady",
		"Window",
		"Window.Accept",
		"Window.Ready",
		"WindowConfig",
	}
	if got := surface["app"]; !slices.Equal(got, wantApp) {
		t.Fatalf("app interface surface = %v, want %v", got, wantApp)
	}

	// Nothing in the module reaches a cluster client or a cookie jar, and the
	// app interface depends on the SDK's declaration types only.
	imports := packageImports(t, root)
	for pkg, paths := range imports {
		for _, path := range paths {
			if strings.HasPrefix(path, "k8s.io/") || path == "net/http/cookiejar" {
				t.Errorf("%s imports %s", pkg, path)
			}
		}
	}
	allowed := map[string]bool{
		"github.com/tsic404/kos-sdk/manifest": true,
		"github.com/tsic404/kos-sdk/validate": true,
	}
	for _, path := range imports["app"] {
		if allowed[path] {
			continue
		}
		if strings.Contains(strings.SplitN(path, "/", 2)[0], ".") {
			t.Errorf("app imports %s; the app interface depends on the SDK's declaration types only", path)
		}
	}
}

// moduleRoot walks up from the test's working directory to the module root.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}

// exportedSurface maps each package of the module to its exported type, const,
// var, func and method names, sorted. Test packages are not part of the
// interface and are skipped.
func exportedSurface(t *testing.T, root string) map[string][]string {
	t.Helper()
	surface := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "tests" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		pkg := file.Name.Name
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if !decl.Name.IsExported() {
					continue
				}
				if decl.Recv == nil {
					surface[pkg] = append(surface[pkg], decl.Name.Name)
					continue
				}
				if len(decl.Recv.List) == 0 {
					continue
				}
				receiver := receiverName(decl.Recv.List[0].Type)
				if receiver == "" || !ast.IsExported(receiver) {
					continue
				}
				surface[pkg] = append(surface[pkg], receiver+"."+decl.Name.Name)
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() {
							surface[pkg] = append(surface[pkg], spec.Name.Name)
						}
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							if name.IsExported() {
								surface[pkg] = append(surface[pkg], name.Name)
							}
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module: %v", err)
	}
	for pkg, names := range surface {
		slices.Sort(names)
		surface[pkg] = slices.Compact(names)
	}
	return surface
}

// packageImports maps each package of the module to its non-test imports.
func packageImports(t *testing.T, root string) map[string][]string {
	t.Helper()
	imports := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "tests" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			imports[file.Name.Name] = append(imports[file.Name.Name], importPath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module imports: %v", err)
	}
	for pkg, paths := range imports {
		slices.Sort(paths)
		imports[pkg] = slices.Compact(paths)
	}
	return imports
}

func receiverName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		return receiverName(expr.X)
	case *ast.IndexExpr:
		return receiverName(expr.X)
	case *ast.IndexListExpr:
		return receiverName(expr.X)
	default:
		return ""
	}
}
