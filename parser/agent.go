package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
)

// Code generation
//////////////////////////////////////////////

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

func createAgentAST(AppName, AgentVariableName string) []dst.Stmt {
	newappArgs := []dst.Expr{
		&dst.CallExpr{
			Fun: &dst.Ident{
				Path: newrelicAgentImport,
				Name: "ConfigFromEnvironment",
			},
		},
	}
	if AppName != "" {
		AppName = "\"" + AppName + "\""
		newappArgs = append([]dst.Expr{&dst.CallExpr{
			Fun: &dst.Ident{
				Path: newrelicAgentImport,
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
					Path: newrelicAgentImport,
				},
				Args: newappArgs,
			},
		},
	}

	return []dst.Stmt{agentInit, panicOnError()}
}

func shutdownAgent(AgentVariableName string) *dst.ExprStmt {
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

// starts a NewRelic transaction
// if overwireVariable is true, the transaction variable will be overwritten by variable assignment, otherwise it will be defined
func startTransaction(appVariableName, transactionVariableName, transactionName string, overwriteVariable bool) *dst.AssignStmt {
	tok := token.DEFINE
	if overwriteVariable {
		tok = token.ASSIGN
	}
	return &dst.AssignStmt{
		Lhs: []dst.Expr{dst.NewIdent(transactionVariableName)},
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Args: []dst.Expr{
					&dst.BasicLit{
						Kind:  token.STRING,
						Value: fmt.Sprintf(`"%s"`, transactionName),
					},
				},
				Fun: &dst.SelectorExpr{
					X:   dst.NewIdent(appVariableName),
					Sel: dst.NewIdent("StartTransaction"),
				},
			},
		},
		Tok: tok,
	}
}

func endTransaction(transactionVariableName string) *dst.ExprStmt {
	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   dst.NewIdent(transactionVariableName),
				Sel: dst.NewIdent("End"),
			},
		},
	}
}

func txnAsParameter(txnName string) *dst.Field {
	return &dst.Field{
		Names: []*dst.Ident{
			{
				Name: txnName,
			},
		},
		Type: &dst.StarExpr{
			X: &dst.Ident{
				Name: "Transaction",
				Path: newrelicAgentImport,
			},
		},
	}
}

func deferSegment(segmentName, txnVarName string) *dst.DeferStmt {
	return &dst.DeferStmt{
		Call: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: dst.NewIdent(txnVarName),
						Sel: &dst.Ident{
							Name: "StartSegment",
						},
					},
					Args: []dst.Expr{
						&dst.BasicLit{
							Kind:  token.STRING,
							Value: fmt.Sprintf(`"%s"`, segmentName),
						},
					},
				},
				Sel: &dst.Ident{
					Name: "End",
				},
			},
		},
	}
}

func txnNewGoroutine(txnVarName string) *dst.CallExpr {
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X: &dst.Ident{
				Name: txnVarName,
			},
			Sel: &dst.Ident{
				Name: "NewGoroutine",
			},
		},
	}
}

func isNamedError(n *types.Named) bool {
	if n == nil {
		return false
	}

	o := n.Obj()
	return o != nil && o.Pkg() == nil && o.Name() == "error"
}

// errorReturnIndex returns the index of the error return value in the function call
// if no error is returned it will return 0, false
func errorReturnIndex(v *dst.CallExpr, pkg *decorator.Package) (int, bool) {
	if pkg == nil {
		return 0, false
	}

	astCall, ok := pkg.Decorator.Ast.Nodes[v]
	if ok {
		ty := pkg.TypesInfo.TypeOf(astCall.(*ast.CallExpr))
		switch n := ty.(type) {
		case *types.Named:
			if isNamedError(n) {
				return 0, true
			}
		case *types.Tuple:
			for i := 0; i < n.Len(); i++ {
				t := n.At(i).Type()
				switch e := t.(type) {
				case *types.Named:
					if isNamedError(e) {
						return i, true
					}
				}
			}
		}
	}
	return 0, false
}

func isNewRelicMethod(call *dst.CallExpr) bool {
	if sel, ok := call.Fun.(*dst.SelectorExpr); ok {
		if pkg, ok := sel.X.(*dst.Ident); ok {
			return pkg.Name == "newrelic"
		}
	} else {
		if ident, ok := call.Fun.(*dst.Ident); ok {
			return ident.Path == newrelicAgentImport
		}
	}
	return false
}

