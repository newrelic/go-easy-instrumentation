package util

import "fmt"

// This exists to test what happens to functions that are just for tests
// we want to see this get ignored.
func DoSomething() {
	// do something
	fmt.Println("do something")
}
