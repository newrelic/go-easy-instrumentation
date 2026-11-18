package usage

import (
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
)

// runIDBytes is the length in bytes of a generated run ID.
const runIDBytes = 16

// newRunID returns a random identifier that ties together the events emitted for
// a single run.
//
// newrelic-diagnostics-cli generates a version 1 UUID here, which embeds the
// host MAC address. A random value is used instead so that no hardware
// identifier leaves the machine.
func newRunID() string {
	b := make([]byte, runIDBytes)
	if _, err := rand.Read(b); err != nil {
		return ""
	}

	return hex.EncodeToString(b)
}

// genInsertKey returns a deterministic hash of runID for the Insert-Key header.
//
// This is not a credential. The usage endpoint is unauthenticated and does not
// validate the header; the header exists because the endpoint's original client
// sent it. The algorithm is a port of genInsertKey in newrelic-diagnostics-cli's
// usage/usage.go, kept identical so that both tools produce the same header for
// the same run ID: every even-indexed byte of the run ID is accumulated in
// reverse order, then hashed with SHA-512.
func genInsertKey(runID string) string {
	hashSeed := []byte{}
	for i, b := range []byte(runID) {
		if i%2 == 0 {
			hashSeed = append([]byte{b}, hashSeed...)
		}
	}

	h := sha512.New()
	h.Write(hashSeed)

	return hex.EncodeToString(h.Sum(nil))
}
