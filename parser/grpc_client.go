package parser

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/parser/codegen"
)

func grpcDialCall(node dst.Node) (*dst.CallExpr, bool) {
	switch v := node.(type) {
	case *dst.AssignStmt:
		if len(v.Rhs) == 1 {
			if call, ok := v.Rhs[0].(*dst.CallExpr); ok {
				if ident, ok := call.Fun.(*dst.Ident); ok {
					if ident.Name == "Dial" && ident.Path == codegen.GrpcImportPath {
						return call, true
					}
				}
			}
		}
	case *dst.ExprStmt:
		if call, ok := v.X.(*dst.CallExpr); ok {
			if ident, ok := call.Fun.(*dst.Ident); ok {
				if ident.Name == "Dial" && ident.Path == codegen.GrpcImportPath {
					return call, true
				}
			}
		}
	}
	return nil, false
}

// Stateless Tracing Functions
// ////////////////////////////////////////////

// InstrumentGrpcDial adds the New Relic gRPC client interceptor to the grpc.Dial client call
// This function does not need any tracing context to work, nor will it produce any tracing context
func InstrumentGrpcDial(manager *InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	if callExpr, ok := grpcDialCall(currentNode); ok {
		decs := codegen.GetCallExpressionArgumentDecorations(callExpr)
		callExpr.Args = append(callExpr.Args, codegen.NrGrpcUnaryClientInterceptor(decs), codegen.NrGrpcStreamClientInterceptor(decs))
		manager.addImport(codegen.NrgrpcImportPath)
	}
}
