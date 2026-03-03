package codegen

import (
	"go/token"
	"reflect"
	"testing"

	"github.com/dave/dst"
)

func Test_getCallExpressionArgumentSpacing(t *testing.T) {
	type args struct {
		call *dst.CallExpr
	}
	tests := []struct {
		name string
		args args
		want dst.NodeDecs
	}{
		{
			name: "calls with 0 arguments",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "NewServer",
						Path: GrpcImportPath,
					},
					Args: []dst.Expr{},
				},
			},
			want: dst.NodeDecs{
				After:  dst.None,
				Before: dst.None,
			},
		},
		{
			name: "calls with 1 argument",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "NewServer",
						Path: GrpcImportPath,
					},
					Args: []dst.Expr{
						&dst.BasicLit{
							Kind:  token.STRING,
							Value: `"localhost:8080"`,
						},
					},
				},
			},
			want: dst.NodeDecs{
				After:  dst.NewLine,
				Before: dst.None,
			},
		},
		{
			name: "calls with many arguments follow existing spacing rules: no newlines",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "NewServer",
						Path: GrpcImportPath,
					},
					Args: []dst.Expr{
						&dst.BasicLit{
							Kind:  token.STRING,
							Value: `"localhost:8080"`,
							Decs: dst.BasicLitDecorations{
								NodeDecs: dst.NodeDecs{
									After:  dst.None,
									Before: dst.None,
								},
							},
						},
						dst.NewIdent("grpc.Creds"),
					},
				},
			},
			want: dst.NodeDecs{
				After:  dst.None,
				Before: dst.None,
			},
		},
		{
			name: "calls with many arguments follow existing spacing rules: newlines",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "NewServer",
						Path: GrpcImportPath,
					},
					Args: []dst.Expr{
						&dst.BasicLit{
							Kind:  token.STRING,
							Value: `"localhost:8080"`,
							Decs: dst.BasicLitDecorations{
								NodeDecs: dst.NodeDecs{
									After:  dst.NewLine,
									Before: dst.NewLine,
								},
							},
						},
						&dst.Ident{
							Name: "grpc.Creds",
							Decs: dst.IdentDecorations{
								NodeDecs: dst.NodeDecs{
									After: dst.NewLine,
								},
							},
						},
					},
				},
			},
			want: dst.NodeDecs{
				After: dst.NewLine,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCallExpressionArgumentSpacing(tt.args.call); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getCallExpressionArgumentSpacing() = %v, want %v", got, tt.want)
			}
			if len(tt.args.call.Args) == 1 {
				if tt.args.call.Args[0].Decorations().After != dst.NewLine {
					t.Errorf("expected the existing spacing After to be overwritten with %v; got %v", dst.NewLine, tt.args.call.Args[0].Decorations().After)
				}
				if tt.args.call.Args[0].Decorations().Before != dst.NewLine {
					t.Errorf("expected the existing spacing Before to be overwritten with %v; got %v", dst.NewLine, tt.args.call.Args[0].Decorations().Before)
				}
			}
		})
	}
}

func TestNrGrpcUnaryClientInterceptor(t *testing.T) {
	tests := []struct {
		name     string
		call     *dst.CallExpr
		wantPath string
		wantName string
	}{
		{
			name: "generates unary client interceptor with no existing args",
			call: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "Dial",
					Path: GrpcImportPath,
				},
				Args: []dst.Expr{},
			},
			wantPath: GrpcImportPath,
			wantName: "WithUnaryInterceptor",
		},
		{
			name: "generates unary client interceptor with existing args",
			call: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "Dial",
					Path: GrpcImportPath,
				},
				Args: []dst.Expr{
					&dst.BasicLit{
						Kind:  token.STRING,
						Value: `"localhost:8080"`,
					},
				},
			},
			wantPath: GrpcImportPath,
			wantName: "WithUnaryInterceptor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NrGrpcUnaryClientInterceptor(tt.call)

			if got == nil {
				t.Fatal("NrGrpcUnaryClientInterceptor() returned nil")
			}

			// Check the function identifier
			funIdent, ok := got.Fun.(*dst.Ident)
			if !ok {
				t.Fatalf("expected Fun to be *dst.Ident, got %T", got.Fun)
			}
			if funIdent.Name != tt.wantName {
				t.Errorf("expected function name %q, got %q", tt.wantName, funIdent.Name)
			}
			if funIdent.Path != tt.wantPath {
				t.Errorf("expected function path %q, got %q", tt.wantPath, funIdent.Path)
			}

			// Check the argument is UnaryClientInterceptor
			if len(got.Args) != 1 {
				t.Fatalf("expected 1 argument, got %d", len(got.Args))
			}
			argIdent, ok := got.Args[0].(*dst.Ident)
			if !ok {
				t.Fatalf("expected Args[0] to be *dst.Ident, got %T", got.Args[0])
			}
			if argIdent.Name != "UnaryClientInterceptor" {
				t.Errorf("expected arg name %q, got %q", "UnaryClientInterceptor", argIdent.Name)
			}
			if argIdent.Path != NrgrpcImportPath {
				t.Errorf("expected arg path %q, got %q", NrgrpcImportPath, argIdent.Path)
			}
		})
	}
}

