package codegen

import (
	"go/token"
	"reflect"
	"testing"

	"github.com/dave/dst"
)

func Test_injectRoundTripper(t *testing.T) {
	type args struct {
		clientVariable dst.Expr
		spacingAfter   dst.SpaceType
	}
	tests := []struct {
		name string
		args args
		want *dst.AssignStmt
	}{
		{
			name: "inject_roundtripper",
			args: args{
				clientVariable: &dst.Ident{Name: "client"},
				spacingAfter:   dst.NewLine,
			},
			want: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.SelectorExpr{
						X:   dst.Clone(&dst.Ident{Name: "client"}).(dst.Expr),
						Sel: dst.NewIdent("Transport"),
					},
				},
				Tok: token.ASSIGN,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "NewRoundTripper",
							Path: NewRelicAgentImportPath,
						},
						Args: []dst.Expr{
							&dst.SelectorExpr{
								X:   dst.Clone(&dst.Ident{Name: "client"}).(dst.Expr),
								Sel: dst.NewIdent("Transport"),
							},
						},
					},
				},
				Decs: dst.AssignStmtDecorations{
					NodeDecs: dst.NodeDecs{
						After: dst.NewLine,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RoundTripper(tt.args.clientVariable, tt.args.spacingAfter); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("injectRoundTripper() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_addTxnToRequestContext(t *testing.T) {
	type args struct {
		request  dst.Expr
		txnVar   dst.Expr
		nodeDecs *dst.NodeDecs
	}
	tests := []struct {
		name string
		args args
		want *dst.AssignStmt
	}{
		{
			name: "add_txn_to_request_context",
			args: args{
				request: &dst.Ident{
					Name: "r",
					Path: HttpImportPath,
				},
				txnVar: dst.NewIdent("txn"),
				nodeDecs: &dst.NodeDecs{
					Before: dst.NewLine,
					Start:  []string{"// this is a comment"},
				},
			},
			want: &dst.AssignStmt{
				Tok: token.ASSIGN,
				Lhs: []dst.Expr{dst.Clone(&dst.Ident{
					Name: "r",
					Path: HttpImportPath,
				}).(dst.Expr)},
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "RequestWithTransactionContext",
							Path: NewRelicAgentImportPath,
						},
						Args: []dst.Expr{
							dst.Clone(&dst.Ident{
								Name: "r",
								Path: HttpImportPath,
							}).(dst.Expr),
							dst.NewIdent("txn"),
						},
					},
				},
				Decs: dst.AssignStmtDecorations{
					NodeDecs: dst.NodeDecs{
						Before: dst.NewLine,
						Start:  []string{"// this is a comment"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WrapRequestContext(tt.args.request, tt.args.txnVar, tt.args.nodeDecs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("addTxnToRequestContext() = %v, want %v", got, tt.want)
			}
			if len(tt.args.nodeDecs.Start) != 0 {
				t.Errorf("should clear the End decorations slice but did NOT")
			}
			if tt.args.nodeDecs.Before != dst.None {
				t.Errorf("should set the Before decorations slice to \"None\" but it was %s", tt.args.nodeDecs.Before.String())
			}
		})
	}
}