func generateNoticeError(errVariableName, txnName string, nodeDecs *dst.NodeDecs) *dst.ExprStmt {
	var decs dst.ExprStmtDecorations
	// copy all decs below the current statement into this statement
	if nodeDecs != nil {
		decs = dst.ExprStmtDecorations{
			NodeDecs: dst.NodeDecs{
				After: nodeDecs.After,
				End:   nodeDecs.End,
			},
		}

		// remove coppied decs from above node
		nodeDecs.After = dst.None
		nodeDecs.End.Clear()
	}

	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X: &dst.Ident{
					Name: txnName,
				},
				Sel: &dst.Ident{
					Name: "NoticeError",
				},
			},
			Args: []dst.Expr{
				&dst.Ident{
					Name: errVariableName,
				},
			},
		},
		Decs: decs,
	}
}

func findErrorVariable(stmt *dst.AssignStmt, pkg *decorator.Package) string {
	if len(stmt.Rhs) == 1 {
		if call, ok := stmt.Rhs[0].(*dst.CallExpr); ok {
			if !isNewRelicMethod(call) {
				if errIndex, ok := errorReturnIndex(call, pkg); ok {
					expr := stmt.Lhs[errIndex]
					if ident, ok := expr.(*dst.Ident); ok {
						return ident.Name
					}
				}
			}
		}
	}
	return ""
}

// StatelessTracingFunctions
//////////////////////////////////////////////

// InstrumentMain looks for the main method of a program, and uses this as an instrumentation initialization and injection point
func InstrumentMain(mainFunctionNode dst.Node, manager *InstrumentationManager, c *dstutil.Cursor) {
	txnStarted := false
	if decl, ok := mainFunctionNode.(*dst.FuncDecl); ok {
		// only inject go agent into the main.main function
		if decl.Name.Name == "main" {
			agentDecl := createAgentAST(manager.appName, manager.agentVariableName)
			decl.Body.List = append(agentDecl, decl.Body.List...)
			decl.Body.List = append(decl.Body.List, shutdownAgent(manager.agentVariableName))

			// add go-agent/v3/newrelic to imports
			manager.AddImport(newrelicAgentImport)

			newMain := dstutil.Apply(decl, func(c *dstutil.Cursor) bool {
				node := c.Node()
				switch v := node.(type) {
				case *dst.ExprStmt:
					rootPkg := manager.currentPackage
					txnVarName := defaultTxnName
					invInfo := manager.GetPackageFunctionInvocation(v)
					// check if the called function has been instrumented already, if not, instrument it.
					if manager.ShouldInstrumentFunction(invInfo) {
						manager.SetPackage(invInfo.packageName)
						decl := manager.GetDeclaration(invInfo.functionName)
						_, wasModified := TraceFunction(manager, decl, defaultTxnName)
						if wasModified {
							// add transaction to declaration arguments
							manager.AddTxnArgumentToFunctionDecl(decl, defaultTxnName)
							manager.AddImport(newrelicAgentImport)
						}
						manager.SetPackage(rootPkg)
					}
					// pass the called function a transaction if needed
					// always check c.Index >= 0 to avoid panics when using c.Insert methods
					if manager.RequiresTransactionArgument(invInfo, txnVarName) && c.Index() >= 0 {
						c.InsertBefore(startTransaction(manager.agentVariableName, txnVarName, invInfo.functionName, txnStarted))
						c.InsertAfter(endTransaction(txnVarName))
						invInfo.call.Args = append(invInfo.call.Args, dst.NewIdent(defaultTxnName))
						txnStarted = true
					}
					WrapHandleFunc(v.X, manager, c)
				}

				return true
			}, nil)
			// this will skip the tracing of this function in the outer tree walking algorithm
			c.Replace(newMain)
		}
	}
}

// StatefulTracingFunctions
//////////////////////////////////////////////

// NoticeError will check for the presence of an error.Error variable in the body at the index in bodyIndex.
// If it finds that an error is returned, it will add a line after the assignment statement to capture an error
// with a newrelic transaction. All transactions are assumed to be named "txn"
func NoticeError(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, txnName string) bool {
	switch nodeVal := stmt.(type) {
	case *dst.AssignStmt:
		errVar := findErrorVariable(nodeVal, manager.GetDecoratorPackage())
		if errVar != "" && c.Index() >= 0 {
			c.InsertAfter(generateNoticeError(errVar, txnName, nodeVal.Decorations()))
			return true
		}
	}
	return false
}
