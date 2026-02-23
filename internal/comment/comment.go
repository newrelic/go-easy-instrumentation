// comment is a library that provides a generalized way to provide feedback to the user about the state of their code
// and our ability to instrument it. It does this primarily by adding comments in the diff file that either provide
// information or warnings to the user. It also provides a way to print this information to the console if the user
// enables debug mode.
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

	DebugConsoleHeader string = "Debug"
)

func writeComment(node dst.Node, comments []string) {
	decs := node.Decorations()
	if len(decs.Start) > 0 {
		comments = append(comments, "//")
	}
	decs.Start.Prepend(comments...)
}

// Info appends a comment to a node that alerts a user to a non-critical issue in their code.
// It also adds the comment to the console printer if it is enabled.
//
// The message is the main comment, and additionalInfo is a list of optional new lines that will
// be added to the comment. The positionNode is the node where the issue is occurring. The commentNode
// is the node where the comment will be added, for readability purposes, these may not be the same node.
func Info(pkg *decorator.Package, commentNode dst.Node, positionNode dst.Node, message string, additionalInfo ...string) {
	comments := []string{
		fmt.Sprintf("// %s: %s", InfoHeader, message),
	}
	for _, info := range additionalInfo {
		comments = append(comments, fmt.Sprintf("// %s", info))
	}

	writeComment(commentNode, comments)
	printer.add(pkg, positionNode, InfoConsoleHeader, message, additionalInfo...)
}

// Warn appends a comment to a node that alerts a user to an important issue in their code.
// It also adds the comment to the console printer if it is enabled.
//
// The message is the main comment, and additionalInfo is a list of optional new lines that will
// be added to the comment. The positionNode is the node where the issue is occurring. The commentNode
// is the node where the comment will be added, for readability purposes, these may not be the same node.
func Warn(pkg *decorator.Package, commentNode dst.Node, positionNode dst.Node, message string, additionalInfo ...string) {
	comments := []string{
		fmt.Sprintf("// %s: %s", WarnHeader, message),
	}
	for _, info := range additionalInfo {
		comments = append(comments, fmt.Sprintf("// %s", info))
	}

	writeComment(commentNode, comments)
	printer.add(pkg, positionNode, WarnConsoleHeader, message, additionalInfo...)
}

// Debug logs a message to the console only when debug mode is enabled.
// Unlike Info and Warn, this does NOT write any comments to the diff file.
//
// The message is the main log line, and additionalInfo is a list of optional
// new lines that will be printed below the main message. The positionNode
// is used to display the source location of the relevant code.
func Debug(pkg *decorator.Package, positionNode dst.Node, message string, additionalInfo ...string) {
	printer.add(pkg, positionNode, DebugConsoleHeader, message, additionalInfo...)
}
