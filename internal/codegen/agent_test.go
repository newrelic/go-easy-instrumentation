package codegen

import (
	"go/token"
	"reflect"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func Test_InitializeAgent(t *testing.T) {
	type args struct {
		AppName           string
		AgentVariableName string
	}
	tests := []struct {
		name string
		args args
		want []dst.Stmt
	}{
		{
			name: "Test create agent AST",
			args: args{
				AgentVariableName: "testAgent",
			},
			want: []dst.Stmt{&dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "testAgent",
					},
					&dst.Ident{
						Name: agentErrorVariableName,
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "NewApplication",
							Path: NewRelicAgentImportPath,
						},
						Args: []dst.Expr{
							&dst.CallExpr{
								Fun: &dst.Ident{
									Path: NewRelicAgentImportPath,
									Name: "ConfigFromEnvironment",
								},
							},
						},
					},
				},
			}, panicOnError(agentErrorVariableName)},
		},
		{
			name: "Test create agent AST with AppName",
			args: args{
				AgentVariableName: "testAgent",
				AppName:           "testApp",
			},
			want: []dst.Stmt{&dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "testAgent",
					},
					&dst.Ident{
						Name: agentErrorVariableName,
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "NewApplication",
							Path: NewRelicAgentImportPath,
						},
						Args: []dst.Expr{
							&dst.CallExpr{
								Fun: &dst.Ident{
									Path: NewRelicAgentImportPath,
									Name: "ConfigAppName",
								},
								Args: []dst.Expr{
									&dst.BasicLit{
										Kind:  token.STRING,
										Value: `"testApp"`,
									},
								},
							},
							&dst.CallExpr{
								Fun: &dst.Ident{
									Path: NewRelicAgentImportPath,
									Name: "ConfigFromEnvironment",
								},
							},
						},
					},
				},
			}, panicOnError(agentErrorVariableName)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, InitializeAgent(tt.args.AppName, tt.args.AgentVariableName))
		})
	}
}

func Test_shutdownAgent(t *testing.T) {
	type args struct {
		AgentVariableName string
	}
	tests := []struct {
		name string
		args args
		want *dst.ExprStmt
	}{
		{
			name: "Test shutdown agent",
			args: args{
				AgentVariableName: "testAgent",
			},
			want: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "testAgent",
						},
						Sel: &dst.Ident{
							Name: "Shutdown",
						},
					},
					Args: []dst.Expr{
						&dst.BinaryExpr{
							X: &dst.BasicLit{
								Kind:  token.INT,
								Value: "5",
							},
							Op: token.MUL,
							Y: &dst.Ident{
								Name: "Second",
								Path: "time",
							},
						},
					},
				},
				Decs: dst.ExprStmtDecorations{
					NodeDecs: dst.NodeDecs{
						Before: dst.EmptyLine,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShutdownAgent(tt.args.AgentVariableName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_panicOnError(t *testing.T) {
	tests := []struct {
		name string
		want *dst.IfStmt
	}{
		{
			name: "Test panic on error",
			want: &dst.IfStmt{
				Cond: &dst.BinaryExpr{
					X: &dst.Ident{
						Name: "err",
					},
					Op: token.NEQ,
					Y: &dst.Ident{
						Name: "nil",
					},
				},
				Body: &dst.BlockStmt{
					List: []dst.Stmt{
						&dst.ExprStmt{
							X: &dst.CallExpr{
								Fun: &dst.Ident{
									Name: "panic",
								},
								Args: []dst.Expr{
									&dst.Ident{
										Name: "err",
									},
								},
							},
						},
					},
				},
				Decs: dst.IfStmtDecorations{
					NodeDecs: dst.NodeDecs{
						After: dst.EmptyLine,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := panicOnError("err"); !reflect.DeepEqual(got, tt.want) {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
