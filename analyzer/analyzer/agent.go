package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

const (
	agentImport = "github.com/newrelic/go-agent/v3/newrelic"
)

func containsAgentImport(imports []*ast.ImportSpec) bool {
	for _, imp := range imports {
		if imp.Path.Value == agentImport {
			return true
		}
	}
	return false
}

func panicOnError() *ast.IfStmt {
	return &ast.IfStmt{
		If: 27,
		Cond: &ast.BinaryExpr{
			X: &ast.Ident{
				Name: "err",
			},
			OpPos: 34,
			Op:    token.NEQ,
			Y: &ast.Ident{
				Name: "nil",
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: &ast.Ident{
							Name: "panic",
						},
						Lparen: 49,
						Args: []ast.Expr{
							&ast.Ident{
								Name: "err",
							},
						},
						Ellipsis: 0,
					},
				},
			},
		},
	}
}

func createAgent(AppName, AgentVariableName string, agentPos, shutdownPos token.Pos) analysis.Diagnostic {
	agent := fmt.Sprintf(
		`// Initialize the New Relic Go Agent
	// This is a basic initialization. For more info on how to configure the agent, go to
	// https://docs.newrelic.com/docs/apm/agents/go-agent/configuration/go-agent-configuration/#make-config-changes
	%s, _ := newrelic.NewApplication(
		newrelic.ConfigAppName("%s"),
		newrelic.ConfigFromEnvironment(),
	)

`, AgentVariableName, AppName)

	shutdown := fmt.Sprintf(`

	%s.Shutdown(10 * time.Second)`, AgentVariableName)

	return analysis.Diagnostic{
		Message: "initialize the New Relic Go agent",
		URL:     "https://docs.newrelic.com/docs/apm/agents/go-agent/installation/install-new-relic-go/",
		SuggestedFixes: []analysis.SuggestedFix{
			{
				Message: "initialize the Go agent",
				TextEdits: []analysis.TextEdit{
					{
						Pos:     agentPos,
						NewText: []byte(agent),
					},
				},
			},
			{
				Message: "shut down the Go agent",
				TextEdits: []analysis.TextEdit{
					{
						Pos:     shutdownPos,
						NewText: []byte(shutdown),
					},
				},
			},
		},
	}

}

func txnFromCtx() *ast.AssignStmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.Ident{
				Name: "txn",
			},
		},
		Tok: token.DEFINE,
		Rhs: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "newrelic",
					},
					Sel: &ast.Ident{
						Name: "FromContext",
					},
				},
				Lparen: 54,
				Args: []ast.Expr{
					&ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X: &ast.Ident{
								Name: "r",
							},
							Sel: &ast.Ident{
								Name: "Context",
							},
						},
						Lparen:   64,
						Ellipsis: 0,
					},
				},
				Ellipsis: 0,
			},
		},
	}
}

func ImportAgent(p *analysis.Pass, file *ast.File) {
	if !containsAgentImport(file.Imports) {
		newImport := "\"github.com/newrelic/go-agent/v3/newrelic\"\n"

		p.Report(analysis.Diagnostic{
			Pos:     file.Imports[0].Pos(),
			Message: "import the New Relic Go agent",
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: "add github.com/newrelic/go-agent/v3/newrelic imports",
					TextEdits: []analysis.TextEdit{
						{
							Pos:     file.Imports[0].Pos(),
							NewText: []byte(newImport),
						},
					},
				},
			},
		})
	}
}

func InitializeAgent(n ast.Node, data *InstrumentationData) string {
	if decl, ok := n.(*ast.FuncDecl); ok {
		// only inject go agent into the main.main function
		if data.AstFile.Name.Name == "main" && decl.Name.Name == "main" {
			ImportAgent(data.P, data.AstFile)

			agentReport := createAgent(data.AppName, data.AgentVariableName, decl.Body.List[0].Pos(), decl.Body.List[len(decl.Body.List)-1].End())
			data.P.Report(agentReport)
			return ""
		}
	}
	return ""
}
