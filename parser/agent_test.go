package main

import (
	"go/token"
	"reflect"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

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
			if got := panicOnError(); !reflect.DeepEqual(got, tt.want) {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func Test_createAgentAST(t *testing.T) {
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
						Name: "err",
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "NewApplication",
							Path: newrelicAgentImport,
						},
						Args: []dst.Expr{
							&dst.CallExpr{
								Fun: &dst.Ident{
									Path: newrelicAgentImport,
									Name: "ConfigFromEnvironment",
								},
							},
						},
					},
				},
			}, panicOnError()},
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
						Name: "err",
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "NewApplication",
							Path: newrelicAgentImport,
						},
						Args: []dst.Expr{
							&dst.CallExpr{
								Fun: &dst.Ident{
									Path: newrelicAgentImport,
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
									Path: newrelicAgentImport,
									Name: "ConfigFromEnvironment",
								},
							},
						},
					},
				},
			}, panicOnError()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, createAgentAST(tt.args.AppName, tt.args.AgentVariableName))
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
			got := shutdownAgent(tt.args.AgentVariableName)
			assert.Equal(t, tt.want, got)
		})
	}
}

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
			got := startTransaction(tt.args.appVariableName, tt.args.transactionVariableName, tt.args.transactionName, tt.args.overwriteVariable)
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
			got := endTransaction(tt.args.transactionVariableName)
			assert.Equal(t, tt.want, got)
		})
	}
}
