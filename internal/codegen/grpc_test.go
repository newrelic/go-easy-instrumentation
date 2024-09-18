package codegen

import (
	"go/token"
	"reflect"
	"testing"

	"github.com/dave/dst"
)

func Test_getCallExpressionArgumentSpacing(t *testing.T) {
	type args struct {
		call *dst.CallExpr
	}
	tests := []struct {
		name string
		args args
		want dst.NodeDecs
	}{
		{
			name: "calls with 0 arguments",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "NewServer",
						Path: GrpcImportPath,
					},
					Args: []dst.Expr{},
				},
			},
			want: dst.NodeDecs{
				After:  dst.None,
				Before: dst.None,
			},
		},
		{
			name: "calls with 1 argument",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "NewServer",
						Path: GrpcImportPath,
					},
					Args: []dst.Expr{
						&dst.BasicLit{
							Kind:  token.STRING,
							Value: `"localhost:8080"`,
						},
					},
				},
			},
			want: dst.NodeDecs{
				After:  dst.NewLine,
				Before: dst.None,
			},
		},
		{
			name: "calls with many arguments follow existing spacing rules: no newlines",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "NewServer",
						Path: GrpcImportPath,
					},
					Args: []dst.Expr{
						&dst.BasicLit{
							Kind:  token.STRING,
							Value: `"localhost:8080"`,
							Decs: dst.BasicLitDecorations{
								NodeDecs: dst.NodeDecs{
									After:  dst.None,
									Before: dst.None,
								},
							},
						},
						dst.NewIdent("grpc.Creds"),
					},
				},
			},
			want: dst.NodeDecs{
				After:  dst.None,
				Before: dst.None,
			},
		},
		{
			name: "calls with many arguments follow existing spacing rules: newlines",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "NewServer",
						Path: GrpcImportPath,
					},
					Args: []dst.Expr{
						&dst.BasicLit{
							Kind:  token.STRING,
							Value: `"localhost:8080"`,
							Decs: dst.BasicLitDecorations{
								NodeDecs: dst.NodeDecs{
									After:  dst.NewLine,
									Before: dst.NewLine,
								},
							},
						},
						&dst.Ident{
							Name: "grpc.Creds",
							Decs: dst.IdentDecorations{
								NodeDecs: dst.NodeDecs{
									After: dst.NewLine,
								},
							},
						},
					},
				},
			},
			want: dst.NodeDecs{
				After: dst.NewLine,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCallExpressionArgumentSpacing(tt.args.call); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getCallExpressionArgumentSpacing() = %v, want %v", got, tt.want)
			}
			if len(tt.args.call.Args) == 1 {
				if tt.args.call.Args[0].Decorations().After != dst.NewLine {
					t.Errorf("expected the existing spacing After to be overwritten with %v; got %v", dst.NewLine, tt.args.call.Args[0].Decorations().After)
				}
				if tt.args.call.Args[0].Decorations().Before != dst.NewLine {
					t.Errorf("expected the existing spacing Before to be overwritten with %v; got %v", dst.NewLine, tt.args.call.Args[0].Decorations().Before)
				}
			}
		})
	}
}
