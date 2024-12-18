package util

import (
	"strings"

	"github.com/dave/dst"
)

// DebugPrint returns a string representation of the given node.
// This is useful for debugging purposes, and will pretty print
// the structure of a node in human readable form.
//
// Do Not Use: This function is only for debugging purposes.
func DebugPrint(node dst.Node) string {
	objString := strings.Builder{}
	_ = dst.Fprint(&objString, node, dst.NotNilFilter)
	return objString.String()
}
