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

// Context is a trace object that contains a context.Context object.
// Note: this will not work for structured objects that contain a context.Context object.
type Context struct {
	contextParameterName string // the name of the context parameter for a function declaration
}

func NewContext(name ...string) *Context {
	if len(name) > 0 {
		return &Context{contextParameterName: name[0]}
	}
	return &Context{}
}

func (ctx *Context) AddToCall(pkg *decorator.Package, call *dst.CallExpr, transactionVariableName string, async bool) AddToCallReturn {
	for i, arg := range call.Args {
		typ := util.TypeOf(arg, pkg)
		if typ != nil && typ.String() == contextType {
			ident, ok := arg.(*dst.Ident)
			if ok {
				// the context we know has a transaction is being passed to the function call
				if ident.Name == ctx.contextParameterName {
					if async {
						call.Args[i] = codegen.WrapContextExpression(arg, transactionVariableName, async)
						return AddToCallReturn{
							TraceObject: NewContext(),
							Import:      codegen.NewRelicAgentImportPath,
							NeedsTx:     true,
						}
					}
					return AddToCallReturn{
						TraceObject: NewContext(),
						Import:      "",
						NeedsTx:     false,
					}
				}

				// if the context variable being passed is different from the one we know has a transaction
				// pass a transaction to it defensively.
				argumentString := util.WriteExpr(arg, pkg)
				comment.Info(pkg, call, call,
					fmt.Sprintf("a transaction was added to to the context argument %s to ensure a transaction is passed to the function call", argumentString),
					fmt.Sprintf("This may not be necessary, and can be safely removed if this context %s is a child of %s", argumentString, util.WriteExpr(dst.NewIdent(ctx.contextParameterName), pkg)),
				)

				call.Args[i] = codegen.WrapContextExpression(arg, transactionVariableName, async)
				return AddToCallReturn{
					TraceObject: NewContext(),
					Import:      codegen.NewRelicAgentImportPath,
					NeedsTx:     true,
				}
			}
		}
	}

	// if we got this far, we did not find a suitable arguent in the function call
	// so we need to add it
	var transactionExpr dst.Expr
	transactionExpr = dst.NewIdent(transactionVariableName)
	if async {
		transactionExpr = codegen.TxnNewGoroutine(transactionExpr)
	}

	call.Args = append(call.Args, transactionExpr)
	return transactionReturn()
}

func (ctx *Context) AddToFuncDecl(pkg *decorator.Package, decl *dst.FuncDecl) (TraceObject, string) {
	obj, goGet := getTracingParameter(pkg, decl.Type.Params.List)
	if obj != nil {
		return obj, goGet
	}

	// append a transaction if we dont find a context parameter
	decl.Type.Params.List = append(decl.Type.Params.List, codegen.NewTransactionParameter(codegen.DefaultTransactionVariable))
	return NewTransaction(), codegen.NewRelicAgentImportPath
}

func (ctx *Context) AddToFuncLit(pkg *decorator.Package, lit *dst.FuncLit) (TraceObject, string) {
	obj, goGet := getTracingParameter(pkg, lit.Type.Params.List)
	if obj != nil {
		return obj, goGet
	}

	lit.Type.Params.List = append(lit.Type.Params.List, codegen.NewTransactionParameter(codegen.DefaultTransactionVariable))
	return NewTransaction(), codegen.NewRelicAgentImportPath
}

func (ctx *Context) AssignTransactionVariable(variableName string) (dst.Stmt, string) {
	return codegen.TxnFromContext(variableName, dst.NewIdent(ctx.contextParameterName)), ""
}
