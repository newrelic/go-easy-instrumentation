package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoSomething(t *testing.T) {
	assert.NotPanics(t, DoSomething)
}
