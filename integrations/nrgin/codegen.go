package nrgin

import (
	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/integrations/common"
)

const (
	NrginImportPath = "github.com/newrelic/go-agent/v3/integrations/nrgin"
	GinImportPath   = "github.com/gin-gonic/gin"
)

// GinMiddleware is the configured HttpMiddleware for the nrgin integration.
var GinMiddleware = &common.HttpMiddleware{
	ImportPath:         NrginImportPath,
	MiddlewareFuncName: "Middleware",
	TxnFuncName:        "Transaction",
	RouterMethodName:   "Use",
}

// NrGinMiddleware returns a Gin middleware call that instruments the router
// with New Relic. Returns the middleware statement and the import path.
//
// Example output:
//
//	router.Use(nrgin.Middleware(app))
func NrGinMiddleware(routerName string, agentVariableName dst.Expr) (*dst.ExprStmt, string) {
	return GinMiddleware.MiddlewareStmt(routerName, agentVariableName)
}

// TxnFromGinContext generates code to extract a New Relic transaction from
// a Gin context.
//
// Example output:
//
//	txn := nrgin.Transaction(c)
func TxnFromGinContext(txnVariable string, ctxName string) *dst.AssignStmt {
	return GinMiddleware.TxnFromContext(txnVariable, ctxName)
}
