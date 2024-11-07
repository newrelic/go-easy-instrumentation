package traceobject

import (
	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
)

const (
	transactionType = "*newrelic.Transaction"
)

type Transaction struct {
	transactionVariable string
}

func NewTransaction() *Transaction {
	return &Transaction{}
}

func transactionReturn() AddToCallReturn {
	return AddToCallReturn{
		TraceObject: NewTransaction(),
		Import:      codegen.NewRelicAgentImportPath,
		NeedsTx:     true,
	}
}

func isTransactionArgument(arg dst.Expr, transactionVariable string) bool {
	ident, ok := arg.(*dst.Ident)
	if !ok {
		return false
	}

	if ident.Name == transactionVariable {
		return true
	}

	return false
}

// AddToCall adds the transaction as an argument to the function call.
// If async is true, the transaction will cloned by calling NewGoroutine().
func (txn *Transaction) AddToCall(pkg *decorator.Package, call *dst.CallExpr, transactionVariable string, async bool) AddToCallReturn {
	for i, arg := range call.Args {
		typ := util.TypeOf(arg, pkg)

		// check if this function contains a transaction argument
		if isTransactionArgument(arg, transactionVariable) {
			return transactionReturn()
		}

		// if the call already contains a context, inject a transaction into it rather than adding an argument
		if typ != nil && typ.String() == contextType {
			call.Args[i] = codegen.WrapContextExpression(arg, transactionVariable, async)
			return AddToCallReturn{
				TraceObject: NewContext(),
				Import:      codegen.NewRelicAgentImportPath,
				NeedsTx:     true,
			}
		}
	}

	// if we got this far, we did not find a suitable arguent in the function call
	// so we need to add it
	var transactionExpr dst.Expr
	transactionExpr = dst.NewIdent(transactionVariable)
	if async {
		transactionExpr = codegen.TxnNewGoroutine(transactionExpr)
	}

	call.Args = append(call.Args, transactionExpr)
	return transactionReturn()
}

func getTracingParameter(pkg *decorator.Package, params []*dst.Field) (TraceObject, string) {
	for _, param := range params {
		ident, ok := param.Type.(*dst.Ident)
		if ok && ident.Name == "Transaction" && ident.Path == codegen.NewRelicAgentImportPath {
			return NewTransaction(), codegen.NewRelicAgentImportPath
		}

		typ := util.TypeOf(param.Type, pkg)
		if typ != nil && typ.String() == contextType {
			return &Context{contextParameterName: param.Names[0].Name}, ""
		}
	}

	return nil, ""
}

// AddToFuncDecl adds the transaction as a parameter to the function declaration
func (txn *Transaction) AddToFuncDecl(pkg *decorator.Package, decl *dst.FuncDecl) (TraceObject, string) {
	obj, goGet := getTracingParameter(pkg, decl.Type.Params.List)
	if obj != nil {
		return obj, goGet
	}

	decl.Type.Params.List = append(decl.Type.Params.List, codegen.NewTransactionParameter(codegen.DefaultTransactionVariable))
	return txn, codegen.NewRelicAgentImportPath
}

// AddToFuncLit adds the transaction as a parameter to the function literal
func (txn *Transaction) AddToFuncLit(pkg *decorator.Package, lit *dst.FuncLit) (TraceObject, string) {
	obj, goGet := getTracingParameter(pkg, lit.Type.Params.List)
	if obj != nil {
		return obj, goGet
	}

	lit.Type.Params.List = append(lit.Type.Params.List, codegen.NewTransactionParameter(codegen.DefaultTransactionVariable))
	return txn, codegen.NewRelicAgentImportPath
}

// This is already a transaction
// no-op
func (txn *Transaction) AssignTransactionVariable(variableName string) (dst.Stmt, string) {
	return nil, ""
}
