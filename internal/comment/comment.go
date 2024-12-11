package comment

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

const (
	InfoHeader        string = "NR INFO"
	InfoConsoleHeader string = "Info"

	WarnHeader        string = "NR WARN"
	WarnConsoleHeader string = "Warn"
)

func writeComment(node dst.Node, comments []string) {
	decs := node.Decorations()
	if len(decs.Start) > 0 {
		comments = append(comments, "//")
	}
	decs.Start.Prepend(comments...)
}

// Info appends a new relic info comment to the node.
// This function is used to add comments that will be written to the generated code.
// The message is the main comment, and additionalInfo is a list of optional
// comments that will be printed on new lines below the main comment.
func Info(pkg *decorator.Package, commentNode dst.Node, positionNode dst.Node, message string, additionalInfo ...string) {
	comments := []string{
		fmt.Sprintf("// %s: %s", InfoHeader, message),
	}
	for _, info := range additionalInfo {
		comments = append(comments, fmt.Sprintf("// %s", info))
	}

	writeComment(commentNode, comments)
	printer.Add(pkg, positionNode, InfoConsoleHeader, message, additionalInfo...)
}

func Warn(pkg *decorator.Package, commentNode dst.Node, positionNode dst.Node, message string, additionalInfo ...string) {
	comments := []string{
		fmt.Sprintf("// %s: %s", WarnHeader, message),
	}
	for _, info := range additionalInfo {
		comments = append(comments, fmt.Sprintf("// %s", info))
	}

	writeComment(commentNode, comments)
	printer.
		Add(pkg, positionNode, WarnConsoleHeader, message, additionalInfo...)
}
