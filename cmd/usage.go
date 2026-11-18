package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/newrelic/go-easy-instrumentation/internal/usage"
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

	return usage.RunData{
		Mode:          mode,
		Verbose:       debug,
		OutputPathSet: diffFile != "",
		ExcludeCount:  excludeCount(),
	}
}

// excludeCount reports how many directories were excluded. The names themselves
// are never reported: they describe the layout of the user's source tree.
func excludeCount() int {
	count := 0
	for _, dir := range strings.Split(excludeDirs, ",") {
		if strings.TrimSpace(dir) != "" {
			count++
		}
	}

	return count
}

// fileSize returns the size of the file at path, or zero if it cannot be read.
func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}

	return info.Size()
}

// debugPrintf prints a message only when --debug is set.
func debugPrintf(format string, args ...any) {
	if !debug {
		return
	}

	fmt.Printf(format+"\n", args...)
}
