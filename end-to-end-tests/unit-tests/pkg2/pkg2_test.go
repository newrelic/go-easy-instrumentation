package pkg2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMultiply(t *testing.T) {
	assert.Equal(t, 6, Multiply(2, 3))
	assert.Equal(t, 0, Multiply(0, 1))
	assert.Equal(t, -6, Multiply(-2, 3))
}

func TestDivide(t *testing.T) {
	result, err := Divide(6, 3)
	assert.NoError(t, err)
	assert.Equal(t, 2, result)

	_, err = Divide(1, 0)
	assert.Error(t, err)
}
