package cmd

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/newrelic/go-easy-instrumentation/internal/usage"
	"github.com/stretchr/testify/assert"
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
