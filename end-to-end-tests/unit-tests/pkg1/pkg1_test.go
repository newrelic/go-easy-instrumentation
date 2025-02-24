package pkg1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	assert.Equal(t, 5, Add(2, 3))
	assert.Equal(t, 0, Add(-1, 1))
	assert.Equal(t, -5, Add(-2, -3))
}

func TestSubtract(t *testing.T) {
	assert.Equal(t, 1, Subtract(3, 2))
	assert.Equal(t, -2, Subtract(-1, 1))
	assert.Equal(t, 1, Subtract(-2, -3))
}

func TestFunc1(t *testing.T) {
	Func1()
}
