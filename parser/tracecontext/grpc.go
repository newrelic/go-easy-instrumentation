package tracecontext

import (
	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
)

// StreamObject is a transaction carrier for gRPC server stream objects
type StreamObject struct {
	typeExpr                 *dst.Ident
	typeName                 string
	streamObjectVariableName string
	transactionVariableName  string
}

func NewGrpcStreamObject(typeExpr *dst.Ident, typeName, variableName string) *StreamObject {
	return &StreamObject{
		typeExpr:                 typeExpr,
		typeName:                 typeName,
		streamObjectVariableName: variableName,
	}
}

// AssignTransactionVariable returns the code to assign the gRPC stream server object to a variable.
func (s *StreamObject) AssignTransactionVariable(transactionVariableName string) dst.Stmt {
	if s.transactionVariableName != "" {
		return nil
	}

	s.transactionVariableName = transactionVariableName
	return codegen.TxnFromContext(transactionVariableName, codegen.GrpcStreamContext(dst.NewIdent(s.streamObjectVariableName)))
}

func compareTypeIdents(a, b *dst.Ident) bool {
	return a.Name == b.Name && a.Path == b.Path
}

// Pass checks the function declaration and passes a trace context to a call expression based on its arguments and parameters.
// Passing preferrs to pass by type,
func (s *StreamObject) Pass(decl *dst.FuncDecl, call *dst.CallExpr, c *dstutil.Cursor, async bool) (TraceContext, error) {
	contextIndex := -1
	streamIndex := -1
	argumentIndex := 0
	for _, arg := range decl.Type.Params.List {
		if isContextParam(arg) && contextIndex == -1 {
			contextIndex = argumentIndex
		} else {
			ident, ok := arg.Type.(*dst.Ident)
			if ok && compareTypeIdents(ident, s.typeExpr) && streamIndex == -1 {
				streamIndex = argumentIndex
			}
		}
		argumentIndex += len(arg.Names)
	}

	if streamIndex != -1 {
		if len(call.Args) > streamIndex {

		}
	}

	return nil, nil
}

func (s *StreamObject) Type() string {
	return s.typeName
}
