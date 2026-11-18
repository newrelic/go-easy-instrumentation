// Package usage reports anonymous information about a single instrumentation run
// to New Relic.
//
// The payload is posted to an unauthenticated endpoint on help-agnostic-service,
// which records it as a New Relic custom event using its own agent. This tool
// therefore holds no license key and no credential of any kind. The payload
// shape and the request headers match newrelic-diagnostics-cli, because the same
// endpoint serves both tools.
//
// Nothing describing the user's code is reported: no source, no identifiers from
// their program, no file paths, no module path, no hostname, and no user name.
// Flags whose values could carry any of those are reduced to whether they were
// set.
package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/newrelic/go-easy-instrumentation/internal/common"
)

const (
	// protocolVersion is the version of the payload format the endpoint expects.
	protocolVersion = "1.0"

	// requestTimeout bounds a single attempt. Usage reporting happens after the
	// user already has their diff, so this delays nothing they are waiting on.
	requestTimeout = 3 * time.Second

	// devVersion is what the Go toolchain records as the main module version for
	// a binary built from a local checkout rather than installed by version.
	devVersion = "(devel)"

	// integrationPrefix is the import path prefix shared by every New Relic Go
	// agent integration.
	integrationPrefix = "github.com/newrelic/go-agent/v3/integrations/"

	// agentImportPath is the base agent import, added whenever any
	// instrumentation at all is applied.
	agentImportPath = "github.com/newrelic/go-agent/v3/newrelic"
)

// Statuses reported for a single result. These match the status strings the
// endpoint counts, so they must not be renamed.
const (
	statusSuccess = "Success"
	statusError   = "Error"
	// statusNone reports an outcome that is genuinely unknown, rather than
	// claiming one that was never observed.
	statusNone = "None"
)

// Modes describing how the tool was driven for a run.
const (
	ModeTUI         = "tui"
	ModeText        = "text"
	ModeInteractive = "interactive"
)

// RunData describes one instrumentation run. Every field is either a count, a
// boolean, or a value fixed by this tool: none of them can carry information
// about the code being instrumented.
type RunData struct {
	// AddedImports is every import path the run added across all packages.
	// Integration names are derived from it.
	AddedImports []string

	// Mode is how the tool was driven: "tui", "text", or "interactive".
	Mode string

	// FailedStep is the name of the pipeline step that failed, or empty if the
	// run succeeded.
	FailedStep string

	// Cancelled reports that the user interrupted the run before it finished.
	Cancelled bool

	// Duration is how long the run took.
	Duration time.Duration

	// DiffBytes is the size of the generated diff file.
	DiffBytes int64

	// PackagesLoaded is how many packages were loaded for instrumentation.
	PackagesLoaded int

	// PackagesWithErrors is how many packages, including dependencies, failed to
	// load cleanly.
	PackagesWithErrors int

	// ExcludeCount is how many directories were excluded, never which ones.
	ExcludeCount int

	// OutputPathSet records whether a custom output path was given, never the
	// path itself.
	OutputPathSet bool

	// Verbose records whether debug output was enabled.
	Verbose bool
}

// payload is the request body. It mirrors the endpoint's expected shape.
type payload struct {
	Protocol string  `json:"protocol"`
	Data     runData `json:"data"`
}

type runData struct {
	MetaData      metaData     `json:"metadata"`
	Configuration []configFlag `json:"configuration"`
	Results       []taskResult `json:"results"`
}

// configFlag becomes a single event attribute. The endpoint prefixes the name
// with "config" and capitalizes it, so Name "product" arrives as
// "configProduct".
type configFlag struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

type metaData struct {
	Timestamp     int64    `json:"timestamp"`
	NRDiagVersion string   `json:"nrdiagVersion"`
	RunID         string   `json:"runId"`
	RpmApps       []rpmApp `json:"rpmApps"`
	Hostname      string   `json:"hostname"`
	LicenseKeys   []string `json:"licenseKeys"`
}

