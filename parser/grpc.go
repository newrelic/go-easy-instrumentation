package parser

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser/facts"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate/traceobject"
)

const (
	grpcServerType       = "*google.golang.org/grpc.Server"
	grpcServerStreamType = "google.golang.org/grpc.ServerStream"
	grpcPath             = "google.golang.org/grpc"
	contextType          = "context.Context"
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

// traceObject must not be nil if grpcServerTxnData is returned
type grpcServerTxnData struct {
	assignment  *dst.AssignStmt
	traceObject traceobject.TraceObject
}

// getTxnFromGrpcServer finds the transaction object from a gRPC server method
// This is done by looking for a context object or a stream server object in the function parameters
// and then pulling the transaction from that object and assigning it to a variable.
func getTxnFromGrpcServer(manager *InstrumentationManager, params []*dst.Field, txnVariableName string) (*grpcServerTxnData, bool) {
	// Find stream server object parameters first
	var streamServerIdent *dst.Ident
	var contextIdent *dst.Ident

	pkg := manager.getDecoratorPackage()
	for _, param := range params {
		paramType := util.TypeOf(param.Names[0], pkg)
		if paramType != nil {
			underlyingType := paramType.Underlying()
			if len(param.Names) == 1 {
				// check if this is a stream server object or a context object
				paramTypeName := paramType.String()
				if util.IsUnderlyingType(underlyingType, grpcServerStreamType) {
					streamServerIdent = param.Names[0]
				} else if paramTypeName == contextType {
					contextIdent = param.Names[0]
				}
			}
		}
	}

	var txnData *grpcServerTxnData
	var ok bool
	if streamServerIdent != nil {
		ok = true
		txnData = &grpcServerTxnData{
			assignment:  codegen.TxnFromContext(txnVariableName, codegen.GrpcStreamContext(streamServerIdent)),
			traceObject: traceobject.NewTransaction(),
		}
	} else if contextIdent != nil {
		ok = true
		txnData = &grpcServerTxnData{
			traceObject: traceobject.NewContext(contextIdent.Name),
		}
	}

	return txnData, ok
}

// isGrpcServerMethod checks if a function declaration is a method of the user's gRPC server
// based on facts generated from scanning their gRPC configuration code.
func isGrpcServerMethod(manager *InstrumentationManager, funcDecl *dst.FuncDecl) bool {
	if funcDecl.Recv == nil || len(funcDecl.Recv.List) != 1 || len(funcDecl.Recv.List[0].Names) != 1 {
		return false
	}

	// attempt to get the type of the receiver
	pkg := manager.getDecoratorPackage()
	recvType := util.TypeOf(funcDecl.Recv.List[0].Names[0], pkg)
	if recvType == nil {
		return false
	}

	// check if the receiver is a gRPC server method using the FactStore
	recvTypeString := recvType.String()
	fact := manager.facts.GetFact(recvTypeString)
	return fact == facts.GrpcServerType
}

// InstrumentGrpcServerMethod finds methods of a declared gRPC server and pulls tracing through it
func InstrumentGrpcServerMethod(manager *InstrumentationManager, c *dstutil.Cursor) {
	n := c.Node()
	funcDecl, ok := n.(*dst.FuncDecl)
	if !ok || !isGrpcServerMethod(manager, funcDecl) {
		return
	}

	// find either a context or a server stream object
	txnData, ok := getTxnFromGrpcServer(manager, funcDecl.Type.Params.List, codegen.DefaultTransactionVariable)
	if !ok {
		return
	}

	// ok is true if the body of this function has any tracing code added to it. If this is true, we know it needs a transaction to get
	// pulled from the grpc server object
	node, ok := TraceFunction(manager, funcDecl, tracestate.FunctionBody(codegen.DefaultTransactionVariable, txnData.traceObject))
	decl := node.(*dst.FuncDecl)
	if ok && txnData.assignment != nil {
		comment.Debug(manager.getDecoratorPackage(), funcDecl, fmt.Sprintf("Instrumenting gRPC server method: %s", funcDecl.Name.Name))
		decl.Body.List = append([]dst.Stmt{txnData.assignment}, decl.Body.List...)
	}
}

// InstrumentGrpcDial adds the New Relic gRPC client interceptor to the grpc.Dial client call
// This function does not need any tracing context to work, nor will it produce any tracing context
func InstrumentGrpcDial(manager *InstrumentationManager, c *dstutil.Cursor) {
	currentNode := c.Node()
	if callExpr, ok := grpcDialCall(currentNode); ok {
		comment.Debug(manager.getDecoratorPackage(), currentNode, "Injecting gRPC client interceptors into grpc.Dial")
		callExpr.Args = append(callExpr.Args, codegen.NrGrpcUnaryClientInterceptor(callExpr))
		callExpr.Args = append(callExpr.Args, codegen.NrGrpcStreamClientInterceptor(callExpr))
		manager.addImport(codegen.NrgrpcImportPath)
	}
}

// Stateful Tracing Funcs
//////////////////////////////////////////////

// InstrumentGrpcServer adds the New Relic gRPC server interceptors to the grpc.NewServer call
func InstrumentGrpcServer(manager *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
	// determine if this is a gRPC server initialization
	callExpr, ok := grpcNewServerCall(stmt)
	if !ok {
		return false
	}

	// inject middleware
	comment.Debug(manager.getDecoratorPackage(), stmt, "Injecting gRPC server interceptors into grpc.NewServer")
	callExpr.Args = append(callExpr.Args, codegen.NrGrpcUnaryServerInterceptor(tracing.AgentVariable(), callExpr))
	callExpr.Args = append(callExpr.Args, codegen.NrGrpcStreamServerInterceptor(tracing.AgentVariable(), callExpr))
	manager.addImport(codegen.NrgrpcImportPath)
	return true
}

// Dependency Scans
// ////////////////////////////////////////////

// isGrpcRegisterServerCall checks if a call expression is a call to a gRPC Register***Server function
// must check length of call.Args == 2 before calling this.
func isGrpcRegisterServerCall(call *dst.CallExpr, pkg *decorator.Package) bool {
	if len(call.Args) != 2 {
		return false
	}

	callFuncName := util.FunctionName(call)
	if strings.Index(callFuncName, "Register") == 0 && strings.Index(callFuncName, "Server") == len(callFuncName)-6 {
		if serverIdent, ok := call.Args[0].(*dst.Ident); ok {
			serverType := util.TypeOf(serverIdent, pkg)
			return serverType != nil && serverType.String() == grpcServerType || util.IsUnderlyingType(serverType.Underlying(), grpcServerType)
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

// FindGrpcServerObject scans for a call to Register...Server in the package
// It uses this call to identify the gRPC server Implementation object
func FindGrpcServerObject(pkg *decorator.Package, node dst.Node) (facts.Entry, bool) {
	if node == nil {
		return facts.Entry{}, false
	}

	expr, ok := node.(*dst.ExprStmt)
	if !ok {
		return facts.Entry{}, false
	}

	// look for gRPC server registration call
	call, ok := expr.X.(*dst.CallExpr)
	if !ok || !isGrpcRegisterServerCall(call, pkg) {
		return facts.Entry{}, false
	}

	// get the server object that was registered
	serverHandlerIdent, ok := getRegisteredServerIdent(call)
	if !ok {
		return facts.Entry{}, false
	}

	// find the type of the server object
	handlerType := util.TypeOf(serverHandlerIdent, pkg)
	if handlerType == nil {
		return facts.Entry{}, false
	}

	// this is an interface, so the object will always be a pointer in the implemented code
	handlerTypeString := handlerType.String()
	if handlerTypeString[0] != '*' {
		handlerTypeString = "*" + handlerTypeString
	}
	return facts.Entry{Name: handlerTypeString, Fact: facts.GrpcServerType}, true
}