func TestNrGrpcStreamClientInterceptor(t *testing.T) {
	tests := []struct {
		name     string
		call     *dst.CallExpr
		wantPath string
		wantName string
	}{
		{
			name: "generates stream client interceptor",
			call: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "Dial",
					Path: GrpcImportPath,
				},
				Args: []dst.Expr{},
			},
			wantPath: GrpcImportPath,
			wantName: "WithStreamInterceptor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NrGrpcStreamClientInterceptor(tt.call)

			if got == nil {
				t.Fatal("NrGrpcStreamClientInterceptor() returned nil")
			}

			// Check the function identifier
			funIdent, ok := got.Fun.(*dst.Ident)
			if !ok {
				t.Fatalf("expected Fun to be *dst.Ident, got %T", got.Fun)
			}
			if funIdent.Name != tt.wantName {
				t.Errorf("expected function name %q, got %q", tt.wantName, funIdent.Name)
			}
			if funIdent.Path != tt.wantPath {
				t.Errorf("expected function path %q, got %q", tt.wantPath, funIdent.Path)
			}

			// Check the argument is StreamClientInterceptor
			if len(got.Args) != 1 {
				t.Fatalf("expected 1 argument, got %d", len(got.Args))
			}
			argIdent, ok := got.Args[0].(*dst.Ident)
			if !ok {
				t.Fatalf("expected Args[0] to be *dst.Ident, got %T", got.Args[0])
			}
			if argIdent.Name != "StreamClientInterceptor" {
				t.Errorf("expected arg name %q, got %q", "StreamClientInterceptor", argIdent.Name)
			}
			if argIdent.Path != NrgrpcImportPath {
				t.Errorf("expected arg path %q, got %q", NrgrpcImportPath, argIdent.Path)
			}
		})
	}
}

func TestNrGrpcUnaryServerInterceptor(t *testing.T) {
	tests := []struct {
		name          string
		agentVariable dst.Expr
		call          *dst.CallExpr
		wantPath      string
		wantName      string
	}{
		{
			name:          "generates unary server interceptor",
			agentVariable: dst.NewIdent("app"),
			call: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "NewServer",
					Path: GrpcImportPath,
				},
				Args: []dst.Expr{},
			},
			wantPath: GrpcImportPath,
			wantName: "UnaryInterceptor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NrGrpcUnaryServerInterceptor(tt.agentVariable, tt.call)

			if got == nil {
				t.Fatal("NrGrpcUnaryServerInterceptor() returned nil")
			}

			// Check the function identifier
			funIdent, ok := got.Fun.(*dst.Ident)
			if !ok {
				t.Fatalf("expected Fun to be *dst.Ident, got %T", got.Fun)
			}
			if funIdent.Name != tt.wantName {
				t.Errorf("expected function name %q, got %q", tt.wantName, funIdent.Name)
			}
			if funIdent.Path != tt.wantPath {
				t.Errorf("expected function path %q, got %q", tt.wantPath, funIdent.Path)
			}

			// Check there's one argument which is a call expression
			if len(got.Args) != 1 {
				t.Fatalf("expected 1 argument, got %d", len(got.Args))
			}
			innerCall, ok := got.Args[0].(*dst.CallExpr)
			if !ok {
				t.Fatalf("expected Args[0] to be *dst.CallExpr, got %T", got.Args[0])
			}

			// Check the inner call is UnaryServerInterceptor
			innerFun, ok := innerCall.Fun.(*dst.Ident)
			if !ok {
				t.Fatalf("expected inner Fun to be *dst.Ident, got %T", innerCall.Fun)
			}
			if innerFun.Name != "UnaryServerInterceptor" {
				t.Errorf("expected inner function name %q, got %q", "UnaryServerInterceptor", innerFun.Name)
			}
			if innerFun.Path != NrgrpcImportPath {
				t.Errorf("expected inner function path %q, got %q", NrgrpcImportPath, innerFun.Path)
			}

			// Check the agent variable is passed
			if len(innerCall.Args) != 1 {
				t.Fatalf("expected inner call to have 1 argument, got %d", len(innerCall.Args))
			}
		})
	}
}

