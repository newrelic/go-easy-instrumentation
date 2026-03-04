package pkg1

import "fmt"

func Func1() {
	fmt.Println("Func1 in pkg1")
}

func Add(a, b int) int {
	return a + b
}

func Subtract(a, b int) int {
	return a - b
}
