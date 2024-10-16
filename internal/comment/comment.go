package comment

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

const (
	InfoHeader string = "NR INFO"
	WarnHeader string = "NR WARN"
)

// Info appends a new relic info comment to the node.
// This function is used to add comments that will be written to the generated code.
// The message is the main comment, and additionalInfo is a list of optional
// comments that will be printed on new lines below the main comment.
func Info(pkg *decorator.Package, node dst.Node, message string, additionalInfo ...string) {
	comments := []string{
		fmt.Sprintf("// %s: %s", InfoHeader, message),
	}
	for _, info := range additionalInfo {
		comments = append(comments, fmt.Sprintf("// %s", info))
	}

	decs := node.Decorations()
	if len(decs.Start) > 0 {
		comments = append(comments, "//")
	}

	decs.Start.Prepend(comments...)
	printer.Add(pkg, node, InfoHeader, message, additionalInfo...)
}

func Warn(pkg *decorator.Package, node dst.Node, message string, additionalInfo ...string) {
	comments := []string{
		fmt.Sprintf("// %s: %s", WarnHeader, message),
	}
	for _, info := range additionalInfo {
		comments = append(comments, fmt.Sprintf("// %s", info))
	}

	decs := node.Decorations()
	if len(decs.Start) > 0 {
		comments = append(comments, "//")
	}

	decs.Start.Prepend(comments...)
	printer.Add(pkg, node, WarnHeader, message, additionalInfo...)
}
