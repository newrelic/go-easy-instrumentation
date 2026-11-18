package usage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenInsertKey(t *testing.T) {
	tests := []struct {
		name  string
		runID string
		want  string
	}{
		{
			// Verified against newrelic-diagnostics-cli's genInsertKey. If this
			// fails, the two tools no longer agree and the port has drifted.
			name:  "matches the diagnostics CLI for a known run ID",
			runID: "gei-spike-1",
			want:  "2c35c6c5e986f92dd5f8b5f210f9b9a48e9f31a67132c522208824b3e9c4914bf967c4aa6cec0ab3cddfa5c74222f8b6ab3b59cbf384455073cc79fab9831b40",
		},
		{
			name:  "empty run ID hashes the empty seed",
			runID: "",
			want:  "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, genInsertKey(tt.runID))
		})
	}
}

func TestGenInsertKeyIsDeterministic(t *testing.T) {
	assert.Equal(t, genInsertKey("abc123"), genInsertKey("abc123"))
	// Differs at an even index, so it reaches the hash seed.
	assert.NotEqual(t, genInsertKey("abc123"), genInsertKey("abd123"))
}

// TestGenInsertKeyIgnoresOddIndexes documents a surprising property of the
// algorithm this package ports: only even-indexed bytes of the run ID reach the
// hash, so run IDs differing only at odd indexes produce the same key. That is
// harmless here because the value is not a credential and the endpoint does not
// validate it, but it means the key is not a unique identifier for a run.
func TestGenInsertKeyIgnoresOddIndexes(t *testing.T) {
	assert.Equal(t, genInsertKey("abc123"), genInsertKey("aXcY2Z"))
}

func TestNewRunID(t *testing.T) {
	id := newRunID()

	assert.Len(t, id, runIDBytes*2, "run ID should be hex encoded")
	assert.NotEqual(t, id, newRunID(), "run IDs should not repeat")
}
