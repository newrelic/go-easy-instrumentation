package pkg4

import (
	"encoding/base64"
	"fmt"
)

type Child struct {
	secret string
}

func (c *Child) DecodeSecret() string {
	// Decode the base64 encoded secret
	decodedBytes, err := base64.StdEncoding.DecodeString(c.secret)
	if err != nil {
		// Handle error in decoding
		return "Error decoding secret: " + err.Error()
	}
	// Return the decoded string
	return string(decodedBytes)
}

type Counter struct {
	count int
	name  string
	child *Child // Embedding Child struct to demonstrate composition
}

func NewCounter(name string) *Counter {
	return &Counter{
		name:  name,
		count: 0, // Initialize count to 0
		child: &Child{
			base64.StdEncoding.EncodeToString([]byte("Hi, I am a secret. Don't look at me!")),
		},
	}
}

func (e *Counter) Increment() {
	e.count++
}

func (e *Counter) GetCount() int {
	return e.count
}

func (e *Counter) GetChild() *Child {
	return e.child
}

func (e *Counter) Name() string {
	return e.name
}

func (e *Counter) String() string {
	return fmt.Sprintf("%s: %d", e.name, e.count)
}
