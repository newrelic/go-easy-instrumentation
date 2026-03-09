package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndex(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(index)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "hello world", rr.Body.String())
}

func TestAnotherFuncWithAContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), "key", "value")
	anotherFuncWithAContext(ctx)
	// Since this function only logs, we can't assert much here without a logging framework that supports testing
}

func TestAFunctionWithContextArguments(t *testing.T) {
	ctx := context.Background()
	aFunctionWithContextArguments(ctx)
	// Since this function calls other functions, we would need to mock those to assert behavior
}

func TestDoAThing(t *testing.T) {
	t.Run("without error", func(t *testing.T) {
		result, success, err := DoAThing(false)
		assert.NoError(t, err)
		assert.Equal(t, "thing complete", result)
		assert.True(t, success)
	})

	t.Run("with error", func(t *testing.T) {
		result, success, err := DoAThing(true)
		assert.Error(t, err)
		assert.Equal(t, "thing not done", result)
		assert.False(t, success)
	})
}

func TestNoticeError(t *testing.T) {
	req, err := http.NewRequest("GET", "/error", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(noticeError)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "this is an error", rr.Body.String())
}
