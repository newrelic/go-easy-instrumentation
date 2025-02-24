package pkg3

import (
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
