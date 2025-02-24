package pkg3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConcat(t *testing.T) {
	assert.Equal(t, "hello world", Concat("hello ", "world"))
	assert.Equal(t, "foo", Concat("f", "oo"))
	assert.Equal(t, "barbaz", Concat("bar", "baz"))
}

func TestSplit(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, Split("a,b,c", ","))
	assert.Equal(t, []string{"foo", "bar"}, Split("foo bar", " "))
	assert.Equal(t, []string{"hello", "world"}, Split("hello-world", "-"))
}
