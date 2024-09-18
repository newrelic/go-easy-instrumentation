package codegen

import (
	"go/token"

	"github.com/dave/dst"
)

const (
	// the import path for the newrelic package
	NewRelicAgentImportPath string = "github.com/newrelic/go-agent/v3/newrelic"
)

func InitializeAgent(AppName, AgentVariableName string) []dst.Stmt {
	newappArgs := []dst.Expr{
		&dst.CallExpr{
			Fun: &dst.Ident{
				Path: NewRelicAgentImportPath,
				Name: "ConfigFromEnvironment",
			},
		},
	}
	if AppName != "" {
		AppName = "\"" + AppName + "\""
		newappArgs = append([]dst.Expr{&dst.CallExpr{
			Fun: &dst.Ident{
				Path: NewRelicAgentImportPath,
				Name: "ConfigAppName",
			},
			Args: []dst.Expr{
				&dst.BasicLit{
					Kind:  token.STRING,
					Value: AppName,
				},
			},
		}}, newappArgs...)
	}

	agentInit := &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.Ident{
				Name: AgentVariableName,
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
					Path: NewRelicAgentImportPath,
				},
				Args: newappArgs,
			},
		},
	}

	return []dst.Stmt{agentInit, panicOnError()}
}

func ShutdownAgent(AgentVariableName string) *dst.ExprStmt {
	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X: &dst.Ident{
					Name: AgentVariableName,
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
	}
}

func panicOnError() *dst.IfStmt {
	return &dst.IfStmt{
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
	}
}
