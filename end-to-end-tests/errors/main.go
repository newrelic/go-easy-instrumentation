package main

import (
	"bytes"
	"fmt"
	"io"
)

func foo() error {
	return fmt.Errorf("foo error")
}

func myFunction(r io.Reader) bool {
	// we want to see that this error gets captured in the foo function, not here
	err := foo()
	if err != nil {
		return false
	}

	// this error should be captured here
	_, err = fmt.Fscan(r)
	if err != nil {
		return false
	}

	// this error should be captured here in the body of this if statement
	if err = fmt.Errorf("oopsiedoodle"); err != nil {
		return false
	}

	return true
}

func main() {
	buf := &bytes.Buffer{}
	buf.WriteString("hello this is a test")
	if myFunction(buf) {
		fmt.Println("myFunction returned true")
	}
}
