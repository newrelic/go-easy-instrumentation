package codegen

import (
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func TestIfErrorNotNilNoticeError(t *testing.T) {
	tests := []struct {
		name                string
		errorVariable       dst.Expr
		transactionVariable dst.Expr
		wantErrorVarName    string
		wantTxnVarName      string
	}{
		{
			name:                "creates if err != nil with NoticeError",
			errorVariable:       dst.NewIdent("err"),
			transactionVariable: dst.NewIdent("txn"),
			wantErrorVarName:    "err",
			wantTxnVarName:      "txn",
		},
		{
			name:                "creates if error != nil with custom names",
			errorVariable:       dst.NewIdent("myError"),
			transactionVariable: dst.NewIdent("myTxn"),
			wantErrorVarName:    "myError",
			wantTxnVarName:      "myTxn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IfErrorNotNilNoticeError(tt.errorVariable, tt.transactionVariable)

			// Check it's an if statement
			assert.NotNil(t, got)

			// Check the condition is err != nil
			binExpr, ok := got.Cond.(*dst.BinaryExpr)
			assert.True(t, ok, "expected Cond to be *dst.BinaryExpr")
			assert.Equal(t, token.NEQ, binExpr.Op)

			// Check X is the error variable
			xIdent, ok := binExpr.X.(*dst.Ident)
			assert.True(t, ok, "expected X to be *dst.Ident")
			assert.Equal(t, tt.wantErrorVarName, xIdent.Name)

			// Check Y is nil
			yIdent, ok := binExpr.Y.(*dst.Ident)
			assert.True(t, ok, "expected Y to be *dst.Ident")
			assert.Equal(t, "nil", yIdent.Name)

			// Check the body contains NoticeError call
			assert.NotNil(t, got.Body)
			assert.Len(t, got.Body.List, 1)

			exprStmt, ok := got.Body.List[0].(*dst.ExprStmt)
			assert.True(t, ok, "expected body statement to be *dst.ExprStmt")

			call, ok := exprStmt.X.(*dst.CallExpr)
			assert.True(t, ok, "expected X to be *dst.CallExpr")

			// Check the call is txn.NoticeError(err)
			selExpr, ok := call.Fun.(*dst.SelectorExpr)
			assert.True(t, ok, "expected Fun to be *dst.SelectorExpr")

			txnIdent, ok := selExpr.X.(*dst.Ident)
			assert.True(t, ok, "expected X to be *dst.Ident")
			assert.Equal(t, tt.wantTxnVarName, txnIdent.Name)
			assert.Equal(t, "NoticeError", selExpr.Sel.Name)

			// Check the error is passed as argument
			assert.Len(t, call.Args, 1)
			errArg, ok := call.Args[0].(*dst.Ident)
			assert.True(t, ok, "expected Args[0] to be *dst.Ident")
			assert.Equal(t, tt.wantErrorVarName, errArg.Name)
		})
	}
}
