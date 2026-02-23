package comment

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
)

// getPosition creates a human readable string representing the position of a node in an application.
// In order to improve readability, the filename will be localized to the root of the application.
// The format of the string is as follows based on the positional info available:
//
// Info 					|		Formatting
// ------------------------------------------------------------------
// filename,line, column	|	filename line:column
// filename, line			|	filename line
// filename, column			|	filename
// filename					|	filename
// invalid or empty			|	""
func getPosition(pkg *decorator.Package, node dst.Node, appRoot string) string {
	pos := util.Position(node, pkg)
	if pos == nil || !pos.IsValid() {
		return ""
	}

	split := strings.Split(pos.Filename, string(filepath.Separator))
	path := strings.Builder{}
	for _, segment := range split {
		if path.Len() != 0 {
			path.WriteByte(filepath.Separator)
			path.WriteString(segment)
		}
		if segment == appRoot {
			path.WriteString(segment)
		}
	}

	if pos.Line != 0 {
		path.WriteByte(' ')
		path.WriteString(strconv.Itoa(pos.Line))
		if pos.Column != 0 {
			path.WriteByte(':')
			path.WriteString(strconv.Itoa(pos.Column))
		}
	}

	return path.String()
}
