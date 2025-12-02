package parser

import (
	"fmt"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
)

const (
	slogImportPath = "log/slog"
)

// slogMiddlewareCall returns the variable name of the slog TextHandler so that new relic middleware can be appended
func slogMiddlewareCall(stmt dst.Stmt) string {
	v, ok := stmt.(*dst.AssignStmt)
	if !ok || len(v.Rhs) != 1 {
		return ""
	}
	if call, ok := v.Rhs[0].(*dst.CallExpr); ok {
		if ident, ok := call.Fun.(*dst.Ident); ok {
			if (ident.Name == "NewTextHandler") && ident.Path == slogImportPath {
				if v.Lhs != nil {
					return v.Lhs[0].(*dst.Ident).Name
				}
			}
		}
	}

	return ""
}

// Stateless Tracing Functions
// ////////////////////////////////////////////
// InstrumentSlogHandler will check to see if any slog.NewTextHandler calls are made within the main function
// NOTE: Should we be limiting this to main? Is it possible/widely accepted to initialize a logging library outside of main?

func InstrumentSlogHandler(manager *InstrumentationManager, c *dstutil.Cursor) {
	mainFunctionNode := c.Node()
	if decl, ok := mainFunctionNode.(*dst.FuncDecl); ok {
		if decl.Name.Name != "main" {
			return
		}

		// loop through all statements within the body of the main method to see if any slog TextHandler calls are made
		for i, stmt := range decl.Body.List {
			slogHandler := slogMiddlewareCall(stmt)
			// No handler detected, continue onto next statement
			if slogHandler == "" {
				continue
			}
			fmt.Println("Bingo!", slogHandler)
			// bingo! Handler detected, lets inject our new relic integration here
			middleware, goGet := codegen.SlogHandlerWrapper(slogHandler)
			decl.Body.List = append(decl.Body.List[:i+1], append([]dst.Stmt{middleware}, decl.Body.List[i+1:]...)...)
			manager.addImport(goGet)
			return

		}

	}
}
