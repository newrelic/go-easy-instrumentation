package tracecontext

import "github.com/dave/dst"

// increment by each param name, since this is the true count of how many params it takes in
// https://cs.opensource.google/go/go/+/refs/tags/go1.23.2:src/go/ast/ast.go;l=261
func incrementParameterCount(param *dst.Field) int {
	if len(param.Names) == 0 {
		return 1
	}
	return len(param.Names)
}
