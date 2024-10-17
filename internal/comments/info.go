package comments

import (
	"fmt"

	"github.com/dave/dst"
)

const nrInfo = "NR INFO"

// Info appends a new relic info comment to the node.
func Info(node dst.Node, message string, additionalInfo ...string) {
	headers := []string{
		fmt.Sprintf("// %s: %s", nrInfo, message),
	}
	for _, info := range additionalInfo {
		headers = append(headers, fmt.Sprintf("// %s", info))
	}

	decs := node.Decorations()
	if len(decs.Start) > 0 {
		headers = append(headers, "//")
	}

	decs.Start.Prepend(headers...)
}
