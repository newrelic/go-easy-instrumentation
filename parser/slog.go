package parser

import (
	"slices"

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
		var handlerNames []string
		for i := 0; i < len(decl.Body.List); i++ {
			stmt := decl.Body.List[i]

			slogHandler := slogMiddlewareCall(stmt)
			if slogHandler != "" {
				// We detected an slog handler
				nrHandler := "NR" + slogHandler
				handlerNames = append(handlerNames, slogHandler)
				middleware, goGet := codegen.SlogHandlerWrapper(slogHandler, nrHandler)
				decl.Body.List = append(decl.Body.List[:i+1], append([]dst.Stmt{middleware}, decl.Body.List[i+1:]...)...)
				manager.addImport(goGet)
				i++
			} else if len(handlerNames) > 0 {
				// Translate any handlers we've seen so far to our wrapped ones
				//dst.Print(stmt)
				switch s := stmt.(type) {
				case *dst.AssignStmt:
					if len(s.Rhs) == 1 {
						if cs, isCall := s.Rhs[0].(*dst.CallExpr); isCall {
							for ai, arg := range cs.Args {
								if aident, isIdentifier := arg.(*dst.Ident); isIdentifier {
									if slices.Contains(handlerNames, aident.Name) {
										s.Rhs[0].(*dst.CallExpr).Args[ai].(*dst.Ident).Name = "NR" + aident.Name
									}
								}
							}
						}
					}
				case *dst.ExprStmt:
					// TODO - in the future, be more aggressive about hunting more cases where these
					// identifiers appear
				}
			}
		}

	}
}
