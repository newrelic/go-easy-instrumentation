package util

import (
	"strings"

	"github.com/dave/dst"
)

func DebugPrint(node dst.Node) string {
	objString := strings.Builder{}
	_ = dst.Fprint(&objString, node, dst.NotNilFilter)
	return objString.String()
}
