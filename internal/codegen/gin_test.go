package codegen

import (
	"go/token"
	"reflect"
	"testing"

	"github.com/dave/dst"
)

func Test_NrGinMiddleware(t *testing.T) {
	type args struct {
		call              *dst.CallExpr
		routerName        string
		agentVariableName dst.Expr
	}
	tests := []struct {
		name string
		args args
		want *dst.ExprStmt
	}{
		{
			name: "inject_nrgin_middleware",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "Default",
							Path: GinImportPath,
						},
					},
				},
				routerName:        "router",
				agentVariableName: &dst.Ident{Name: "NewRelicApplication"},
			},
			want: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "router",
						},
						Sel: &dst.Ident{
							Name: "Use",
						},
					},
					Args: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{
								Name: "Middleware",
								Path: NrginImportPath,
							},
							Args: []dst.Expr{
								&dst.Ident{Name: "NewRelicApplication"},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, imp := NrGinMiddleware(tt.args.routerName, tt.args.agentVariableName)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NrGinMiddleware() = %v, want %v", got, tt.want)
			}
			if imp != NrginImportPath {
				t.Errorf("NrGinMiddleware() = %v, want %v", imp, NrginImportPath)
			}
		})
	}
}

func Test_TxnFromGinContext(t *testing.T) {
	type args struct {
		txnVariable string
		ctxName     string
	}
	tests := []struct {
		name string
		args args
		want *dst.AssignStmt
	}{
		{
			name: "assign txn from gin context",
			args: args{
				txnVariable: "nrTxn",
				ctxName:     "c",
			},
			want: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "nrTxn",
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "Transaction",
							Path: NrginImportPath,
						},
						Args: []dst.Expr{
							&dst.Ident{
								Name: "c",
							},
						},
					},
				},
				Decs: dst.AssignStmtDecorations{
					NodeDecs: dst.NodeDecs{
						Before: dst.NewLine,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TxnFromGinContext(tt.args.txnVariable, tt.args.ctxName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TxnFromGinContext() = %v, want %v", got, tt.want)
			}
		})
	}
}
