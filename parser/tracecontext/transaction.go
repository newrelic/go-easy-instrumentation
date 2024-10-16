package tracecontext

import (
	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
)

const (
	TransactionType = "*newrelic.Transaction"
)

func transactionArgumentType() *dst.StarExpr {
	return &dst.StarExpr{
		X: &dst.Ident{
			Name: "Transaction",
			Path: "newrelic",
		},
	}
}

func isTransactionParam(arg *dst.Field) bool {
	star, ok := arg.Type.(*dst.StarExpr)
	if ok {
		ident, ok := star.X.(*dst.Ident)
		return ok && ident.Name == "Transaction" && ident.Path == "newrelic"
	}
	return false
}

// Transaction is an object that represents a new relic transaction object.
type Transaction struct {
	variableName string
}

// StartTransaction creates a new transaction and returns the transaction and the code to start the transaction.
// The return values can never be nil.
// If overwrite is true, the variable will be assigned instead of defined.
func StartTransaction(variableName, transactionName, agentVariable string, overwrite bool) (*Transaction, dst.Stmt) {
	return &Transaction{variableName: variableName}, codegen.StartTransaction(agentVariable, variableName, transactionName, overwrite)
}

// NewTransaction creates a new transaction object.
func NewTransaction(variableName string) *Transaction {
	return &Transaction{
		variableName: variableName,
	}
}

// AssignTransactionToVariable returns nil for transactions because the transaction is already assigned to a variable.
func (t *Transaction) AssignTransactionVariable(variableName string) dst.Stmt {
	return nil
}

// Pass Transaction will search for a context or transaction parameter in the function declaration.
// If found, it will attempt to pass the transaction to the function call, write any necessary code before the call, and
// return the trace context for the child function call.
//
// The Pass function for Transactions preferrs to pass by context.Context if a function already accepts one, but will otherwise pass by transaction.
// We should always assume that AddParam has been called on the declaration.
//
// Address the following cases:
//  1. The function declaration has a context parameter; we will inject the transaction into the passed context argument
//  2. The function declaration has a transaction parameter;
//     a. The function call does not have a transaction argument; we will append a transaction argument to the function call
//     b. The function call already has a transaction argument; do nothing
//  3. The function declaration does not have a context or transaction parameter; we will append a transaction argument to the function declaration and the function call
func (t *Transaction) Pass(decl *dst.FuncDecl, call *dst.CallExpr, async bool) (TraceContext, error) {
	var txn dst.Expr

	// if async, create an async transaction
	txn = dst.NewIdent(t.variableName)
	if async {
		txn = codegen.TxnNewGoroutine(dst.NewIdent(t.variableName))
	}

	// argumentIndex counts the number of arguments seen so far so we can compare the types
	// agains the types of the function declaration.
	// This is a shortcut that would save us a lot of type checking.
	argumentIndex := 0
	for _, param := range decl.Type.Params.List {
		// if the function declaration has a context argument, we will inject the transaction into it
		if isContextParam(param) {
			if argumentIndex < len(call.Args) {
				// update the context argument to include the transaction
				arg := call.Args[argumentIndex]
				if async {
					arg = codegen.WrapContextExpression(arg, codegen.TxnNewGoroutine(dst.NewIdent(t.variableName)))
				} else {
					arg = codegen.WrapContextExpression(arg, dst.NewIdent(t.variableName))
				}
			}
			// return the context trace context for the child function
			return NewContext(param.Names[0].Name), nil

		} else if isTransactionParam(param) {
			// this will always be the last argument, so check to make sure we have not already added it
			// applications already using the go agent are not supported
			numParams := decl.Type.Params.NumFields()
			if len(call.Args) < numParams && argumentIndex == len(call.Args) {
				if async {
					call.Args = append(call.Args, codegen.TxnNewGoroutine(dst.NewIdent(t.variableName)))
				} else {
					call.Args = append(call.Args, dst.NewIdent(t.variableName))
				}
			}

			// return the trace context for the subprocess
			return NewTransaction(param.Names[0].Name), nil
		}

		argumentIndex += incrementParameterCount(param)
	}

	// if we reach this point, we have not found a context or transaction parameter
	// add a transaction parameter to the function declaration
	decl.Type.Params.List = append(decl.Type.Params.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent(t.variableName)},
		Type:  transactionArgumentType(),
	})

	call.Args = append(call.Args, txn)
	return NewTransaction(t.variableName), nil
}

// TransactionVariableName returns the variable name of the transaction.
func (t *Transaction) TransactionVariableName() string {
	return t.variableName
}

// Type returns a *newrelic.Transaction type
func (t *Transaction) Type() string {
	return TransactionType
}
