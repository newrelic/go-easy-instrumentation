package nrslog

import (
	"slices"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/parser"
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
			if (ident.Name == "NewTextHandler" || ident.Name == "NewJSONHandler") && ident.Path == slogImportPath {
				if v.Lhs != nil {
					return v.Lhs[0].(*dst.Ident).Name
				}
			}
		}
	}

	return ""
}

// detectSetDefaultCall checks if a statement is slog.SetDefault()
func detectSetDefaultCall(stmt dst.Stmt) (*dst.CallExpr, bool) {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return nil, false
	}

	call, ok := exprStmt.X.(*dst.CallExpr)
	if !ok {
		return nil, false
	}

	if ident, ok := call.Fun.(*dst.Ident); ok {
		if ident.Name == "SetDefault" && ident.Path == slogImportPath {
			return call, true
		}
	}

	return nil, false
}

// Stateless Tracing Functions
// ////////////////////////////////////////////
// InstrumentSlogHandler will check to see if any slog.NewTextHandler or slog.NewJSONHandler calls are made within any function

func InstrumentSlogHandler(manager *parser.InstrumentationManager, c *dstutil.Cursor) {
	mainFunctionNode := c.Node()
	if decl, ok := mainFunctionNode.(*dst.FuncDecl); ok {

		// loop through all statements within the body of the main method to see if any slog TextHandler calls are made
		var handlerNames []string
		for i := 0; i < len(decl.Body.List); i++ {
			stmt := decl.Body.List[i]

			slogHandler := slogMiddlewareCall(stmt)
			if slogHandler != "" {
				// We detected an slog handler
				nrHandler := "NR" + slogHandler
				handlerNames = append(handlerNames, slogHandler)
				middleware, goGet := SlogHandlerWrapper(slogHandler, nrHandler)
				decl.Body.List = append(decl.Body.List[:i+1], append([]dst.Stmt{middleware}, decl.Body.List[i+1:]...)...)
				manager.AddImport(goGet)
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
				case *dst.ReturnStmt:
					// Handle return statements with slog.New(handler)
					for _, result := range s.Results {
						if cs, isCall := result.(*dst.CallExpr); isCall {
							for ai, arg := range cs.Args {
								if aident, isIdentifier := arg.(*dst.Ident); isIdentifier {
									if slices.Contains(handlerNames, aident.Name) {
										cs.Args[ai].(*dst.Ident).Name = "NR" + aident.Name
									}
								}
							}
						}
					}
				case *dst.ExprStmt:
					// Handle slog.SetDefault() calls
					if setDefaultCall, isSetDefault := detectSetDefaultCall(s); isSetDefault {
						// Replace handler references in SetDefault call
						if len(setDefaultCall.Args) == 1 {
							if nestedCall, ok := setDefaultCall.Args[0].(*dst.CallExpr); ok {
								for ai, arg := range nestedCall.Args {
									if aident, isIdentifier := arg.(*dst.Ident); isIdentifier {
										if slices.Contains(handlerNames, aident.Name) {
											nestedCall.Args[ai].(*dst.Ident).Name = "NR" + aident.Name
										}
									}
								}
							}
						}
					}
				}
			}
		}

	}
}