// taskResult becomes one event of its own. Identifier must have exactly three
// "/"-separated segments: the endpoint splits it into category, subcategory and
// name by index, and a shorter identifier makes it panic.
type taskResult struct {
	Identifier string `json:"identifier"`
	Status     string `json:"status"`
	URL        string `json:"URL"`
}

// rpmApp is always reported empty. This tool never learns an account or
// application ID, so the endpoint leaves the corresponding attributes off the
// event entirely.
type rpmApp struct {
	AppID     string `json:"appId"`
	AccountID string `json:"accountId"`
}

// Send reports one run and returns the run ID it was reported under.
//
// This is a single attempt with no retry: the endpoint has no rate limiting of
// its own, and a tool that runs on every developer's machine should not add
// retry traffic to it. The run ID is returned so a caller can log it when
// debugging.
func Send(data RunData) (string, error) {
	runID := newRunID()

	body, err := json.Marshal(preparePayload(data, runID))
	if err != nil {
		return runID, fmt.Errorf("marshaling usage payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint(), bytes.NewReader(body))
	if err != nil {
		return runID, fmt.Errorf("building usage request: %w", err)
	}

	version, _ := toolVersion()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Usage-Protocol", protocolVersion)
	req.Header.Set("Run-Id", runID)
	req.Header.Set("Insert-Key", genInsertKey(runID))
	req.Header.Set("User-Agent", common.ApplicationName+"/"+version)

	client := &http.Client{Timeout: requestTimeout}

	resp, err := client.Do(req)
	if err != nil {
		return runID, fmt.Errorf("sending usage data: %w", err)
	}
	defer resp.Body.Close()

	// Drain the body so the connection can be reused and so a large response
	// cannot be left dangling.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return runID, fmt.Errorf("usage endpoint returned status %d", resp.StatusCode)
	}

	return runID, nil
}

// preparePayload assembles the request body for a run.
func preparePayload(data RunData, runID string) payload {
	return payload{
		Protocol: protocolVersion,
		Data: runData{
			MetaData:      prepareMeta(runID),
			Configuration: prepareConfiguration(data),
			Results:       prepareResults(data),
		},
	}
}

// prepareMeta builds the run-level metadata.
//
// Hostname is deliberately empty and the license key and application slices are
// deliberately absent. newrelic-diagnostics-cli fills all three, because it runs
// on servers where they are the point; this tool runs on development machines
// where they are only personal data.
func prepareMeta(runID string) metaData {
	version, _ := toolVersion()

	return metaData{
		Timestamp:     time.Now().Unix(),
		NRDiagVersion: version,
		RunID:         runID,
		RpmApps:       []rpmApp{},
		Hostname:      "",
		LicenseKeys:   []string{},
	}
}

// prepareConfiguration builds the run-level attributes.
//
// Names reuse those already reported by newrelic-diagnostics-cli wherever the
// two tools mean the same thing, so that a single dashboard can read both.
func prepareConfiguration(data RunData) []configFlag {
	_, devBuild := toolVersion()

	return []configFlag{
		// product distinguishes this tool's events from the diagnostics CLI's,
		// which share the same event types. Every query needs it.
		{Name: "product", Value: common.ApplicationName},
		{Name: "mode", Value: data.Mode},
		{Name: "interactive", Value: data.Mode == ModeInteractive},
		{Name: "verbose", Value: data.Verbose},
		// outputPath is a boolean, never the path itself.
		{Name: "outputPath", Value: data.OutputPathSet},
		// exclude is a count, never the directory names: those describe the
		// layout of the user's source tree.
		{Name: "exclude", Value: data.ExcludeCount},
		{Name: "goVersion", Value: runtime.Version()},
		{Name: "os", Value: runtime.GOOS},
		{Name: "arch", Value: runtime.GOARCH},
		{Name: "devBuild", Value: devBuild},
		{Name: "packagesLoaded", Value: data.PackagesLoaded},
		{Name: "packagesWithErrors", Value: data.PackagesWithErrors},
		{Name: "diffBytes", Value: data.DiffBytes},
		{Name: "durationMs", Value: data.Duration.Milliseconds()},
	}
}

// prepareResults builds one result per integration applied, plus one for a
// failed pipeline step.
//
// The endpoint derives its per-status totals from these, so no counts are sent
// directly.
func prepareResults(data RunData) []taskResult {
	results := []taskResult{}

	// A run the user interrupted tells us nothing about the integrations it had
	// reached, and reporting them would overstate what was observed. Report the
	// cancellation alone.
	if data.Cancelled {
		return append(results, taskResult{
			Identifier: "Go/Pipeline/Cancelled",
			Status:     statusNone,
		})
	}

	// Integrations are only known to have succeeded if the whole run did. When a
	// later step failed, the import was added but the instrumentation was never
	// written out, so the outcome is unknown rather than successful.
	integrationStatus := statusSuccess
	if data.FailedStep != "" {
		integrationStatus = statusNone
	}

	for _, name := range IntegrationNames(data.AddedImports) {
		results = append(results, taskResult{
			Identifier: "Go/Integration/" + name,
			Status:     integrationStatus,
		})
	}

	if data.FailedStep != "" {
		results = append(results, taskResult{
			Identifier: "Go/Pipeline/" + identifierSegment(data.FailedStep),
			Status:     statusError,
		})
	}

	return results
}

// IntegrationNames returns the integrations represented by a set of added import
// paths, in the order they appear.
//
// An integration calls AddImport only when it has matched and injected code, so
// its import path appearing here means it was applied. Note that the net/http
// and database/sql instrumentation add only the base agent import and so cannot
// be told apart from each other; both are reported as "newrelic".
func IntegrationNames(addedImports []string) []string {
	names := []string{}
	seen := map[string]bool{}

	for _, path := range addedImports {
		var name string

		switch {
		case strings.HasPrefix(path, integrationPrefix):
			// Trailing segment, so that nested paths such as
			// logcontext-v2/nrslog report as "nrslog".
			name = path[strings.LastIndex(path, "/")+1:]
		case path == agentImportPath:
			name = "newrelic"
		default:
			// Anything else is a standard library or third-party import added
			// as a side effect, such as "context".
			continue
		}

		if name == "" || seen[name] {
			continue
		}

		seen[name] = true
		names = append(names, name)
	}

	return names
}

// identifierSegment converts a pipeline step description into a single
// identifier segment. The endpoint splits identifiers on "/", so a segment must
// not contain one.
func identifierSegment(step string) string {
	var b strings.Builder

	for _, r := range step {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			// Drop spaces, slashes and punctuation rather than substituting, so
			// "Writing diff file" becomes "Writingdifffile".
		}
	}

	if b.Len() == 0 {
		return "Unknown"
	}

	return b.String()
}

// toolVersion returns the version of this tool and whether it is a development
// build.
//
// The version comes from the build info rather than a constant, because this
// tool is distributed with "go install", which stamps the real module version,
// while the AppVersion constant is updated by hand and drifts.
func toolVersion() (version string, devBuild bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return devVersion, true
	}

	return versionFromBuildInfo(info.Main.Version, info.Settings)
}

// versionFromBuildInfo derives the reported version and the development-build
// flag from build info.
//
// Distinguishing a real install from a local build is not as simple as checking
// for "(devel)": when the source is a git checkout, the toolchain stamps a
// pseudo-version derived from the latest tag, so a developer's own build reports
// something that looks like a release. The reliable signal is the presence of
// VCS settings — "go install module@version" builds from the module cache and
// records none, while a build from a checkout always does.
func versionFromBuildInfo(mainVersion string, settings []debug.BuildSetting) (version string, devBuild bool) {
	fromCheckout := false
	for _, setting := range settings {
		if setting.Key == "vcs.revision" {
			fromCheckout = true
			break
		}
	}

	if mainVersion == "" {
		return devVersion, true
	}

	return mainVersion, fromCheckout || mainVersion == devVersion
}
