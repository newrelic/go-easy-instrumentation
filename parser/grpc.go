package parser

import (
	"go/token"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser/facts"
)

const (
	grpcServerType = "*google.golang.org/grpc.Server"
	grpcPath       = "google.golang.org/grpc"
	contextType    = "context.Context"
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

func getTxnFromGrpcServer(manager *InstrumentationManager, params []*dst.Field, txnVariableName string) (*dst.AssignStmt, bool) {
	// Find stream server object parameters first
	var streamServerIdent *dst.Ident
	var contextIdent *dst.Ident

	pkg := manager.getDecoratorPackage()
	f := manager.facts

	for _, param := range params {
		if len(param.Names) == 1 {
			paramType := util.TypeOf(param.Names[0], pkg)
			if paramType != nil {
				paramTypeName := paramType.String()
				fact := f.GetFact(paramTypeName)
				if fact == facts.GrpcServerStream {
					streamServerIdent = param.Names[0]
				} else if paramTypeName == contextType {
					contextIdent = param.Names[0]
				}
			}
		}
	}

	if streamServerIdent != nil {
		return codegen.TxnFromContext(txnVariableName, codegen.GrpcStreamContext(streamServerIdent)), true
	} else if contextIdent != nil {
		return codegen.TxnFromContext(txnVariableName, contextIdent), true
	}

	return nil, false
}

func isGrpcServerMethod(manager *InstrumentationManager, funcDecl *dst.FuncDecl) bool {
	if funcDecl.Recv != nil && len(funcDecl.Recv.List) == 1 && len(funcDecl.Recv.List[0].Names) == 1 {
		recvIdent := funcDecl.Recv.List[0].Names[0]
		pkg := manager.getDecoratorPackage()
		recvType := util.TypeOf(recvIdent, pkg)
		if recvType != nil {
			recvTypeString := recvType.String()
			fact := manager.facts.GetFact(recvTypeString)
			return fact == facts.GrpcServerType
		}
	}
	return false
}

// InstrumentGrpcServerMethod finds methods of a declared gRPC server and pulls tracing through it
func InstrumentGrpcServerMethod(manager *InstrumentationManager, c *dstutil.Cursor) {
	n := c.Node()
	funcDecl, ok := n.(*dst.FuncDecl)
	if ok && isGrpcServerMethod(manager, funcDecl) {
		// find either a context or a server stream object
		txnAssignment, ok := getTxnFromGrpcServer(manager, funcDecl.Type.Params.List, defaultTxnName)
		if ok {
			decl, ok := TraceFunction(manager, funcDecl, TraceDownstreamFunction(defaultTxnName), noSegment())
			if ok {
				decl.Body.List = append([]dst.Stmt{txnAssignment}, decl.Body.List...)
				c.Replace(decl)
			}
		}
	}
}

// Stateful Tracing Funcs
//////////////////////////////////////////////

// InstrumentGrpcServer adds the New Relic gRPC server interceptors to the grpc.NewServer call
func InstrumentGrpcServer(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracingState) bool {
	if callExpr, ok := grpcNewServerCall(stmt); ok {
		callExpr.Args = append(callExpr.Args, codegen.NrGrpcUnaryServerInterceptor(tracing.GetAgentVariable(), callExpr))
		callExpr.Args = append(callExpr.Args, codegen.NrGrpcStreamServerInterceptor(tracing.GetAgentVariable(), callExpr))
		manager.addImport(codegen.NrgrpcImportPath)
		return true
	}
	return false
}

// Dependency Scans
// ////////////////////////////////////////////

// isGrpcRegisterServerCall checks if a call expression is a call to a gRPC Register***Server function
// must check length of call.Args == 2 before calling this.
func isGrpcRegisterServerCall(call *dst.CallExpr, pkg *decorator.Package) bool {
	callFuncName := util.FunctionName(call)
	if strings.Index(callFuncName, "Register") == 0 && strings.Index(callFuncName, "Server") == len(callFuncName)-6 {
		if serverIdent, ok := call.Args[0].(*dst.Ident); ok {
			serverType := util.TypeOf(serverIdent, pkg)
			return serverType != nil && serverType.String() == grpcServerType
		}
	}
	return false
}

// Must be called on a call with 2 arguments
func getRegisteredServerIdent(call *dst.CallExpr) (*dst.Ident, bool) {
	switch v := call.Args[1].(type) {
	case *dst.Ident:
		return v, true
	case *dst.UnaryExpr:
		composite, ok := v.X.(*dst.CompositeLit)
		if ok && composite.Type != nil && v.Op == token.AND {
			ident, ok := composite.Type.(*dst.Ident)
			return ident, ok
		}
	}

	return nil, false
}

// FindGrpcServerObject scans for a call to Register***Server in the package
// It uses this call to identify the gRPC server Implementation object
func FindGrpcServerObject(pkg *decorator.Package, node dst.Node) (facts.Entry, bool) {
	if node == nil {
		return facts.Entry{}, false
	}

	if expr, ok := node.(*dst.ExprStmt); ok {
		call, ok := expr.X.(*dst.CallExpr)
		if ok && isGrpcRegisterServerCall(call, pkg) && len(call.Args) == 2 {
			serverHandlerIdent, ok := getRegisteredServerIdent(call)
			if ok {
				handlerType := util.TypeOf(serverHandlerIdent, pkg)
				if handlerType != nil {
					handlerTypeString := handlerType.String()
					if handlerTypeString[0] != '*' {
						handlerTypeString = "*" + handlerTypeString
					}
					return facts.Entry{Name: handlerTypeString, Fact: facts.GrpcServerType}, true
				}
			}
		}
	}
	return facts.Entry{}, false
}

// FindGrpcServerStreamInterface scans for an interface that embeds the grpc.ServerStream object
// We know this is a carrier of contexts injected with New Relic Transactions
func FindGrpcServerStreamInterface(pkg *decorator.Package, node dst.Node) (facts.Entry, bool) {
	if node == nil {
		return facts.Entry{}, false
	}

	if genDecl, ok := node.(*dst.GenDecl); ok && len(genDecl.Specs) == 1 {
		typeSpec, ok := genDecl.Specs[0].(*dst.TypeSpec)
		if ok && typeSpec.Type != nil {
			interfaceType, ok := typeSpec.Type.(*dst.InterfaceType)
			if ok && interfaceType.Methods != nil && interfaceType.Methods.List != nil {
				for _, method := range interfaceType.Methods.List {
					ident, ok := method.Type.(*dst.Ident)
					if ok {
						if ident.Name == "ServerStream" && ident.Path == grpcPath {
							serverStreamType := util.TypeOf(typeSpec.Name, pkg)
							if serverStreamType != nil {
								return facts.Entry{Name: serverStreamType.String(), Fact: facts.GrpcServerStream}, true
							}
						}
					}
				}
			}
		}
	}

	return facts.Entry{}, false
}
