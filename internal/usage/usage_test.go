package usage

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIntegrationNames(t *testing.T) {
	type args struct {
		addedImports []string
	}

	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "no imports",
			args: args{addedImports: []string{}},
			want: []string{},
		},
		{
			name: "integration import yields its trailing segment",
			args: args{addedImports: []string{"github.com/newrelic/go-agent/v3/integrations/nrgin"}},
			want: []string{"nrgin"},
		},
		{
			name: "nested integration path yields the trailing segment",
			args: args{addedImports: []string{"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog"}},
			want: []string{"nrslog"},
		},
		{
			name: "echo versions stay distinct",
			args: args{addedImports: []string{
				"github.com/newrelic/go-agent/v3/integrations/nrecho-v3",
				"github.com/newrelic/go-agent/v3/integrations/nrecho-v4",
			}},
			want: []string{"nrecho-v3", "nrecho-v4"},
		},
		{
			name: "base agent import reports as newrelic",
			args: args{addedImports: []string{"github.com/newrelic/go-agent/v3/newrelic"}},
			want: []string{"newrelic"},
		},
		{
			name: "standard library imports are ignored",
			args: args{addedImports: []string{"context", "net/http"}},
			want: []string{},
		},
		{
			name: "duplicates are collapsed",
			args: args{addedImports: []string{
				"github.com/newrelic/go-agent/v3/integrations/nrgin",
				"github.com/newrelic/go-agent/v3/integrations/nrgin",
			}},
			want: []string{"nrgin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IntegrationNames(tt.args.addedImports))
		})
	}
}

func TestIdentifierSegment(t *testing.T) {
	tests := []struct {
		name string
		step string
		want string
	}{
		{name: "spaces are dropped", step: "Writing diff file", want: "Writingdifffile"},
		{name: "slashes are dropped", step: "a/b", want: "ab"},
		{name: "empty falls back", step: "", want: "Unknown"},
		{name: "punctuation only falls back", step: "///", want: "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := identifierSegment(tt.step)
			assert.Equal(t, tt.want, got)
			assert.NotContains(t, got, "/", "identifier segments must not contain a separator")
		})
	}
}

func TestPrepareResults(t *testing.T) {
	tests := []struct {
		name string
		data RunData
		want []taskResult
	}{
		{
			name: "no integrations and no failure",
			data: RunData{},
			want: []taskResult{},
		},
		{
			name: "one row per integration",
			data: RunData{AddedImports: []string{
				"github.com/newrelic/go-agent/v3/integrations/nrgin",
				"github.com/newrelic/go-agent/v3/integrations/nrpq",
			}},
			want: []taskResult{
				{Identifier: "Go/Integration/nrgin", Status: statusSuccess},
				{Identifier: "Go/Integration/nrpq", Status: statusSuccess},
			},
		},
		{
			name: "a failed step becomes an error row",
			data: RunData{FailedStep: "Writing diff file"},
			want: []taskResult{
				{Identifier: "Go/Pipeline/Writingdifffile", Status: statusError},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, prepareResults(tt.data))
		})
	}
}

// TestPrepareResultsIdentifiersHaveThreeSegments guards the one payload mistake
// that breaks the receiving service: it splits an identifier into category,
// subcategory and name by index, so anything shorter makes it panic.
func TestPrepareResultsIdentifiersHaveThreeSegments(t *testing.T) {
	data := RunData{
		AddedImports: []string{
			"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/nrslog",
			"github.com/newrelic/go-agent/v3/newrelic",
		},
		FailedStep: "Adding required modules",
	}

	results := prepareResults(data)
	assert.Len(t, results, 3)

	for _, result := range results {
		assert.Len(t, strings.Split(result.Identifier, "/"), 3, result.Identifier)
	}
}

