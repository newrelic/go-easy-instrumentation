package integration

import (
	"testing"
	"unit-tests/pkg1"
	"unit-tests/pkg2"
	"unit-tests/pkg3"

	"unit-tests/util"

	"github.com/stretchr/testify/assert"
)

func TestIntegration(t *testing.T) {
	// Test integration between pkg1 and pkg2
	a := pkg1.Add(2, 3)
	a = pkg1.Subtract(a, 2)
	a = pkg2.Multiply(a, 6)
	a, err := pkg2.Divide(a, 2)
	assert.NoError(t, err)
	assert.Equal(t, 9, a)

	// Test integration between pkg3 and pkg4
	str := pkg3.Concat("hello", "world")
	split := pkg3.Split(str, "")
	assert.Equal(t, []string{"h", "e", "l", "l", "o", "w", "o", "r", "l", "d"}, split)

	util.DoSomething()
}
