package usage

import (
	"strings"
	"testing"

	"github.com/newrelic/go-easy-instrumentation/integrations/nrecho-v3"
	nrecho_v4 "github.com/newrelic/go-easy-instrumentation/integrations/nrecho-v4"
	"github.com/newrelic/go-easy-instrumentation/integrations/nrgin"
	"github.com/newrelic/go-easy-instrumentation/integrations/nrgochi"
	"github.com/newrelic/go-easy-instrumentation/integrations/nrgrpc"
	"github.com/newrelic/go-easy-instrumentation/integrations/nrlogrus"
	"github.com/newrelic/go-easy-instrumentation/integrations/nrpgx5"
	"github.com/newrelic/go-easy-instrumentation/integrations/nrpq"
	"github.com/newrelic/go-easy-instrumentation/integrations/nrslog"
	"github.com/stretchr/testify/assert"
)

// TestSupportedLibrariesMatchIntegrations keeps the library list in step with
// the integrations. If an integration changes the import path it adds, its
// library would silently report Warning on every run.
func TestSupportedLibrariesMatchIntegrations(t *testing.T) {
	want := map[string][2]string{
		"gin":     {nrgin.GinImportPath, nrgin.NrginImportPath},
		"echo-v4": {nrecho_v4.EchoImportPath, nrecho_v4.NrechoImportPath},
		"echo-v3": {nrecho.EchoImportPath, nrecho.NrechoImportPath},
		"chi-v5":  {nrgochi.GochiImportPath, nrgochi.NrChiImportPath},
		"grpc":    {nrgrpc.GrpcImportPath, nrgrpc.NrgrpcImportPath},
		"logrus":  {nrlogrus.LogrusImportPath, nrlogrus.NrlogrusImportPath},
		"slog":    {nrslog.SlogImportPath, nrslog.NrslogImportPath},
		"pq":      {nrpq.LibpqImportPath, nrpq.NrpqImportPath},
		"pgx-v5":  {nrpgx5.PgxImportPath, nrpgx5.Nrpgx5ImportPath},
	}

	got := map[string][2]string{}
	for _, lib := range knownLibraries {
		if lib.integration != "" {
			got[lib.name] = [2]string{lib.importPath, lib.integration}
		}
	}

	assert.Equal(t, want, got)
}

func TestKnownLibrariesAreWellFormed(t *testing.T) {
	names := map[string]bool{}
	paths := map[string]bool{}

	for _, lib := range knownLibraries {
		assert.NotContains(t, lib.name, "/", "a name is one identifier segment")
		assert.NotEmpty(t, lib.name)
		assert.False(t, names[lib.name], "duplicate name %s", lib.name)
		assert.False(t, paths[lib.importPath], "duplicate import path %s", lib.importPath)
		assert.False(t, lib.unattributable && lib.integration != "", "%s: unattributable libraries add no integration import", lib.name)
		names[lib.name] = true
		paths[lib.importPath] = true
	}
}

func TestMatchLibrary(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"github.com/gin-gonic/gin", "gin"},
		{"github.com/gin-gonic/gin/binding", "gin"},
		{"github.com/labstack/echo/v4", "echo-v4"},
		{"github.com/labstack/echo/v4/middleware", "echo-v4"},
		{"github.com/labstack/echo", "echo-v3"},
		{"github.com/labstack/echo/v5", "echo-v5"},
		{"github.com/labstack/echo/v5/middleware", "echo-v5"},
		{"github.com/go-chi/chi/v5", "chi-v5"},
		{"github.com/go-chi/chi", "chi-legacy"},
		{"github.com/jackc/pgx/v5/pgxpool", "pgx-v5"},
		{"github.com/aws/aws-sdk-go-v2/service/s3", "aws-sdk-go-v2"},
		{"github.com/aws/aws-sdk-go/service/s3", "aws-sdk-go"},
		{"go.mongodb.org/mongo-driver/v2/mongo", "mongo"},
		{"github.com/elastic/go-elasticsearch/v8", "elasticsearch"},
		{"net/http", "nethttp"},
		{"net/http/httptest", "nethttp"},
		{"log/slog", "slog"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			lib, ok := matchLibrary(tt.path)
			assert.True(t, ok)
			assert.Equal(t, tt.want, lib.name)
		})
	}
}

