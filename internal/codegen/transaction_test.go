package codegen

import (
	"go/token"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func Test_startTransaction(t *testing.T) {
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

func Test_endTransaction(t *testing.T) {
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

func Test_NewTransactionParameter(t *testing.T) {
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

func Test_txnNewGoroutine(t *testing.T) {
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

func Test_generateNoticeError(t *testing.T) {
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
