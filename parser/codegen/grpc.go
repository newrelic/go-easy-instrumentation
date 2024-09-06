package codegen

import "github.com/dave/dst"

const (
	NrgrpcImportPath = "github.com/newrelic/go-agent/v3/integrations/nrgrpc"
	GrpcImportPath   = "google.golang.org/grpc"
)

// GrpcUnaryInterceptor generates a dst Call Expression for a newrelic nrgrpc unary interceptor
func NrGrpcUnaryClientInterceptor(decs dst.NodeDecs) *dst.CallExpr {
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

func NrGrpcStreamClientInterceptor(decs dst.NodeDecs) *dst.CallExpr {
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

func GetCallExpressionArgumentDecorations(call *dst.CallExpr) dst.NodeDecs {
	// no standard has been set yet, we prefer to newline each new statement we add.
	if len(call.Args) == 1 {
		call.Args[0].Decorations().After = dst.NewLine
		return dst.NodeDecs{
			After: dst.NewLine,
		}
	}
	// if a prescedent exists, copy it.
	if len(call.Args) > 1 {
		decs := call.Args[0].Decorations()
		return dst.NodeDecs{
			After:  decs.After,
			Before: decs.Before,
		}
	}
	return dst.NodeDecs{
		After: dst.None,
	}
}
