package main

import (
	"log"

	"github.com/newrelic/go-easy-instrumentation/cmd"
)

func main() {
	log.Default().SetFlags(0)
	cmd.Execute()
}