// TestPrepareConfigurationAttributes pins the exact set of run-level
// attributes. Adding one is a deliberate decision: each must answer a question
// this data exists for, and none may carry a value from the user's code.
func TestPrepareConfigurationAttributes(t *testing.T) {
	config := prepareConfiguration(RunData{
		Mode:           ModeText,
		LoadErrorKinds: []string{"TypeError", "ListError", "/home/someone/app", "TypeError"},
	})

	byName := map[string]any{}
	names := []string{}
	for _, flag := range config {
		byName[flag.Name] = flag.Value
		names = append(names, flag.Name)
	}

	assert.Equal(t, []string{
		"product", "mode", "goVersion", "os", "arch", "devBuild",
		"packagesLoaded", "packagesWithErrors", "loadErrorKinds", "durationMs",
	}, names)
	assert.Equal(t, "go-easy-instrumentation", byName["product"], "the marker attribute must be present")
	assert.Equal(t, "ListError,TypeError", byName["loadErrorKinds"], "only known kinds, sorted and deduplicated")
}

// TestPrepareMetaOmitsIdentifyingData covers the fields the diagnostics CLI
// fills and this tool deliberately leaves empty.
func TestPrepareMetaOmitsIdentifyingData(t *testing.T) {
	meta := prepareMeta("run-1")

	assert.Equal(t, "run-1", meta.RunID)
	assert.Empty(t, meta.Hostname, "hostname must never be reported")
	assert.Empty(t, meta.LicenseKeys, "this tool never sees a license key")
	assert.Empty(t, meta.RpmApps, "this tool never sees an application ID")
	assert.NotZero(t, meta.Timestamp)
}

// TestPayloadAttributeCountUnderLimit guards the hard 64-attribute limit on
// custom events. The receiving service adds attributes of its own on top of
// these, so the payload needs real headroom rather than to merely fit.
func TestPayloadAttributeCountUnderLimit(t *testing.T) {
	config := prepareConfiguration(RunData{})
	assert.Less(t, len(config), 30, "leave room for service-added attributes")
}

func TestSend(t *testing.T) {
	var (
		gotHeaders http.Header
		gotBody    []byte
		gotMethod  string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeaders = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	t.Setenv(endpointEnvVar, server.URL)

	runID, err := Send(RunData{
		AddedImports:   []string{"github.com/newrelic/go-agent/v3/integrations/nrgin"},
		Mode:           ModeText,
		PackagesLoaded: 7,
		Duration:       1500 * time.Millisecond,
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, runID)
	assert.Equal(t, http.MethodPost, gotMethod)

	assert.Equal(t, "application/json", gotHeaders.Get("Content-Type"))
	assert.Equal(t, protocolVersion, gotHeaders.Get("Usage-Protocol"))
	assert.Equal(t, runID, gotHeaders.Get("Run-Id"))
	assert.Equal(t, genInsertKey(runID), gotHeaders.Get("Insert-Key"))
	assert.Contains(t, gotHeaders.Get("User-Agent"), "go-easy-instrumentation/")

	var sent payload
	assert.NoError(t, json.Unmarshal(gotBody, &sent), "body must be valid JSON")
	assert.Equal(t, protocolVersion, sent.Protocol)
	assert.Equal(t, runID, sent.Data.MetaData.RunID)
	assert.Equal(t, []taskResult{
		{Identifier: "Go/Integration/nrgin", Status: statusSuccess},
	}, sent.Data.Results)
}

func TestSendReturnsErrorOnNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	t.Setenv(endpointEnvVar, server.URL)

	_, err := Send(RunData{})
	assert.ErrorContains(t, err, "503")
}

// TestSendReturnsErrorWhenUnreachable is the case that matters most: a dead
// endpoint must produce an error to discard, not a panic.
func TestSendReturnsErrorWhenUnreachable(t *testing.T) {
	// Port 1 is reserved and refuses connections immediately.
	t.Setenv(endpointEnvVar, "http://127.0.0.1:1/usage")

	runID, err := Send(RunData{})
	assert.Error(t, err)
	assert.NotEmpty(t, runID, "the run ID is returned even on failure, for logging")
}

func TestEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
		want  bool
	}{
		{name: "on by default", set: false, want: true},
		{name: "empty value is on", set: true, value: "", want: true},
		{name: "false disables", set: true, value: "false", want: false},
		{name: "zero disables", set: true, value: "0", want: false},
		{name: "true enables", set: true, value: "true", want: true},
		{name: "unparseable value disables", set: true, value: "maybe", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv(enabledEnvVar, tt.value)
			} else {
				// Set then unset, so the cleanup t.Setenv registers still
				// restores whatever the developer had exported.
				t.Setenv(enabledEnvVar, "")
				assert.NoError(t, os.Unsetenv(enabledEnvVar))
			}
			assert.Equal(t, tt.want, Enabled())
		})
	}
}

