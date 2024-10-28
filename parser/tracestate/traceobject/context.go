package traceobject

import (
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
)

const (
	contextType = "context.Context"
)

type Context struct{}

func NewContext() *Context {
	return &Context{}
}

func (ctx *Context) AddToCall(pkg *decorator.Package, call *dst.CallExpr, contextVariable string, async bool) string {
	for _, arg := range call.Args {
		typ := util.TypeOf(arg, pkg)
		if typ != nil && typ.String() == contextType {
			return ""
		}
	}

	// if we got this far, we did not find a suitable arguent in the function call
	// so we need to add it
	call.Args = append(call.Args, &dst.Ident{Name: contextVariable})
	return ""
}
