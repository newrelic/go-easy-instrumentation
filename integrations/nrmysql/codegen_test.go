package nrmysql_test

import (
	"github.com/newrelic/go-easy-instrumentation/integrations/nrmysql"
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func Test_CreateSQLTransaction(t *testing.T) {
	tests := []struct {
		name           string
		agentVarName   string
		txnVarName     string
		sqlMethodName  string
		wantTxnName    string
		wantTxnVarName string
	}{
		{
			name:           "creates SQL transaction for QueryRow",
			agentVarName:   "app",
			txnVarName:     "nrTxn",
			sqlMethodName:  "QueryRow",
			wantTxnName:    "mySQL/QueryRow",
			wantTxnVarName: "nrTxn",
		},
		{
			name:           "creates SQL transaction for Exec",
			agentVarName:   "myApp",
			txnVarName:     "sqlTxn",
			sqlMethodName:  "Exec",
			wantTxnName:    "mySQL/Exec",
			wantTxnVarName: "sqlTxn",
		},
		{
			name:           "creates SQL transaction for Query",
			agentVarName:   "application",
			txnVarName:     "txn",
			sqlMethodName:  "Query",
			wantTxnName:    "mySQL/Query",
			wantTxnVarName: "txn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nrmysql.CreateSQLTransaction(tt.agentVarName, tt.txnVarName, tt.sqlMethodName)

			// Check it's an assignment statement with DEFINE token
			assert.NotNil(t, got)
			assert.Equal(t, token.DEFINE, got.Tok)

			// Check LHS
			assert.Len(t, got.Lhs, 1)
			lhsIdent, ok := got.Lhs[0].(*dst.Ident)
			assert.True(t, ok, "expected Lhs[0] to be *dst.Ident")
			assert.Equal(t, tt.wantTxnVarName, lhsIdent.Name)

			// Check RHS is a call expression
			assert.Len(t, got.Rhs, 1)
			rhsCall, ok := got.Rhs[0].(*dst.CallExpr)
			assert.True(t, ok, "expected Rhs[0] to be *dst.CallExpr")

			// Check the call is app.StartTransaction
			selExpr, ok := rhsCall.Fun.(*dst.SelectorExpr)
			assert.True(t, ok, "expected Fun to be *dst.SelectorExpr")

			xIdent, ok := selExpr.X.(*dst.Ident)
			assert.True(t, ok, "expected X to be *dst.Ident")
			assert.Equal(t, tt.agentVarName, xIdent.Name)
			assert.Equal(t, "StartTransaction", selExpr.Sel.Name)

			// Check the argument is the transaction name
			assert.Len(t, rhsCall.Args, 1)
			argLit, ok := rhsCall.Args[0].(*dst.BasicLit)
			assert.True(t, ok, "expected Args[0] to be *dst.BasicLit")
			assert.Equal(t, token.STRING, argLit.Kind)
			assert.Equal(t, `"`+tt.wantTxnName+`"`, argLit.Value)
		})
	}
}

func Test_CreateContextWithTransaction(t *testing.T) {
	tests := []struct {
		name       string
		ctxName    string
		txnName    string
		wantCtxVar string
		wantTxnVar string
	}{
		{
			name:       "creates context with transaction",
			ctxName:    "ctx",
			txnName:    "nrTxn",
			wantCtxVar: "ctx",
			wantTxnVar: "nrTxn",
		},
		{
			name:       "creates context with custom names",
			ctxName:    "myContext",
			txnName:    "myTransaction",
			wantCtxVar: "myContext",
			wantTxnVar: "myTransaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nrmysql.CreateContextWithTransaction(tt.ctxName, tt.txnName)

			// Check it's an assignment statement with DEFINE token
			assert.NotNil(t, got)
			assert.Equal(t, token.DEFINE, got.Tok)

			// Check LHS is the context variable
			assert.Len(t, got.Lhs, 1)
			lhsIdent, ok := got.Lhs[0].(*dst.Ident)
			assert.True(t, ok, "expected Lhs[0] to be *dst.Ident")
			assert.Equal(t, tt.wantCtxVar, lhsIdent.Name)

			// Check RHS is a call to newrelic.NewContext
			assert.Len(t, got.Rhs, 1)
			rhsCall, ok := got.Rhs[0].(*dst.CallExpr)
			assert.True(t, ok, "expected Rhs[0] to be *dst.CallExpr")

			// Check the function is newrelic.NewContext
			funSelExpr, ok := rhsCall.Fun.(*dst.SelectorExpr)
			assert.True(t, ok, "expected Fun to be *dst.SelectorExpr")

			funX, ok := funSelExpr.X.(*dst.Ident)
			assert.True(t, ok, "expected X to be *dst.Ident")
			assert.Equal(t, "newrelic", funX.Name)
			assert.Equal(t, "NewContext", funSelExpr.Sel.Name)

			// Check arguments: context.Background() and transaction
			assert.Len(t, rhsCall.Args, 2)

			// First arg: context.Background()
			arg0Call, ok := rhsCall.Args[0].(*dst.CallExpr)
			assert.True(t, ok, "expected Args[0] to be *dst.CallExpr")

			arg0SelExpr, ok := arg0Call.Fun.(*dst.SelectorExpr)
			assert.True(t, ok, "expected Args[0] Fun to be *dst.SelectorExpr")

			arg0X, ok := arg0SelExpr.X.(*dst.Ident)
			assert.True(t, ok, "expected Args[0] X to be *dst.Ident")
			assert.Equal(t, "context", arg0X.Name)
			assert.Equal(t, "Background", arg0SelExpr.Sel.Name)

			// Second arg: transaction variable
			arg1Ident, ok := rhsCall.Args[1].(*dst.Ident)
			assert.True(t, ok, "expected Args[1] to be *dst.Ident")
			assert.Equal(t, tt.wantTxnVar, arg1Ident.Name)
		})
	}
}

func Test_CreateTransactionEnd(t *testing.T) {
	tests := []struct {
		name       string
		txnName    string
		wantTxnVar string
	}{
		{
			name:       "creates transaction end statement",
			txnName:    "nrTxn",
			wantTxnVar: "nrTxn",
		},
		{
			name:       "creates transaction end statement with custom name",
			txnName:    "myTransaction",
			wantTxnVar: "myTransaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nrmysql.CreateTransactionEnd(tt.txnName)

			// Check it's an expression statement
			assert.NotNil(t, got)

			// Check the expression is a call
			call, ok := got.X.(*dst.CallExpr)
			assert.True(t, ok, "expected X to be *dst.CallExpr")

			// Check the call is txn.End()
			selExpr, ok := call.Fun.(*dst.SelectorExpr)
			assert.True(t, ok, "expected Fun to be *dst.SelectorExpr")

			xIdent, ok := selExpr.X.(*dst.Ident)
			assert.True(t, ok, "expected X to be *dst.Ident")
			assert.Equal(t, tt.wantTxnVar, xIdent.Name)
			assert.Equal(t, "End", selExpr.Sel.Name)

			// Check no arguments are passed
			assert.Len(t, call.Args, 0)
		})
	}
}
