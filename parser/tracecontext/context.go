package tracecontext

import (
	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comments"
)

const (
	ContextType = "context.Context"
)

// contextParameterType returns a new type object for a context argmuent
func contextParameterType() *dst.Ident {
	return &dst.Ident{
		Name: "Context",
		Path: "context",
	}
}

// isContextParam returns true if a field is a context.Context.
func isContextParam(arg *dst.Field) bool {
	ident, ok := arg.Type.(*dst.Ident)
	return ok && ident.Name == "Context" && ident.Path == "context"
}

type Context struct {
	contextVariableName     string
	transactionVariableName string
}

func NewContext(contextVariableName string) *Context {
	return &Context{
		contextVariableName: contextVariableName,
	}
}

func (ctx *Context) AssignTransactionVariable(variableName string) dst.Stmt {
	if ctx.transactionVariableName != "" {
		return nil
	}

	ctx.transactionVariableName = variableName
	return codegen.TxnFromContext(variableName, dst.NewIdent(ctx.contextVariableName))
}

// Pass will search for a context parameter in the function declaration, if found, it will add a wrapped context to the
// call expression at the index of the first known context parameter.
//
// Cases:
//
// The function declaration has a context parameter:
//  1. The function call does not have a context argument; we will append a context argument to the function call
//  2. The function call already has a context argument
//     Async Case:
//     a. the function call has a context with an async child transaction; do nothing
//     b. the function call has any other context; replace it with a context containing an async child transaction
//     Non-Async Case:
//     a. the function call context variable is equal to the context variable; do nothing
//     b. the function call context variable is not equal to the context variable; inject a transaction into it
func (ctx *Context) Pass(decl *dst.FuncDecl, call *dst.CallExpr, async bool) (TraceContext, error) {
	argumentIndex := 0
	for _, param := range decl.Type.Params.List {
		if isContextParam(param) {
			numParams := decl.Type.Params.NumFields()

			// The function declaration has a context parameter, but the function call does not have a context argument
			if len(call.Args) < numParams && argumentIndex == len(call.Args) {
				if async {
					call.Args = append(call.Args, codegen.WrapContextExpression(dst.NewIdent(ctx.contextVariableName), codegen.TxnNewGoroutine(dst.NewIdent(ctx.contextVariableName))))
				} else {
					call.Args = append(call.Args, dst.NewIdent(ctx.contextVariableName))
				}
			}

			// The function call already has a context argument
			if async {
				arg := call.Args[argumentIndex]
				// if this is async, check that the context argument contains a call to `txn.NewGoroutine`
				// since we know that is how we wrap async transactions in easy instrumentation
				if async && !codegen.ContainsTxnNewGoroutine(arg) {
					// if async, and does not contain a call to `txn.NewGoroutine`, we will wrap the context in a call to `txn.NewGoroutine` defensively
					arg = codegen.WrapContextExpression(arg, codegen.TxnNewGoroutine(codegen.TxnFromContextExpression(dst.NewIdent(ctx.contextVariableName))))
				} else {
					// check if the context here is the same as the context that was passed in with the transaction
					ident, ok := arg.(*dst.Ident)
					if !ok || ident.Name != ctx.contextVariableName {
						// if the context is not the same, we will inject the transaction into the context
						arg = codegen.WrapContextExpression(arg, codegen.TxnFromContextExpression(dst.NewIdent(ctx.contextVariableName)))
						comments.Info(call, "the context argument has been defensively injected with a transaction to ensure tracing is not lost.",
							"If the original context argument is a child of a context with a transaction, you may optionally undo this change for better readability.")
					}
				}
			}

			// the child function will use the context parameter
			return NewContext(param.Names[0].Name), nil
		}
		argumentIndex += incrementParameterCount(param)
	}

	// If we have made it this far, we did not find a context parameter in the function declaration, and should not expect one in the function call
	// We will add a context parameter to the function declaration
	decl.Type.Params.List = append(decl.Type.Params.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent(ctx.contextVariableName)},
		Type:  contextParameterType(),
	})

	var context dst.Expr
	context = dst.NewIdent(ctx.contextVariableName)
	if async {
		context = codegen.WrapContextExpression(context, codegen.TxnNewGoroutine(codegen.TxnFromContextExpression(context)))
	}
	call.Args = append(call.Args, context)
	return NewContext(ctx.contextVariableName), nil
}

// If AssignTransactionVariable has been called, this will return the variable name of the transaction.
func (ctx *Context) TransactionVariableName() string {
	return ctx.transactionVariableName
}

// Type returns the type of the context object: context.Context
func (ctx *Context) Type() string {
	return ContextType
}