func TestEndpointDefaultsToStaging(t *testing.T) {
	assert.Equal(t, stagingEndpoint, endpoint())

	t.Setenv(endpointEnvVar, "http://example.invalid/usage")
	assert.Equal(t, "http://example.invalid/usage", endpoint())
}

func TestVersionFromBuildInfo(t *testing.T) {
	checkout := []debug.BuildSetting{
		{Key: "vcs.revision", Value: "2cf23bba3415"},
		{Key: "vcs.modified", Value: "true"},
	}
	installed := []debug.BuildSetting{{Key: "GOARCH", Value: "arm64"}}

	type args struct {
		mainVersion string
		settings    []debug.BuildSetting
	}

	tests := []struct {
		name         string
		args         args
		wantVersion  string
		wantDevBuild bool
	}{
		{
			name:         "installed release is not a dev build",
			args:         args{mainVersion: "v1.1.0", settings: installed},
			wantVersion:  "v1.1.0",
			wantDevBuild: false,
		},
		{
			// The case that made this function necessary: a local build of a
			// tagged checkout reports a pseudo-version, which looks like a
			// release until you notice the VCS settings.
			name:         "pseudo-version from a checkout is a dev build",
			args:         args{mainVersion: "v1.1.1-0.20260702154532-2cf23bba3415+dirty", settings: checkout},
			wantVersion:  "v1.1.1-0.20260702154532-2cf23bba3415+dirty",
			wantDevBuild: true,
		},
		{
			name:         "explicit devel marker is a dev build",
			args:         args{mainVersion: "(devel)", settings: nil},
			wantVersion:  "(devel)",
			wantDevBuild: true,
		},
		{
			name:         "missing version falls back",
			args:         args{mainVersion: "", settings: nil},
			wantVersion:  devVersion,
			wantDevBuild: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, devBuild := versionFromBuildInfo(tt.args.mainVersion, tt.args.settings)
			assert.Equal(t, tt.wantVersion, version)
			assert.Equal(t, tt.wantDevBuild, devBuild)
		})
	}
}

// TestPrepareResultsDoesNotOverstateOutcomes covers the two ways this package
// could report something it never observed: claiming success for integrations
// on a run that failed partway through, and reporting an interrupted run as a
// clean one that happened to instrument nothing. Misleading data is worse than
// no data, so these are guarded.
func TestPrepareResultsDoesNotOverstateOutcomes(t *testing.T) {
	imports := []string{"github.com/newrelic/go-agent/v3/integrations/nrgin"}

	t.Run("a completed run reports success", func(t *testing.T) {
		results := prepareResults(RunData{AddedImports: imports})

		assert.Equal(t, []taskResult{
			{Identifier: "Go/Integration/nrgin", Status: statusSuccess},
		}, results)
	})

	t.Run("a failed run does not claim its integrations succeeded", func(t *testing.T) {
		results := prepareResults(RunData{AddedImports: imports, FailedStep: "Writing diff file"})

		assert.Equal(t, []taskResult{
			{Identifier: "Go/Integration/nrgin", Status: statusNone},
			{Identifier: "Go/Pipeline/Writingdifffile", Status: statusError},
		}, results)
	})

	t.Run("a cancelled run reports only the cancellation", func(t *testing.T) {
		results := prepareResults(RunData{AddedImports: imports, Cancelled: true})

		assert.Equal(t, []taskResult{
			{Identifier: "Go/Pipeline/Cancelled", Status: statusNone},
		}, results)
	})

	t.Run("a cancelled run is distinguishable from an empty successful one", func(t *testing.T) {
		cancelled := prepareResults(RunData{Cancelled: true})
		empty := prepareResults(RunData{})

		assert.NotEqual(t, empty, cancelled)
		assert.Empty(t, empty, "a run that instrumented nothing has no results")
	})
}
