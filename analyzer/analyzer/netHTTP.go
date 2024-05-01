package analyzer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"strings"

	"golang.org/x/tools/go/analysis"
)

func isNetHTTP(n *ast.CallExpr) bool {
	sel, ok := n.Fun.(*ast.SelectorExpr)
	if ok {
		indent, ok := sel.X.(*ast.Ident)
		if ok {
			return indent.Name == "http"
		}
	}
	return false
}

func isHandleFunc(n *ast.CallExpr) bool {
	sel, ok := n.Fun.(*ast.SelectorExpr)
	if ok {
		return sel.Sel.Name == "HandleFunc"
	}

	return false
}

func GetHandleFuncs(n ast.Node, data *InstrumentationData) string {
	var handler string
	callExpr, ok := n.(*ast.CallExpr)
	if ok && isNetHTTP(callExpr) {
		if isHandleFunc(callExpr) {
			if len(callExpr.Args) == 2 {
				// Capture name of handle funcs for deeper instrumentation
				handleFunc, ok := callExpr.Args[1].(*ast.Ident)
				if ok {
					handler = handleFunc.Name
				}
				return handler
			}
		}
	}
	return ""
}

func InstrumentHandleFunc(n ast.Node, data *InstrumentationData) string {
	callExpr, ok := n.(*ast.CallExpr)
	if ok && isNetHTTP(callExpr) {
		if isHandleFunc(callExpr) {
			if len(callExpr.Args) == 2 {
				buf := bytes.NewBuffer([]byte{})
				printer.Fprint(buf, data.P.Fset, callExpr)
				substr := strings.SplitAfterN(buf.String(), `(`, 2)
				fmt.Println(substr)
				wrappedHandler := fmt.Sprintf(`%snewrelic.WrapHandleFunc(%s, %s)`, substr[0], data.AgentVariableName, substr[1])
				data.P.Report(analysis.Diagnostic{
					Pos:     callExpr.Pos(),
					End:     callExpr.End(),
					Message: "wrap HTTP handle function",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "wrap the handle fuction",
							TextEdits: []analysis.TextEdit{
								{
									Pos:     callExpr.Pos(),
									End:     callExpr.End(),
									NewText: []byte(wrappedHandler),
								},
							},
						},
					},
				})
			}
		}
	}
	return ""
}
