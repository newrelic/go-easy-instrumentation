package nrecho_test

import (
	"go/token"
	"reflect"
	"testing"

	"github.com/newrelic/go-easy-instrumentation/integrations/nrecho-v3"

	"github.com/dave/dst"
)

func TestNrEchoMiddleware(t *testing.T) {
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
			name: "inject_nrecho_middleware",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "New",
							Path: nrecho.EchoImportPath,
						},
					},
				},
				routerName:        "e",
				agentVariableName: &dst.Ident{Name: "NewRelicApplication"},
			},
			want: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "e",
						},
						Sel: &dst.Ident{
							Name: "Use",
						},
					},
					Args: []dst.Expr{
						&dst.CallExpr{
							Fun: &dst.Ident{
								Name: "Middleware",
								Path: nrecho.NrechoImportPath,
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
			got, imp := nrecho.NrEchoMiddleware(tt.args.routerName, tt.args.agentVariableName)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nrecho.NrEchoMiddleware() = %v, want %v", got, tt.want)
			}
			if imp != nrecho.NrechoImportPath {
				t.Errorf("nrecho.NrEchoMiddleware() = %v, want %v", imp, nrecho.NrechoImportPath)
			}
		})
	}
}

func TestTxnFromEchoContext(t *testing.T) {
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
			name: "assign txn from echo context",
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
							Name: "FromContext",
							Path: nrecho.NrechoImportPath,
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
			if got := nrecho.TxnFromEchoContext(tt.args.txnVariable, tt.args.ctxName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nrecho.TxnFromEchoContext() = %v, want %v", got, tt.want)
			}
		})
	}
}