func TestNrGrpcStreamServerInterceptor(t *testing.T) {
	tests := []struct {
		name          string
		agentVariable dst.Expr
		call          *dst.CallExpr
		wantPath      string
		wantName      string
	}{
		{
			name:          "generates stream server interceptor",
			agentVariable: dst.NewIdent("app"),
			call: &dst.CallExpr{
				Fun: &dst.Ident{
					Name: "NewServer",
					Path: GrpcImportPath,
				},
				Args: []dst.Expr{},
			},
			wantPath: GrpcImportPath,
			wantName: "StreamInterceptor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NrGrpcStreamServerInterceptor(tt.agentVariable, tt.call)

			if got == nil {
				t.Fatal("NrGrpcStreamServerInterceptor() returned nil")
			}

			// Check the function identifier
			funIdent, ok := got.Fun.(*dst.Ident)
			if !ok {
				t.Fatalf("expected Fun to be *dst.Ident, got %T", got.Fun)
			}
			if funIdent.Name != tt.wantName {
				t.Errorf("expected function name %q, got %q", tt.wantName, funIdent.Name)
			}
			if funIdent.Path != tt.wantPath {
				t.Errorf("expected function path %q, got %q", tt.wantPath, funIdent.Path)
			}

			// Check there's one argument which is a call expression
			if len(got.Args) != 1 {
				t.Fatalf("expected 1 argument, got %d", len(got.Args))
			}
			innerCall, ok := got.Args[0].(*dst.CallExpr)
			if !ok {
				t.Fatalf("expected Args[0] to be *dst.CallExpr, got %T", got.Args[0])
			}

			// Check the inner call is StreamServerInterceptor
			innerFun, ok := innerCall.Fun.(*dst.Ident)
			if !ok {
				t.Fatalf("expected inner Fun to be *dst.Ident, got %T", innerCall.Fun)
			}
			if innerFun.Name != "StreamServerInterceptor" {
				t.Errorf("expected inner function name %q, got %q", "StreamServerInterceptor", innerFun.Name)
			}
			if innerFun.Path != NrgrpcImportPath {
				t.Errorf("expected inner function path %q, got %q", NrgrpcImportPath, innerFun.Path)
			}

			// Check the agent variable is passed
			if len(innerCall.Args) != 1 {
				t.Fatalf("expected inner call to have 1 argument, got %d", len(innerCall.Args))
			}
		})
	}
}

func TestGrpcStreamContext(t *testing.T) {
	tests := []struct {
		name              string
		streamServerObject *dst.Ident
		wantSelectorName  string
	}{
		{
			name: "generates stream context call",
			streamServerObject: &dst.Ident{
				Name: "stream",
			},
			wantSelectorName: "Context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GrpcStreamContext(tt.streamServerObject)

			if got == nil {
				t.Fatal("GrpcStreamContext() returned nil")
			}

			// Check it's a selector expression
			selExpr, ok := got.Fun.(*dst.SelectorExpr)
			if !ok {
				t.Fatalf("expected Fun to be *dst.SelectorExpr, got %T", got.Fun)
			}

			// Check the X is the stream object
			xIdent, ok := selExpr.X.(*dst.Ident)
			if !ok {
				t.Fatalf("expected X to be *dst.Ident, got %T", selExpr.X)
			}
			if xIdent.Name != tt.streamServerObject.Name {
				t.Errorf("expected X name %q, got %q", tt.streamServerObject.Name, xIdent.Name)
			}

			// Check the selector is "Context"
			if selExpr.Sel.Name != tt.wantSelectorName {
				t.Errorf("expected selector name %q, got %q", tt.wantSelectorName, selExpr.Sel.Name)
			}

			// Verify that no args are passed
			if len(got.Args) != 0 {
				t.Errorf("expected 0 arguments, got %d", len(got.Args))
			}
		})
	}
}
