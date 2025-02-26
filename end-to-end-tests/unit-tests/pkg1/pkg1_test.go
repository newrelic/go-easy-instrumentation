package pkg1

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	assert.Equal(t, 5, Add(2, 3))
	assert.Equal(t, 0, Add(-1, 1))
	assert.Equal(t, -5, Add(-2, -3))
}

func TestAddTable(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{2, 3, 5},
		{-1, 1, 0},
		{-2, -3, -5},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Add(%d,%d)", tt.a, tt.b), func(t *testing.T) {
			assert.Equal(t, tt.expected, Add(tt.a, tt.b))
		})
	}
}

func TestSubtract(t *testing.T) {
	assert.Equal(t, 1, Subtract(3, 2))
	assert.Equal(t, -2, Subtract(-1, 1))
	assert.Equal(t, 1, Subtract(-2, -3))
}

func TestFunc1(t *testing.T) {
	Func1()
}