// TestMatchLibraryRejectsLookalikes guards the path-boundary check: a module
// whose path merely starts with a known one is someone else's code.
func TestMatchLibraryRejectsLookalikes(t *testing.T) {
	for _, path := range []string{
		"github.com/gin-gonic/gin-contrib",
		"github.com/labstack/echoextra",
		"net/httputil",
		"go.uber.org/zapper",
		"github.com/acme/private-service/internal/db",
		"context",
	} {
		_, ok := matchLibrary(path)
		assert.False(t, ok, path)
	}
}

// TestDetectedLibrariesNeverLeakUnknownPaths is the privacy guarantee for
// library detection: only names from the fixed list come out.
func TestDetectedLibrariesNeverLeakUnknownPaths(t *testing.T) {
	data := RunData{
		DirectImports: []string{
			"github.com/acme/secret-project/billing",
			"github.com/gin-gonic/gin",
			"github.com/gin-gonic/gin/binding",
			"internal.example.com/team/repo",
			"fmt",
		},
	}

	results := prepareResults(data)

	for _, r := range results {
		assert.NotContains(t, r.Identifier, "acme")
		assert.NotContains(t, r.Identifier, "example.com")
		assert.NotContains(t, r.Identifier, "fmt")
	}
	assert.Contains(t, identifiers(results), "Go/Library/gin")
	assert.Len(t, results, 1, "gin is reported once, and nothing else is reported")
}

func TestLibraryStatus(t *testing.T) {
	gin, _ := matchLibrary("github.com/gin-gonic/gin")
	gorm, _ := matchLibrary("gorm.io/gorm")
	echoV5, _ := matchLibrary("github.com/labstack/echo/v5")
	nethttp, _ := matchLibrary("net/http")
	applied := map[string]bool{gin.integration: true}

	tests := []struct {
		name      string
		lib       library
		applied   map[string]bool
		runFailed bool
		want      string
	}{
		{"supported and applied", gin, applied, false, statusSuccess},
		{"supported, not applied", gin, map[string]bool{}, false, statusWarning},
		{"supported, not applied, run failed", gin, map[string]bool{}, true, statusNone},
		{"already instrumented, run failed", gin, applied, true, statusSuccess},
		{"newer major version of a supported library", echoV5, map[string]bool{}, false, statusInfo},
		{"unsupported", gorm, applied, false, statusInfo},
		{"unsupported, run failed", gorm, applied, true, statusInfo},
		{"unattributable", nethttp, applied, false, statusNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, libraryStatus(tt.lib, tt.applied, tt.runFailed))
		})
	}
}

func TestPrepareResultsLibraries(t *testing.T) {
	data := RunData{
		AddedImports: []string{
			nrgin.NrginImportPath,
			"github.com/newrelic/go-agent/v3/newrelic",
		},
		DirectImports: []string{
			"github.com/gin-gonic/gin",
			"github.com/sirupsen/logrus",
			"gorm.io/gorm",
			"net/http",
		},
	}

	got := map[string]string{}
	for _, r := range prepareResults(data) {
		got[r.Identifier] = r.Status
		assert.Len(t, strings.Split(r.Identifier, "/"), 3, r.Identifier)
	}

	assert.Equal(t, map[string]string{
		"Go/Integration/nrgin":    statusSuccess,
		"Go/Integration/newrelic": statusSuccess,
		"Go/Library/gin":          statusSuccess,
		"Go/Library/logrus":       statusWarning,
		"Go/Library/gorm":         statusInfo,
		"Go/Library/nethttp":      statusNone,
	}, got)
}

// TestPrepareResultsAlreadyInstrumented guards against an application that
// already imports an integration, from an earlier run or by hand, reporting a
// pattern-matching gap because this run had nothing left to add.
func TestPrepareResultsAlreadyInstrumented(t *testing.T) {
	results := prepareResults(RunData{
		DirectImports: []string{
			"github.com/gin-gonic/gin",
			nrgin.NrginImportPath,
		},
	})

	assert.Equal(t, []taskResult{{Identifier: "Go/Library/gin", Status: statusSuccess}}, results)
}

func TestPrepareResultsCancelledReportsNoLibraries(t *testing.T) {
	results := prepareResults(RunData{
		Cancelled:     true,
		DirectImports: []string{"github.com/gin-gonic/gin"},
	})

	assert.Equal(t, []string{"Go/Pipeline/Cancelled"}, identifiers(results))
}

func identifiers(results []taskResult) []string {
	ids := make([]string, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.Identifier)
	}
	return ids
}
