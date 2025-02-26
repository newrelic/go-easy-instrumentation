package pkg3

import (
	"context"
	"fmt"
	"strings"
)

func Func3() {
	fmt.Println("Func3 in pkg3")
}

func Concat(a, b string) string {
	return a + b
}

func Split(s, sep string) []string {
	return strings.Split(s, sep)
}

func ThingWithContext(ctx context.Context) bool {
	return true
}

// UnrulyFunction is a complex function to test DST code
func UnrulyFunction(a, b string, ctx context.Context) (string, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in UnrulyFunction", r)
		}
	}()

	if a == "" {
		return "", fmt.Errorf("a is empty")
	}

	result := Concat(a, b)
	parts := Split(result, " ")

	for _, part := range parts {
		if part == "error" {
			return "", fmt.Errorf("found error in parts")
		}
	}

	if !ThingWithContext(ctx) {
		return "", fmt.Errorf("context check failed")
	}

	return result, nil
}

// CrazyFunction is another complex function to test DST code
func CrazyFunction(a string, b int, c []string, d map[string]int, e struct{ X, Y int }, ctx context.Context) (string, error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in CrazyFunction", r)
		}
	}()

	if a == "" {
		return "", fmt.Errorf("a is empty")
	}

	if b < 0 {
		return "", fmt.Errorf("b is negative")
	}

	if len(c) == 0 {
		return "", fmt.Errorf("c is empty")
	}

	if len(d) == 0 {
		return "", fmt.Errorf("d is empty")
	}

	if e.X == 0 && e.Y == 0 {
		return "", fmt.Errorf("e is zero")
	}

	if !ThingWithContext(ctx) {
		return "", fmt.Errorf("context check failed")
	}

	result := Concat(a, fmt.Sprintf("%d", b))
	for _, s := range c {
		result = Concat(result, s)
	}

	for k, v := range d {
		result = Concat(result, fmt.Sprintf("%s%d", k, v))
	}

	result = Concat(result, fmt.Sprintf("%d%d", e.X, e.Y))

	return result, nil
}
