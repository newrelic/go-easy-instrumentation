package usage

import (
	"os"
	"strconv"
)

const (
	// endpointEnvVar overrides the URL usage data is posted to. It exists for
	// local testing and is not intended for general use.
	endpointEnvVar = "NEW_RELIC_USAGE_ENDPOINT"

	// enabledEnvVar disables usage reporting when set to a false value.
	enabledEnvVar = "NEW_RELIC_USAGE_REPORTING_ENABLED"

	// stagingEndpoint is the staging usage endpoint on help-agnostic-service.
	stagingEndpoint = "https://diagnostics.staging-service.newrelic.com/usage"

	// productionEndpoint is the endpoint real user traffic should eventually
	// report to. It is deliberately unused: this tool has no build tooling that
	// could select an endpoint at release time, so every build reports to
	// staging until that exists. See docs/spike-usage-monitoring.md.
	productionEndpoint = "https://diagnostics.service.newrelic.com/usage"
)

// endpoint returns the URL usage data is posted to.
func endpoint() string {
	if override := os.Getenv(endpointEnvVar); override != "" {
		return override
	}
	return stagingEndpoint
}

// Enabled reports whether usage reporting is turned on for this run.
//
// Reporting is on by default and is disabled by setting enabledEnvVar to a
// false value. A value that cannot be parsed as a boolean also disables
// reporting: someone who set the variable at all meant to change something, and
// declining to send data is the safer way to interpret that.
func Enabled() bool {
	raw := os.Getenv(enabledEnvVar)
	if raw == "" {
		return true
	}

	on, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}

	return on
}
