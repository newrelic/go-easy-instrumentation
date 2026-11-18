package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/usage"
	"golang.org/x/tools/go/packages"
)

// reportUsage sends anonymous usage data for a finished run.
//
// It runs after the user already has their diff and their completion message, so
// a slow endpoint delays nothing they are waiting on. Reporting can never change
// the outcome of a run: every failure, including a panic, is swallowed and shown
// only under --debug.
func reportUsage(data usage.RunData) {
	if usageOptOut || !usage.Enabled() {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			debugPrintf("usage reporting panicked: %v", r)
		}
	}()

	runID, err := usage.Send(data)
	if err != nil {
		debugPrintf("usage reporting failed (run %s): %v", runID, err)
		return
	}

	debugPrintf("usage reported under run ID %s", runID)
}

// newRunData returns run data pre-filled with everything knowable before the
// pipeline starts.
//
// mode is the mode the caller runs in, and is overridden when the tool was
// driven interactively.
func newRunData(mode string) usage.RunData {
	if interactiveMode {
		mode = usage.ModeInteractive
	}

	return usage.RunData{Mode: mode}
}

// directImports returns every import path imported directly by the
// application's non-test packages, sorted and deduplicated.
//
// These are the application's own imports, which is what "uses a library"
// means. Transitive dependencies are left out, so that a library pulled in by
// gin is not reported as used by the application. Test packages are left out
// too: packages are loaded with tests, and a library used only by tests has no
// application code for this tool to instrument. The paths are filtered through
// a fixed list before anything is sent.
func directImports(pkgs []*decorator.Package) []string {
	seen := map[string]bool{}
	paths := []string{}

	for _, pkg := range pkgs {
		if pkg == nil || pkg.Package == nil || isTestPackage(pkg.Package) {
			continue
		}
		// pkg.Package.Imports, not pkg.Imports: decorator.Package declares its
		// own Imports field, which shadows the embedded one.
		for path := range pkg.Package.Imports {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}

	sort.Strings(paths)

	return paths
}

// isTestPackage reports whether pkg exists only because packages were loaded
// with tests: a test variant such as "app [app.test]", an external "app_test"
// package, or the generated "app.test" main. The plain package is always loaded
// alongside, so skipping these loses no application imports.
func isTestPackage(pkg *packages.Package) bool {
	return pkg.ForTest != "" || strings.HasSuffix(pkg.ID, ".test")
}

// loadErrorKinds returns the kinds of load error found anywhere in the loaded
// package graph, such as "TypeError", deduplicated and sorted.
func loadErrorKinds(pkgs []*decorator.Package) []string {
	roots := make([]*packages.Package, 0, len(pkgs))
	for _, pkg := range pkgs {
		if pkg != nil && pkg.Package != nil {
			roots = append(roots, pkg.Package)
		}
	}

	seen := map[string]bool{}
	kinds := []string{}

	packages.Visit(roots, nil, func(pkg *packages.Package) {
		for _, err := range pkg.Errors {
			kind := errorKindName(err.Kind)
			if !seen[kind] {
				seen[kind] = true
				kinds = append(kinds, kind)
			}
		}
	})

	sort.Strings(kinds)

	return kinds
}

// errorKindName names a package load error kind. ErrorKind has no String
// method, and its numeric values are not meant to be stable.
func errorKindName(kind packages.ErrorKind) string {
	switch kind {
	case packages.ListError:
		return "ListError"
	case packages.ParseError:
		return "ParseError"
	case packages.TypeError:
		return "TypeError"
	default:
		return "UnknownError"
	}
}

// debugPrintf prints a message only when --debug is set.
func debugPrintf(format string, args ...any) {
	if !debug {
		return
	}

	fmt.Printf(format+"\n", args...)
}
