package codegen

import (
	"reflect"
	"testing"

	"github.com/dave/dst"
)

const (
	ChiImportPath = "github.com/go-chi/chi/v5"
)

func Test_NrChiMiddleware(t *testing.T) {
	type args struct {
		call              *dst.CallExpr
		routerName        string
		agentVariableName dst.Expr
	}

	type test struct {
		name string
		args args
		want *dst.ExprStmt
	}

	tests := []test{
		{
			name: "inject_nrgochi_middleware",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "NewRouter",
							Path: ChiImportPath,
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
								Path: NrChiImportPath,
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
			got, imp := NrChiMiddleware(tt.args.routerName, tt.args.agentVariableName)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NrChiMiddleware() = %v, want %v", got, tt.want)
			}
			if imp != NrChiImportPath {
				t.Errorf("NrChiMiddleware() = %v, want %v", imp, NrChiImportPath)
			}
		})
	}
}
