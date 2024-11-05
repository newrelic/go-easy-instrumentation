package traceobject

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
)

const (
	contextType = "context.Context"
)

// contextParameterType returns a new type object for a context argmuent
func contextParameterType() *dst.Ident {
	return &dst.Ident{
		Name: "Context",
		Path: "context",
	}
}

type Context struct {
	contextVariable string
}

func NewContext() *Context {
	return &Context{}
}

func (ctx *Context) AddToCall(pkg *decorator.Package, call *dst.CallExpr, contextVariable string, async bool) (TraceObject, string) {
	for i, arg := range call.Args {
		typ := util.TypeOf(arg, pkg)
		if typ != nil && typ.String() == contextType {
			ident, ok := arg.(*dst.Ident)
			if ok {
				if ident.Name == ctx.contextVariable {
					return NewContext(), ""
				}

				// if the context variable being passed is different from the one we know has a transaction
				// pass a transaction to it defensively.
				call.Args[i] = codegen.TransferTransactionToContext(dst.NewIdent(ctx.contextVariable), arg)
				argumentString := util.WriteExpr(arg, pkg)
				comment.Info(pkg, call, fmt.Sprintf("A transaction was added to to the context argument %s to ensure a transaction is passed to the function call", argumentString), fmt.Sprintf("This may not be necessary, and can be safely removed if this context %s is a child of %s", argumentString, util.WriteExpr(dst.NewIdent(ctx.contextVariable), pkg)))
				return nil, codegen.NewRelicAgentImportPath
			}
		}
	}

	// if we got this far, we did not find a suitable arguent in the function call
	// so we need to add it
	call.Args = append(call.Args, &dst.Ident{Name: contextVariable})
	return NewContext(), ""
}

func getContextParameter(pkg *decorator.Package, params []*dst.Field) (TraceObject, string) {
	for _, param := range params {
		ident, ok := param.Type.(*dst.Ident)
		if ok && ident.Name == "Transaction" && ident.Path == codegen.NewRelicAgentImportPath {
			return NewTransaction(), codegen.NewRelicAgentImportPath
		}

		typ := util.TypeOf(param.Type, pkg)
		if typ != nil && typ.String() == contextType {
			ctx := NewContext()
			ctx.contextVariable = param.Names[0].Name
			return ctx, ""
		}
	}

	return nil, ""
}

func (ctx *Context) AddToFuncDecl(pkg *decorator.Package, decl *dst.FuncDecl) (TraceObject, string) {
	obj, goGet := getContextParameter(pkg, decl.Type.Params.List)
	if obj != nil {
		return obj, goGet
	}

	decl.Type.Params.List = append(decl.Type.Params.List, codegen.ContextParameter(codegen.DefaultContextParameter))
	ctx.contextVariable = codegen.DefaultContextParameter
	return ctx, ""
}

func (ctx *Context) AddToFuncLit(pkg *decorator.Package, lit *dst.FuncLit) (TraceObject, string) {
	obj, goGet := getContextParameter(pkg, lit.Type.Params.List)
	if obj != nil {
		return obj, goGet
	}

	lit.Type.Params.List = append(lit.Type.Params.List, codegen.ContextParameter(codegen.DefaultContextParameter))
	ctx.contextVariable = codegen.DefaultContextParameter
	return ctx, ""
}

func (ctx *Context) AssignTransactionVariable(variableName string) (dst.Stmt, string) {
	return codegen.TxnFromContext(variableName, dst.NewIdent(ctx.contextVariable)), ""
}
