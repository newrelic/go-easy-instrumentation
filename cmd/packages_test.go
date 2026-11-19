package cmd

import (
	"fmt"
	"go/types"
	"path/filepath"
	"testing"

	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/parser"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

// pkg builds a decorator.Package wrapping the given load state. Imports are
// attached so the dependency walk can be exercised.
func pkg(id string, illTyped bool, errs []string, imports ...*packages.Package) *decorator.Package {
	loadErrors := make([]packages.Error, 0, len(errs))
	for _, e := range errs {
		loadErrors = append(loadErrors, packages.Error{Msg: e, Kind: packages.UnknownError})
	}

	importMap := map[string]*packages.Package{}
	for _, imp := range imports {
		importMap[imp.ID] = imp
	}

	return &decorator.Package{
		Package: &packages.Package{
			ID:       id,
			Errors:   loadErrors,
			IllTyped: illTyped,
			Imports:  importMap,
		},
	}
}

func TestPackageLoadIssues(t *testing.T) {
	tests := []struct {
		name string
		pkgs []*decorator.Package
		want []string // issue IDs, in order
	}{
		{
			name: "no packages",
			pkgs: []*decorator.Package{},
			want: []string{},
		},
		{
			name: "clean package has no issues",
			pkgs: []*decorator.Package{pkg("app", false, nil)},
			want: []string{},
		},
		{
			name: "load error is reported",
			pkgs: []*decorator.Package{pkg("app", false, []string{"cannot find module"})},
			want: []string{"app"},
		},
		{
			name: "ill-typed package is reported even with no errors",
			pkgs: []*decorator.Package{pkg("app", true, nil)},
			want: []string{"app"},
		},
		{
			// The case that matters for this tool: the root builds fine but a
			// dependency did not load, so type-driven matching through it fails.
			name: "broken dependency is reported",
			pkgs: []*decorator.Package{
				pkg("app", false, nil, &packages.Package{
					ID:     "net/http",
					Errors: []packages.Error{{Msg: "export data error"}},
				}),
			},
			want: []string{"net/http"},
		},
		{
			name: "results are sorted by ID",
			pkgs: []*decorator.Package{
				pkg("zebra", true, nil),
				pkg("apple", true, nil),
			},
			want: []string{"apple", "zebra"},
		},
		{
			name: "nil packages are skipped",
			pkgs: []*decorator.Package{nil, pkg("app", true, nil)},
			want: []string{"app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := packageLoadIssues(tt.pkgs)

			got := make([]string, 0, len(issues))
			for _, issue := range issues {
				got = append(got, issue.ID)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPackageLoadIssuesDeduplicatesSharedDependencies(t *testing.T) {
	broken := &packages.Package{ID: "net/http", Errors: []packages.Error{{Msg: "export data error"}}}

	issues := packageLoadIssues([]*decorator.Package{
		pkg("a", false, nil, broken),
		pkg("b", false, nil, broken),
	})

	assert.Len(t, issues, 1, "a dependency shared by two packages should be reported once")
}

func TestCheckPackageLoad(t *testing.T) {
	t.Run("clean load produces no warning and no error", func(t *testing.T) {
		warning, err := checkPackageLoad([]*decorator.Package{pkg("app", false, nil)})

		assert.NoError(t, err)
		assert.Empty(t, warning)
	})

	t.Run("partial failure warns and continues", func(t *testing.T) {
		warning, err := checkPackageLoad([]*decorator.Package{
			pkg("good", false, nil),
			pkg("bad", true, []string{"undefined: Foo"}),
		})

		assert.NoError(t, err, "a usable root means instrumentation should proceed")
		assert.Contains(t, warning, "WARNING")
		assert.Contains(t, warning, "bad")
		assert.Contains(t, warning, "undefined: Foo")
		assert.Contains(t, warning, "may be incomplete")
		// Points the reader at today's most likely causes.
		assert.Contains(t, warning, "golang.org/x/tools")
	})

	t.Run("total failure is an error", func(t *testing.T) {
		_, err := checkPackageLoad([]*decorator.Package{
			pkg("a", true, []string{"broken"}),
			pkg("b", true, []string{"broken"}),
		})

		assert.ErrorContains(t, err, "no packages could be loaded")
	})

	t.Run("empty load is not an error", func(t *testing.T) {
		// Matching no packages is a pattern problem, not a load failure, and is
		// left to the existing behavior.
		warning, err := checkPackageLoad([]*decorator.Package{})

		assert.NoError(t, err)
		assert.Empty(t, warning)
	})
}

func TestIndentedListTruncates(t *testing.T) {
	issues := make([]packageLoadIssue, 0, maxReportedPackages+3)
	for i := 0; i < maxReportedPackages+3; i++ {
		issues = append(issues, packageLoadIssue{ID: fmt.Sprintf("pkg%d", i), IllTyped: true})
	}

	got := indentedList(issues)

	assert.Contains(t, got, "and 3 more")
	assert.NotContains(t, got, fmt.Sprintf("pkg%d", maxReportedPackages+1))
}

// TestLoadedPackagesHaveCompleteTypes is a guard against a mismatch between the
// Go toolchain and golang.org/x/tools.
//
// When x/tools cannot decode the toolchain's export data format, standard
// library packages load without type information. Depending on the x/tools
// version that either crashes the loader or, worse, succeeds quietly and makes
// the type-driven matching in parser/ silently stop recognizing calls — which
// shows up as instrumentation missing from generated diffs rather than as a
// loader problem. This test fails loudly at the real cause instead.
func TestLoadedPackagesHaveCompleteTypes(t *testing.T) {
	const code = `package main

import "net/http"

func main() {
	http.Get("http://example.com")
}
`

	// Built under t.TempDir so a crashed run cannot leave a directory behind
	// that breaks later runs: CreateTestApp uses a plain os.Mkdir, which fails
	// if the target already exists.
	testAppDir := filepath.Join(t.TempDir(), "testapp")
	pkgs, err := parser.CreateTestApp(t, testAppDir, "main.go", code)
	assert.NoError(t, err)

	assert.Empty(t, packageLoadIssues(pkgs),
		"packages did not load cleanly; check golang.org/x/tools against the Go toolchain version")

	var checked int
	for _, p := range pkgs {
		for path, imp := range p.Imports {
			if path != "net/http" || imp == nil || imp.Types == nil {
				continue
			}

			checked++
			assert.True(t, imp.Types.Complete(),
				"net/http has incomplete type information; golang.org/x/tools cannot decode this toolchain's export data")
			assert.NotNil(t, imp.Types.Scope().Lookup("Get"),
				"net/http.Get is missing from the type scope")
			assert.IsType(t, (*types.Func)(nil), imp.Types.Scope().Lookup("Get"))
		}
	}

	assert.Positive(t, checked, "net/http types were never checked; the fixture or load mode changed")
}
