package comment

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
)

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
		path.WriteString(strconv.Itoa(pos.Column))
	}

	if pos.Column != 0 {
		path.WriteByte(':')
		path.WriteString(strconv.Itoa(pos.Line))
	}

	return path.String()
}
