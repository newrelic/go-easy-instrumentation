package pkg2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocking an external dependency
type MockExternalService struct {
	mock.Mock
}

func (m *MockExternalService) multiply(a, b int) int {
	args := m.Called(a, b)
	return args.Int(0)
}

func TestMockedMultiply(t *testing.T) {
	mockService := new(MockExternalService)
	mockService.On("Multiply", 2, 3).Return(6)

	result := mockService.multiply(2, 3)
	assert.Equal(t, 6, result)

	mockService.AssertExpectations(t)
}

func setupMultTest(t *testing.T, a, b, expect int) {
	t.Helper()
	assert.Equal(t, expect, Multiply(a, b))
}

func TestMultiply(t *testing.T) {
	setupMultTest(t, 2, 3, 6)
	assert.Equal(t, 36, Multiply(6, 6))
}

func BenchmarkMultiply(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Multiply(2, 3)
	}
}

func TestDivide(t *testing.T) {
	result, err := Divide(6, 3)
	assert.NoError(t, err)
	assert.Equal(t, 2, result)

	_, err = Divide(1, 0)
	assert.Error(t, err)
}
