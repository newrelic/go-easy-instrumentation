package common

import (
	"go/token"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
)

// HttpMiddleware holds the configuration for generating New Relic middleware
// instrumentation code for HTTP frameworks that follow the router.Use(nrXxx.Middleware(app)) pattern.
type HttpMiddleware struct {
	ImportPath         string
	MiddlewareFuncName string
	TxnFuncName        string
	RouterMethodName   string
}

// MiddlewareStmt returns a router.Use(nrXxx.Middleware(app)) expression statement
// and the import path to add.
func (m *HttpMiddleware) MiddlewareStmt(routerName string, agentVariableName dst.Expr) (*dst.ExprStmt, string) {
	return &dst.ExprStmt{
		X: &dst.CallExpr{
			Fun: &dst.SelectorExpr{
				X:   &dst.Ident{Name: routerName},
				Sel: &dst.Ident{Name: m.RouterMethodName},
			},
			Args: []dst.Expr{
				&dst.CallExpr{
					Fun: &dst.Ident{
						Name: m.MiddlewareFuncName,
						Path: m.ImportPath,
					},
					Args: []dst.Expr{
						agentVariableName,
					},
				},
			},
		},
	}, m.ImportPath
}

// TxnFromContext returns a txn := nrXxx.TxnFunc(ctx) assignment statement.
func (m *HttpMiddleware) TxnFromContext(txnVariable string, ctxName string) *dst.AssignStmt {
	return &dst.AssignStmt{
		Lhs: []dst.Expr{
			&dst.Ident{Name: txnVariable},
		},
		Tok: token.DEFINE,
		Rhs: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: m.TxnFuncName,
					Path: m.ImportPath,
				},
				Args: []dst.Expr{
					&dst.Ident{Name: ctxName},
				},
			},
		},
		Decs: dst.AssignStmtDecorations{
			NodeDecs: dst.NodeDecs{
				Before: dst.NewLine,
			},
		},
	}
}

// HasExistingMiddleware reports whether the middleware call is already present
// in the block after the current cursor position.
func (m *HttpMiddleware) HasExistingMiddleware(c *dstutil.Cursor) bool {
	parent := c.Parent()
	blockStmt, ok := parent.(*dst.BlockStmt)
	if !ok {
		return false
	}

	currentIndex := -1
	for i, stmt := range blockStmt.List {
		if stmt == c.Node() {
			currentIndex = i
			break
		}
	}

	if currentIndex < 0 {
		return false
	}

	for i := currentIndex + 1; i < len(blockStmt.List); i++ {
		exprStmt, ok := blockStmt.List[i].(*dst.ExprStmt)
		if !ok {
			continue
		}
		callExpr, ok := exprStmt.X.(*dst.CallExpr)
		if !ok {
			continue
		}
		selExpr, ok := callExpr.Fun.(*dst.SelectorExpr)
		if !ok {
			continue
		}
		if selExpr.Sel.Name == m.RouterMethodName && len(callExpr.Args) > 0 {
			if argCall, ok := callExpr.Args[0].(*dst.CallExpr); ok {
				if ident, ok := argCall.Fun.(*dst.Ident); ok {
					if ident.Name == m.MiddlewareFuncName && ident.Path == m.ImportPath {
						return true
					}
				}
			}
		}
	}

	return false
}

// HasExistingTransaction reports whether the function body already contains
// a transaction extraction call.
func (m *HttpMiddleware) HasExistingTransaction(funcDecl *dst.FuncDecl) bool {
	if funcDecl == nil || funcDecl.Body == nil {
		return false
	}

	found := false
	dstutil.Apply(funcDecl.Body, func(c *dstutil.Cursor) bool {
		if stmt, ok := c.Node().(*dst.AssignStmt); ok {
			for _, rhs := range stmt.Rhs {
				if callExpr, ok := rhs.(*dst.CallExpr); ok {
					if ident, ok := callExpr.Fun.(*dst.Ident); ok {
						if ident.Name == m.TxnFuncName && ident.Path == m.ImportPath {
							found = true
							return false
						}
					}
				}
			}
		}
		return true
	}, nil)

	return found
}

// DefineTxnFromCtx prepends a transaction extraction statement to the function body.
func (m *HttpMiddleware) DefineTxnFromCtx(body *dst.BlockStmt, txnVariable string, ctxName string) {
	stmts := make([]dst.Stmt, len(body.List)+1)
	stmts[0] = m.TxnFromContext(txnVariable, ctxName)
	for i, stmt := range body.List {
		stmts[i+1] = stmt
	}
	body.List = stmts
}
