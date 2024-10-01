package codegen

import "github.com/dave/dst"

const (
	NrgrpcImportPath = "github.com/newrelic/go-agent/v3/integrations/nrgrpc"
	GrpcImportPath   = "google.golang.org/grpc"
)

// This must be invoked on each argument added to a call expression to ensure the correct spacing rules are applied
func getCallExpressionArgumentSpacing(call *dst.CallExpr) dst.NodeDecs {
	// no standard has been set yet, we prefer to newline each new statement we add.
	// this will change the original decorator rules
	if len(call.Args) == 1 {
		call.Args[0].Decorations().After = dst.NewLine
		call.Args[0].Decorations().Before = dst.NewLine
		return dst.NodeDecs{
			After: dst.NewLine,
		}
	}
	// if a prescedent exists, copy it.
	if len(call.Args) > 1 {
		decs := call.Args[1].Decorations()
		return dst.NodeDecs{
			After:  decs.After,
			Before: decs.Before,
		}
	}
	return dst.NodeDecs{
		After:  dst.None,
		Before: dst.None,
	}
}

// GrpcUnaryInterceptor generates a dst Call Expression for a newrelic nrgrpc unary interceptor
func NrGrpcUnaryClientInterceptor(call *dst.CallExpr) *dst.CallExpr {
	decs := getCallExpressionArgumentSpacing(call)
	return &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "WithUnaryInterceptor",
			Path: GrpcImportPath,
		},
		Args: []dst.Expr{
			&dst.Ident{
				Name: "UnaryClientInterceptor",
				Path: NrgrpcImportPath,
			},
		},
		Decs: dst.CallExprDecorations{
			NodeDecs: decs,
		},
	}
}

func NrGrpcStreamClientInterceptor(call *dst.CallExpr) *dst.CallExpr {
	decs := getCallExpressionArgumentSpacing(call)
	return &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "WithStreamInterceptor",
			Path: GrpcImportPath,
		},
		Args: []dst.Expr{
			&dst.Ident{
				Name: "StreamClientInterceptor",
				Path: NrgrpcImportPath,
			},
		},
		Decs: dst.CallExprDecorations{
			NodeDecs: decs,
		},
	}
}

func NrGrpcUnaryServerInterceptor(agentVariableName string, call *dst.CallExpr) *dst.CallExpr {
	decs := getCallExpressionArgumentSpacing(call)
	return &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "UnaryInterceptor",
			Path: GrpcImportPath,
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "UnaryServerInterceptor",
					Path: NrgrpcImportPath,
				},
				Args: []dst.Expr{
					dst.NewIdent(agentVariableName),
				},
			},
		},
		Decs: dst.CallExprDecorations{
			NodeDecs: decs,
		},
	}
}

func NrGrpcStreamServerInterceptor(agentVariableName string, call *dst.CallExpr) *dst.CallExpr {
	decs := getCallExpressionArgumentSpacing(call)
	return &dst.CallExpr{
		Fun: &dst.Ident{
			Name: "StreamInterceptor",
			Path: GrpcImportPath,
		},
		Args: []dst.Expr{
			&dst.CallExpr{
				Fun: &dst.Ident{
					Name: "StreamServerInterceptor",
					Path: NrgrpcImportPath,
				},
				Args: []dst.Expr{
					dst.NewIdent(agentVariableName),
				},
			},
		},
		Decs: dst.CallExprDecorations{
			NodeDecs: decs,
		},
	}
}

func GrpcStreamContext(streamServerObject *dst.Ident) *dst.CallExpr {
	return &dst.CallExpr{
		Fun: &dst.SelectorExpr{
			X:   dst.Clone(streamServerObject).(*dst.Ident),
			Sel: &dst.Ident{Name: "Context"},
		},
	}
}
