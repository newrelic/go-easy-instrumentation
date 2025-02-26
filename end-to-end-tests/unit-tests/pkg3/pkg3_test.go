package pkg3

import (
	"context"
	"testing"
	"unit-tests/util"

	"github.com/stretchr/testify/assert"
)

func TestConcat(t *testing.T) {
	assert.Equal(t, "hello world", Concat("hello ", "world"))
	assert.Equal(t, "foo", Concat("f", "oo"))
	assert.Equal(t, "barbaz", Concat("bar", "baz"))

	util.DoSomething()
}

func TestSplit(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, Split("a,b,c", ","))
	assert.Equal(t, []string{"foo", "bar"}, Split("foo bar", " "))
	assert.Equal(t, []string{"hello", "world"}, Split("hello-world", "-"))
}

func TestThingWithContext(t *testing.T) {
	assert.Equal(t, ThingWithContext(context.Background()), true)
}

func TestUnrulyFunction(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		ctx     context.Context
		want    string
		wantErr bool
	}{
		{
			name:    "valid input",
			a:       "hello",
			b:       "world",
			ctx:     context.Background(),
			want:    "helloworld",
			wantErr: false,
		},
		{
			name:    "empty a",
			a:       "",
			b:       "world",
			ctx:     context.Background(),
			want:    "",
			wantErr: true,
		},
		{
			name:    "contains error",
			a:       "hello",
			b:       "error",
			ctx:     context.Background(),
			want:    "",
			wantErr: true,
		},
		{
			name:    "context check failed",
			a:       "hello",
			b:       "world",
			ctx:     context.TODO(),
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnrulyFunction(tt.a, tt.b, tt.ctx) // this should not get modified
			if (err != nil) != tt.wantErr {
				if err == nil {
					return
				}
				t.Errorf("UnrulyFunction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCrazyFunction(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       int
		c       []string
		d       map[string]int
		e       struct{ X, Y int }
		ctx     context.Context
		want    string
		wantErr bool
	}{
		{
			name:    "valid input",
			a:       "hello",
			b:       42,
			c:       []string{"foo", "bar"},
			d:       map[string]int{"baz": 1, "qux": 2},
			e:       struct{ X, Y int }{X: 3, Y: 4},
			ctx:     context.Background(),
			want:    "hello42foobarbaz1qux234",
			wantErr: false,
		},
		{
			name:    "empty a",
			a:       "",
			b:       42,
			c:       []string{"foo", "bar"},
			d:       map[string]int{"baz": 1, "qux": 2},
			e:       struct{ X, Y int }{X: 3, Y: 4},
			ctx:     context.Background(),
			want:    "",
			wantErr: true,
		},
		{
			name:    "negative b",
			a:       "hello",
			b:       -1,
			c:       []string{"foo", "bar"},
			d:       map[string]int{"baz": 1, "qux": 2},
			e:       struct{ X, Y int }{X: 3, Y: 4},
			ctx:     context.Background(),
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty c",
			a:       "hello",
			b:       42,
			c:       []string{},
			d:       map[string]int{"baz": 1, "qux": 2},
			e:       struct{ X, Y int }{X: 3, Y: 4},
			ctx:     context.Background(),
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty d",
			a:       "hello",
			b:       42,
			c:       []string{"foo", "bar"},
			d:       map[string]int{},
			e:       struct{ X, Y int }{X: 3, Y: 4},
			ctx:     context.Background(),
			want:    "",
			wantErr: true,
		},
		{
			name:    "zero e",
			a:       "hello",
			b:       42,
			c:       []string{"foo", "bar"},
			d:       map[string]int{"baz": 1, "qux": 2},
			e:       struct{ X, Y int }{X: 0, Y: 0},
			ctx:     context.Background(),
			want:    "",
			wantErr: true,
		},
		{
			name:    "context check failed",
			a:       "hello",
			b:       42,
			c:       []string{"foo", "bar"},
			d:       map[string]int{"baz": 1, "qux": 2},
			e:       struct{ X, Y int }{X: 3, Y: 4},
			ctx:     context.TODO(),
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CrazyFunction(tt.a, tt.b, tt.c, tt.d, tt.e, tt.ctx)
			if (err != nil) != tt.wantErr {
				if err == nil {
					return
				}
				t.Errorf("CrazyFunction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
