package nrecho

import (
	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/integrations/common"
)

const (
	NrechoImportPath = "github.com/newrelic/go-agent/v3/integrations/nrecho-v3"
	EchoImportPath   = "github.com/labstack/echo"
)

// EchoMiddleware is the configured HttpMiddleware for the nrecho-v3 integration.
var EchoMiddleware = &common.HttpMiddleware{
	ImportPath:         NrechoImportPath,
	MiddlewareFuncName: "Middleware",
	TxnFuncName:        "FromContext",
	RouterMethodName:   "Use",
}

// NrEchoMiddleware returns an Echo middleware call that instruments the router
// with New Relic. Returns the middleware statement and the import path.
//
// Example output:
//
//	e.Use(nrecho.Middleware(app))
func NrEchoMiddleware(routerName string, agentVariableName dst.Expr) (*dst.ExprStmt, string) {
	return EchoMiddleware.MiddlewareStmt(routerName, agentVariableName)
}

// TxnFromEchoContext generates code to extract a New Relic transaction from
// an Echo context.
//
// Example output:
//
//	txn := nrecho.FromContext(c)
func TxnFromEchoContext(txnVariable string, ctxName string) *dst.AssignStmt {
	return EchoMiddleware.TxnFromContext(txnVariable, ctxName)
}
