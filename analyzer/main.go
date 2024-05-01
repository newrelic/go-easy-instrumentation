package main

import (
	"go-agent-analyzer/analyzer"

	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	multichecker.Main(analyzer.Analyzer)
}
