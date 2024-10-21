package tracecontext

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
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
	pkg                 *decorator.Package
	agentVariable       string
	transactionVariable string
}

// StartTransaction creates a new transaction and returns the transaction and the code to start the transaction.
// The return values can never be nil.
// If overwrite is true, the variable will be assigned instead of defined.
func StartTransaction(pkg *decorator.Package, variableName, transactionName, agentVariable string, overwrite bool) (*Transaction, dst.Stmt) {
	tc := &Transaction{
		transactionVariable: variableName,
		agentVariable:       agentVariable,
		pkg:                 pkg,
	}
	stmt := codegen.StartTransaction(agentVariable, variableName, transactionName, overwrite)
	return tc, stmt
}

// NewTransaction creates a new transaction object.
func NewTransaction(variableName string, pkg *decorator.Package) *Transaction {
	return &Transaction{
		pkg:                 pkg,
		transactionVariable: variableName,
	}
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
func (t *Transaction) Pass(decl *dst.FuncDecl, call *dst.CallExpr, async bool) TraceContext {
	var txn dst.Expr

	// if async, create an async transaction
	txn = dst.NewIdent(t.transactionVariable)
	if async {
		txn = codegen.TxnNewGoroutine(dst.NewIdent(t.transactionVariable))
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
				if !codegen.ContainsWrapContextExpression(arg) {
					call.Args[argumentIndex] = codegen.WrapContextExpression(arg, txn)
				}
			}
			return NewContext(param.Names[0].Name, t.pkg)

		} else if isTransactionParam(param) {
			// this will always be the last argument, so check to make sure we have not already added it
			// applications already using the go agent are not supported
			numParams := decl.Type.Params.NumFields()
			if len(call.Args) < numParams && argumentIndex == len(call.Args) {
				call.Args = append(call.Args, txn)
			}

			// return the trace context for the subprocess
			return NewTransaction(param.Names[0].Name, t.pkg)
		}

		argumentIndex += incrementParameterCount(param)
	}

	// if we reach this point, we have not found a context or transaction parameter
	// add a transaction parameter to the function declaration
	decl.Type.Params.List = append(decl.Type.Params.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent(t.transactionVariable)},
		Type:  transactionArgumentType(),
	})

	call.Args = append(call.Args, txn)
	return NewTransaction(t.transactionVariable, t.pkg)
}

// Transaction returns the variable name of the transaction.
func (t *Transaction) Transaction() (string, dst.Stmt) {
	return t.transactionVariable, nil
}

// Transaction returns the variable name of the transaction.
// If the agent has not yet been assigned to a variable, a line of code to do that will be returned as the second return value.
// This code must be inserted before the current cursor index.
func (t *Transaction) Agent() (string, dst.Stmt) {
	if t.agentVariable != "" {
		return t.agentVariable, nil
	}

	stmt := codegen.GetApplication(dst.NewIdent(t.transactionVariable), codegen.DefaultAgentVariableName)
	fmt.Println(stmt)
	t.agentVariable = codegen.DefaultAgentVariableName
	return t.agentVariable, stmt
}

// Type returns a *newrelic.Transaction type
func (t *Transaction) Type() string {
	return TransactionType
}
