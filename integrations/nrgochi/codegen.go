package nrgochi

import (
	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/integrations/common"
)

const (
	NrChiImportPath = "github.com/newrelic/go-agent/v3/integrations/nrgochi"
)

// ChiMiddleware is the configured HttpMiddleware for the nrgochi integration.
var ChiMiddleware = &common.HttpMiddleware{
	ImportPath:         NrChiImportPath,
	MiddlewareFuncName: "Middleware",
	RouterMethodName:   "Use",
}

// NrChiMiddleware injects New Relic middleware into the Chi router via router.Use().
//
// Example output:
//
//	router.Use(nrgochi.Middleware(app))
func NrChiMiddleware(routerName string, agentVariableName dst.Expr) (*dst.ExprStmt, string) {
	return ChiMiddleware.MiddlewareStmt(routerName, agentVariableName)
}
