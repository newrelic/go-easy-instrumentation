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
			if got := NrGinMiddleware(tt.args.call, tt.args.routerName, tt.args.agentVariableName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NrGinMiddleware() = %v, want %v", got, tt.want)
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
				Decs: dst.AssignStmtDecorations{
					NodeDecs: dst.NodeDecs{
						After: dst.EmptyLine,
					},
				},
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

func Test_DeferStartSegment(t *testing.T) {
	type args struct {
		txnVariable string
		route       string
	}
	tests := []struct {
		name string
		args args
		want *dst.DeferStmt
	}{
		{
			name: "defer start segment",
			args: args{
				txnVariable: "txn",
				route:       `"api/route"`,
			},
			want: &dst.DeferStmt{
				Call: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.CallExpr{
							Fun: &dst.SelectorExpr{
								X: &dst.Ident{
									Name: "txn",
								},
								Sel: &dst.Ident{
									Name: "StartSegment",
								},
							},
							Args: []dst.Expr{
								&dst.BasicLit{
									Kind:  token.STRING,
									Value: `"api/route"`,
								},
							},
						},
						Sel: &dst.Ident{
							Name: "End",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeferStartSegment(tt.args.txnVariable, tt.args.route); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeferStartSegment() = %v, want %v", got, tt.want)
			}
		})
	}
}
