package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func buildGetRequest(path string) (*http.Request, error) {
	req, err := http.NewRequest("GET", path, nil)
	if err == io.EOF {
		fmt.Println("io eof")
	} else if err == io.ErrClosedPipe {
		fmt.Println("nothing")
	} else if err != nil {
		errMsg := fmt.Sprintf("failed to build request: %v", err)
		slog.Error(errMsg)

	}
	return req, nil
}

func testConditionalError() {
	yes := true
	_, err := http.NewRequest("GET", "/", nil)
	if err != nil && yes {
		fmt.Println("not nill")
	}

	yes2 := true
	_, err2 := http.NewRequest("GET", "/", nil)
	if yes2 && err2 != nil {
		fmt.Println("not nill")
	}

	_, err3 := http.NewRequest("GET", "/", nil)
	if err3 != nil && yes2 && yes {
		fmt.Println("not nill")
	}
}

func testDoubleErrors() {
	_, err := http.NewRequest("GET", "/", nil)
	if err == io.EOF {
		fmt.Println("io eof")
	}
	if err != nil {
		fmt.Println("not nill")
	}
}

func testAdvancedError() {
	type testStruct struct {
		err error
	}
	test := testStruct{}
	yes := true
	_, test.err = http.NewRequest("GET", "/", nil)
	if test.err == io.EOF {
		fmt.Println("io eof")
	} else if test.err == io.ErrClosedPipe {
		fmt.Println("nothing")
	} else if test.err != nil && yes {
		errMsg := fmt.Sprintf("failed to build request: %v", test.err)
		slog.Error(errMsg)

	}
}
func checkError(err error) {
	if err != nil {
		fmt.Println("not nill")
	}
}

func errorBeingCalled() {
	_, err := http.NewRequest("GET", "/", nil)
	checkError(err)
}

func unusedError() {
	_, err := http.NewRequest("GET", "/", nil)
	fmt.Println(err)
}

func main() {
	buildGetRequest("hi")
	testDoubleErrors()
	testConditionalError()
	testAdvancedError()
	errorBeingCalled()
	unusedError()
}
