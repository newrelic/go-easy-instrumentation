package cmd

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/usage"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

// TestInitialModelSeedsUsageData guards the reason a cancelled run used to look
// like a successful one: if the model's run data starts as a zero value, a
// cancel before the worker reports anything sends an event with no mode, no
// duration and no failure, indistinguishable from a clean run.
func TestInitialModelSeedsUsageData(t *testing.T) {
	m := initialModel(".", "out.diff")

	assert.Equal(t, usage.ModeTUI, m.usageData.Mode, "run data must be populated before the worker reports")
	assert.False(t, m.cancelled)
}

func TestCtrlCMarksRunCancelled(t *testing.T) {
	m := initialModel(".", "out.diff")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	got, ok := updated.(model)
	assert.True(t, ok)
	assert.True(t, got.cancelled, "ctrl+c must be recorded, so reporting can distinguish it")
	assert.NotNil(t, cmd, "ctrl+c should still quit")
}

func TestUsageMsgStoresRunData(t *testing.T) {
	m := initialModel(".", "out.diff")

	updated, _ := m.Update(usageMsg(usage.RunData{Mode: usage.ModeTUI, PackagesLoaded: 3}))

	got, ok := updated.(model)
	assert.True(t, ok)
	assert.Equal(t, 3, got.usageData.PackagesLoaded)
	assert.False(t, got.cancelled)
}

// TestCtrlCStopsWorker guards against the worker starting further pipeline
// steps, such as `go get` on the user's module, while a cancelled run is being
// reported.
func TestCtrlCStopsWorker(t *testing.T) {
	m := initialModel(".", "out.diff")
	m.stop = make(chan struct{})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	select {
	case <-updated.(model).stop:
	default:
		t.Fatal("ctrl+c must close the stop channel")
	}

	assert.NotPanics(t, func() {
		updated.(model).Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	}, "a second ctrl+c must not close the stop channel again")
}

// TestDirectImportsExcludesTransitiveDependencies guards what "uses a library"
// means: a library gin depends on is not one the application uses.
func TestDirectImportsExcludesTransitiveDependencies(t *testing.T) {
	transitive := &packages.Package{ID: "github.com/go-playground/validator/v10", PkgPath: "github.com/go-playground/validator/v10"}
	gin := &packages.Package{ID: "github.com/gin-gonic/gin", Imports: map[string]*packages.Package{
		"github.com/go-playground/validator/v10": transitive,
	}}

	pkgs := []*decorator.Package{
		{Package: &packages.Package{ID: "app", Imports: map[string]*packages.Package{
			"github.com/gin-gonic/gin": gin,
			"fmt":                      {ID: "fmt"},
		}}},
		nil,
	}

	assert.Equal(t, []string{"fmt", "github.com/gin-gonic/gin"}, directImports(pkgs))
}

// TestDirectImportsSkipsTestPackages guards against a library used only from
// _test.go files being reported as one the application uses.
func TestDirectImportsSkipsTestPackages(t *testing.T) {
	imports := func(paths ...string) map[string]*packages.Package {
		m := map[string]*packages.Package{}
		for _, p := range paths {
			m[p] = &packages.Package{ID: p}
		}
		return m
	}

	pkgs := []*decorator.Package{
		{Package: &packages.Package{ID: "app", Imports: imports("fmt")}},
		{Package: &packages.Package{ID: "app [app.test]", ForTest: "app", Imports: imports("fmt", "net/http/httptest")}},
		{Package: &packages.Package{ID: "app_test [app.test]", ForTest: "app", Imports: imports("github.com/mattn/go-sqlite3")}},
		{Package: &packages.Package{ID: "app.test", Imports: imports("testing/internal/testdeps")}},
	}

	assert.Equal(t, []string{"fmt"}, directImports(pkgs))
}

func TestLoadErrorKinds(t *testing.T) {
	dep := &packages.Package{ID: "dep", Errors: []packages.Error{{Kind: packages.ListError}}}
	pkgs := []*decorator.Package{
		{Package: &packages.Package{
			ID:      "app",
			Errors:  []packages.Error{{Kind: packages.TypeError}, {Kind: packages.TypeError}, {Kind: packages.ParseError}},
			Imports: map[string]*packages.Package{"dep": dep},
		}},
	}

	assert.Equal(t, []string{"ListError", "ParseError", "TypeError"}, loadErrorKinds(pkgs))
	assert.Empty(t, loadErrorKinds([]*decorator.Package{{Package: &packages.Package{ID: "clean"}}}))
}
