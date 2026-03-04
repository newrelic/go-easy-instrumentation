package codegen

import (
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func TestStartTransaction(t *testing.T) {
	type args struct {
		appVariableName         string
		transactionVariableName string
		transactionName         string
		overwriteVariable       bool
	}
	tests := []struct {
		name string
		args args
		want *dst.AssignStmt
	}{
		{
			name: "Test start transaction",
			args: args{
				appVariableName:         "testApp",
				transactionVariableName: "testTxn",
				transactionName:         "testTxnName",
				overwriteVariable:       false,
			},
			want: &dst.AssignStmt{
				Lhs: []dst.Expr{dst.NewIdent("testTxn")},
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Args: []dst.Expr{
							&dst.BasicLit{
								Kind:  token.STRING,
								Value: `"testTxnName"`,
							},
						},
						Fun: &dst.SelectorExpr{
							X:   dst.NewIdent("testApp"),
							Sel: dst.NewIdent("StartTransaction"),
						},
					},
				},
				Tok: token.DEFINE,
			},
		},
		{
			name: "Test start transaction with overwrite",
			args: args{
				appVariableName:         "testApp",
				transactionVariableName: "testTxn",
				transactionName:         "testTxnName",
				overwriteVariable:       true,
			},
			want: &dst.AssignStmt{
				Lhs: []dst.Expr{dst.NewIdent("testTxn")},
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Args: []dst.Expr{
							&dst.BasicLit{
								Kind:  token.STRING,
								Value: `"testTxnName"`,
							},
						},
						Fun: &dst.SelectorExpr{
							X:   dst.NewIdent("testApp"),
							Sel: dst.NewIdent("StartTransaction"),
						},
					},
				},
				Tok: token.ASSIGN,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StartTransaction(tt.args.appVariableName, tt.args.transactionVariableName, tt.args.transactionName, tt.args.overwriteVariable)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEndTransaction(t *testing.T) {
	type args struct {
		transactionVariableName string
	}
	tests := []struct {
		name string
		args args
		want *dst.ExprStmt
	}{
		{
			name: "Test end transaction",
			args: args{
				transactionVariableName: "testTxn",
			},
			want: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent("testTxn"),
						Sel: dst.NewIdent("End"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EndTransaction(tt.args.transactionVariableName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewTransactionParameter(t *testing.T) {
	type args struct {
		txnName string
	}
	tests := []struct {
		name string
		args args
		want *dst.Field
	}{
		{
			name: "Test txn as parameter",
			args: args{
				txnName: "testTxn",
			},
			want: &dst.Field{
				Names: []*dst.Ident{
					{
						Name: "testTxn",
					},
				},
				Type: &dst.StarExpr{
					X: &dst.Ident{
						Name: "Transaction",
						Path: NewRelicAgentImportPath,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewTransactionParameter(tt.args.txnName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTxnNewGoroutine(t *testing.T) {
	type args struct {
		txnVarName string
	}
	tests := []struct {
		name string
		args args
		want *dst.CallExpr
	}{
		{
			name: "Test txn new goroutine",
			args: args{
				txnVarName: "testTxn",
			},
			want: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X: &dst.Ident{
						Name: "testTxn",
					},
					Sel: &dst.Ident{
						Name: "NewGoroutine",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TxnNewGoroutine(dst.NewIdent(tt.args.txnVarName))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateNoticeError(t *testing.T) {
	type args struct {
		errExpr  dst.Expr
		txnName  string
		nodeDecs *dst.NodeDecs
	}
	tests := []struct {
		name string
		args args
		want *dst.ExprStmt
	}{
		{
			name: "generate notice error",
			args: args{
				errExpr:  dst.NewIdent("err"),
				txnName:  "txn",
				nodeDecs: nil,
			},
			want: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "txn",
						},
						Sel: &dst.Ident{
							Name: "NoticeError",
						},
					},
					Args: []dst.Expr{
						&dst.Ident{
							Name: "err",
						},
					},
				},
			},
		},
		{
			name: "generate notice error with end decorations",
			args: args{
				errExpr: dst.NewIdent("err"),
				txnName: "txn",
				nodeDecs: &dst.NodeDecs{
					After: dst.NewLine,
					End:   dst.Decorations{"// end"},
				},
			},
			want: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "txn",
						},
						Sel: &dst.Ident{
							Name: "NoticeError",
						},
					},
					Args: []dst.Expr{
						&dst.Ident{
							Name: "err",
						},
					},
				},
				Decs: dst.ExprStmtDecorations{
					NodeDecs: dst.NodeDecs{
						After: dst.NewLine,
						End:   dst.Decorations{"// end"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			stmtBlock := &dst.ExprStmt{}
			if tt.args.nodeDecs != nil {
				stmtBlock = &dst.ExprStmt{
					Decs: dst.ExprStmtDecorations{
						NodeDecs: *tt.args.nodeDecs,
					},
				}
			}

			got := NoticeError(tt.args.errExpr, dst.NewIdent(tt.args.txnName), stmtBlock)
			if tt.args.nodeDecs != nil {
				assert.Equal(t, tt.args.nodeDecs.After, stmtBlock.Decs.After, "whitespace after stmtblock should not be modified")
				assert.Equal(t, tt.args.nodeDecs.End, stmtBlock.Decs.End, "comment after stmtblock should not be modified")
				assert.Equal(t, tt.args.nodeDecs.Before, got.Decs.Before, "generated notice error statement should inherit before decorations from stmtblock")
				assert.Equal(t, tt.args.nodeDecs.Start, got.Decs.Start, "generated notice error statement should inherit start decorations from stmtblock")

				assert.Equal(t, dst.None, stmtBlock.Decs.Before, "whitespace before stmtblock should be none")
				assert.Equal(t, dst.Decorations(nil), stmtBlock.Decs.Start, "comments before stmtblock should be empty")

			}
		})
	}
}

func TestGetApplication(t *testing.T) {
	tests := []struct {
		name                          string
		transactionVariableExpression dst.Expr
		wantTxnVarName                string
	}{
		{
			name:                          "generates Application() call",
			transactionVariableExpression: dst.NewIdent("txn"),
			wantTxnVarName:                "txn",
		},
		{
			name:                          "generates Application() call with custom name",
			transactionVariableExpression: dst.NewIdent("myTransaction"),
			wantTxnVarName:                "myTransaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetApplication(tt.transactionVariableExpression)

			// Check it's a call expression
			call, ok := got.(*dst.CallExpr)
			assert.True(t, ok, "expected result to be *dst.CallExpr")

			// Check it's a selector expression
			selExpr, ok := call.Fun.(*dst.SelectorExpr)
			assert.True(t, ok, "expected Fun to be *dst.SelectorExpr")

			// Check X is the transaction variable
			xIdent, ok := selExpr.X.(*dst.Ident)
			assert.True(t, ok, "expected X to be *dst.Ident")
			assert.Equal(t, tt.wantTxnVarName, xIdent.Name)

			// Check selector is "Application"
			assert.Equal(t, "Application", selExpr.Sel.Name)

			// Check no arguments
			assert.Len(t, call.Args, 0)
		})
	}
}

func TestTxnFromContext(t *testing.T) {
	tests := []struct {
		name          string
		txnVariable   string
		contextObject dst.Expr
		wantTxnVar    string
		wantCtxVar    string
	}{
		{
			name:          "generates txn from context assignment",
			txnVariable:   "txn",
			contextObject: dst.NewIdent("ctx"),
			wantTxnVar:    "txn",
			wantCtxVar:    "ctx",
		},
		{
			name:          "generates txn from context with custom names",
			txnVariable:   "nrTxn",
			contextObject: dst.NewIdent("requestContext"),
			wantTxnVar:    "nrTxn",
			wantCtxVar:    "requestContext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TxnFromContext(tt.txnVariable, tt.contextObject)

			// Check it's an assignment statement with DEFINE token
			assert.NotNil(t, got)
			assert.Equal(t, token.DEFINE, got.Tok)

			// Check decorations include empty line after
			assert.Equal(t, dst.EmptyLine, got.Decs.After)

			// Check LHS is the transaction variable
			assert.Len(t, got.Lhs, 1)
			lhsIdent, ok := got.Lhs[0].(*dst.Ident)
			assert.True(t, ok, "expected Lhs[0] to be *dst.Ident")
			assert.Equal(t, tt.wantTxnVar, lhsIdent.Name)

			// Check RHS is a call to newrelic.FromContext
			assert.Len(t, got.Rhs, 1)
			rhsCall, ok := got.Rhs[0].(*dst.CallExpr)
			assert.True(t, ok, "expected Rhs[0] to be *dst.CallExpr")

			// Check the function is FromContext from newrelic package
			funIdent, ok := rhsCall.Fun.(*dst.Ident)
			assert.True(t, ok, "expected Fun to be *dst.Ident")
			assert.Equal(t, "FromContext", funIdent.Name)
			assert.Equal(t, NewRelicAgentImportPath, funIdent.Path)

			// Check the argument is the context object
			assert.Len(t, rhsCall.Args, 1)
			argIdent, ok := rhsCall.Args[0].(*dst.Ident)
			assert.True(t, ok, "expected Args[0] to be *dst.Ident")
			assert.Equal(t, tt.wantCtxVar, argIdent.Name)
		})
	}
}

func TestTxnFromContextExpression(t *testing.T) {
	tests := []struct {
		name          string
		contextObject dst.Expr
		wantCtxVar    string
	}{
		{
			name:          "generates FromContext expression",
			contextObject: dst.NewIdent("ctx"),
			wantCtxVar:    "ctx",
		},
		{
			name:          "generates FromContext expression with custom name",
			contextObject: dst.NewIdent("requestContext"),
			wantCtxVar:    "requestContext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TxnFromContextExpression(tt.contextObject)

			// Check it's a call expression
			call, ok := got.(*dst.CallExpr)
			assert.True(t, ok, "expected result to be *dst.CallExpr")

			// Check the function is FromContext from newrelic package
			funIdent, ok := call.Fun.(*dst.Ident)
			assert.True(t, ok, "expected Fun to be *dst.Ident")
			assert.Equal(t, "FromContext", funIdent.Name)
			assert.Equal(t, NewRelicAgentImportPath, funIdent.Path)

			// Check the argument is the context object
			assert.Len(t, call.Args, 1)
			argIdent, ok := call.Args[0].(*dst.Ident)
			assert.True(t, ok, "expected Args[0] to be *dst.Ident")
			assert.Equal(t, tt.wantCtxVar, argIdent.Name)
		})
	}
}
