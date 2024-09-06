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

func grpcNewServerCall(node dst.Node) (*dst.CallExpr, bool) {
	switch v := node.(type) {
	case *dst.AssignStmt:
		if len(v.Rhs) == 1 {
			if call, ok := v.Rhs[0].(*dst.CallExpr); ok {
				if ident, ok := call.Fun.(*dst.Ident); ok {
					if ident.Name == "NewServer" && ident.Path == codegen.GrpcImportPath {
						return call, true
					}
				}
			}
		}
	case *dst.ExprStmt:
		if call, ok := v.X.(*dst.CallExpr); ok {
			if ident, ok := call.Fun.(*dst.Ident); ok {
				if ident.Name == "NewServer" && ident.Path == codegen.GrpcImportPath {
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
		callExpr.Args = append(callExpr.Args, codegen.NrGrpcUnaryClientInterceptor(callExpr))
		callExpr.Args = append(callExpr.Args, codegen.NrGrpcStreamClientInterceptor(callExpr))
		manager.addImport(codegen.NrgrpcImportPath)
	}
}

// Stateful Tracing Funcs
//////////////////////////////////////////////

func InstrumentGrpcServer(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracingState) bool {
	if callExpr, ok := grpcNewServerCall(stmt); ok {
		callExpr.Args = append(callExpr.Args, codegen.NrGrpcUnaryServerInterceptor(tracing.GetAgentVariable(), callExpr))
		callExpr.Args = append(callExpr.Args, codegen.NrGrpcStreamServerInterceptor(tracing.GetAgentVariable(), callExpr))
		manager.addImport(codegen.NrgrpcImportPath)
		return true
	}
	return false
}
