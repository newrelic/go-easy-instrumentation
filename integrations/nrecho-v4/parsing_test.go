package nrecho_test

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/newrelic/go-easy-instrumentation/integrations/nragent"
	"github.com/newrelic/go-easy-instrumentation/integrations/nrecho-v4"
	"github.com/newrelic/go-easy-instrumentation/parser"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestInstrumentEchoRouter(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "detect and trace echo router in main function",
			code: `package main
	import (
		echo "github.com/labstack/echo/v4"
	)

	func main() {
		e := echo.New()
		e.Start(":8000")
	}
`,
			expect: `package main

import (
	"time"

	echo "github.com/labstack/echo/v4"
	"github.com/newrelic/go-agent/v3/integrations/nrecho-v4"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	NewRelicAgent, agentInitError := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if agentInitError != nil {
		panic(agentInitError)
	}

	e := echo.New()
	e.Use(nrecho.Middleware(NewRelicAgent))
	e.Start(":8000")

	NewRelicAgent.Shutdown(5 * time.Second)
}
`,
		},
		{
			name: "detect and trace echo router in setup function",
			code: `package main

import (
	echo "github.com/labstack/echo/v4"
)

func setupRouter() *echo.Echo {
	e := echo.New()
	e.Start(":8000")
	return e
}

func main() {
	setupRouter()
}
`,
			expect: `package main

import (
	"time"

	echo "github.com/labstack/echo/v4"
	"github.com/newrelic/go-agent/v3/integrations/nrecho-v4"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func setupRouter(nrTxn *newrelic.Transaction) *echo.Echo {
	defer nrTxn.StartSegment("setupRouter").End()

	e := echo.New()
	e.Use(nrecho.Middleware(nrTxn.Application()))
	e.Start(":8000")
	return e
}

func main() {
	NewRelicAgent, agentInitError := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if agentInitError != nil {
		panic(agentInitError)
	}

	nrTxn := NewRelicAgent.StartTransaction("setupRouter")
	setupRouter(nrTxn)
	nrTxn.End()

	NewRelicAgent.Shutdown(5 * time.Second)
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := parser.RunStatelessTracingFunction(t, tt.code, nragent.InstrumentMain, nrecho.InstrumentEchoMiddleware)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestEchoMiddlewareCall(t *testing.T) {
	tests := []struct {
		name string
		stmt dst.Stmt
		want string
	}{
		{
			name: "detect echo middleware call - New",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "e",
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "New",
							Path: nrecho.EchoImportPath,
						},
					},
				},
			},
			want: "e",
		},
		{
			name: "detect echo middleware call - Incorrect Import Path",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "e",
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "New",
							Path: "blah",
						},
					},
				},
			},
			want: "",
		},
		{
			name: "detect echo middleware call - Multiple Rhs",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "e",
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "New",
							Path: nrecho.EchoImportPath,
						},
					},
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "New",
							Path: nrecho.EchoImportPath,
						},
					},
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer parser.PanicRecovery(t)
			got := nrecho.EchoMiddlewareCall(tt.stmt)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetEchoHandlerContext(t *testing.T) {
	contextParamName := &dst.Ident{Name: "c"}
	astContext := &ast.Ident{Name: "c"}
	echoHandlerType := &dst.FuncType{
		Params: &dst.FieldList{
			List: []*dst.Field{
				{
					Names: []*dst.Ident{
						{Name: "c"},
					},
					Type: &dst.Ident{
						Name: "Context",
						Path: nrecho.EchoImportPath,
					},
				},
			},
		},
	}
	echoHandlerTypeAST := &ast.FuncType{
		Params: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{
						{Name: "c"},
					},
					Type: &ast.SelectorExpr{
						X:   &ast.Ident{Name: "echo"},
						Sel: &ast.Ident{Name: "Context"},
					},
				},
			},
		},
	}
	pkg := &decorator.Package{
		Package: &packages.Package{
			TypesInfo: &types.Info{
				Types: map[ast.Expr]types.TypeAndValue{
					echoHandlerTypeAST.Params.List[0].Type: {
						Type: types.NewNamed(types.NewTypeName(token.NoPos, types.NewPackage("github.com/labstack/echo/v4", "github.com/labstack/echo/v4"), "Context", nil), nil, nil),
					},
					astContext: {
						Type: types.NewNamed(types.NewTypeName(token.NoPos, types.NewPackage("context", "context"), "Context", nil), nil, nil),
					},
				},
			},
		},
		Decorator: &decorator.Decorator{
			Map: decorator.Map{
				Ast: decorator.AstMap{
					Nodes: map[dst.Node]ast.Node{
						contextParamName:                    astContext,
						echoHandlerType:                     echoHandlerTypeAST,
						echoHandlerType.Params.List[0].Type: echoHandlerTypeAST.Params.List[0].Type,
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		node *dst.FuncType
		pkg  *decorator.Package
		want string
	}{
		{
			name: "valid echo handler",
			node: echoHandlerType,
			pkg:  pkg,
			want: "c",
		},
		{
			name: "invalid echo handler with multiple params",
			node: &dst.FuncType{
				Params: &dst.FieldList{
					List: []*dst.Field{
						{
							Names: []*dst.Ident{
								{Name: "c"},
							},
							Type: &dst.Ident{
								Name: "Context",
								Path: nrecho.EchoImportPath,
							},
						},
						{
							Names: []*dst.Ident{
								{Name: "d"},
							},
							Type: &dst.Ident{
								Name: "Context",
								Path: nrecho.EchoImportPath,
							},
						},
					},
				},
			},
			pkg:  pkg,
			want: "",
		},
		{
			name: "invalid echo handler with no names",
			node: &dst.FuncType{
				Params: &dst.FieldList{
					List: []*dst.Field{
						{
							Names: []*dst.Ident{},
							Type: &dst.Ident{
								Name: "Context",
								Path: nrecho.EchoImportPath,
							},
						},
					},
				},
			},
			pkg:  pkg,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok := nrecho.GetEchoContextFromHandler(tt.node, tt.pkg)
			if ok != tt.want {
				t.Errorf("expected %v, got %v", tt.want, ok)
			}
		})
	}
}

func TestDefineTxnFromEchoCtx(t *testing.T) {
	tests := []struct {
		name        string
		body        *dst.BlockStmt
		txnVariable string
		ctxName     string
		want        *dst.BlockStmt
	}{
		{
			name: "inject transaction into echo context",
			body: &dst.BlockStmt{
				List: []dst.Stmt{
					&dst.ExprStmt{
						X: &dst.CallExpr{
							Fun: &dst.Ident{Name: "doSomething"},
						},
					},
				},
			},
			txnVariable: "nrTxn",
			ctxName:     "c",
			want: &dst.BlockStmt{
				List: []dst.Stmt{
					&dst.AssignStmt{
						Lhs: []dst.Expr{
							&dst.Ident{
								Name: "nrTxn",
							},
						},
						Tok: token.DEFINE,
						Rhs: []dst.Expr{
							&dst.CallExpr{
								Fun: &dst.Ident{
									Name: "FromContext",
									Path: "github.com/newrelic/go-agent/v3/integrations/nrecho-v4",
								},
								Args: []dst.Expr{
									&dst.Ident{
										Name: "c",
									},
								},
							},
						},
						Decs: dst.AssignStmtDecorations{
							NodeDecs: dst.NodeDecs{
								Before: dst.NewLine,
							},
						},
					},
					&dst.ExprStmt{
						X: &dst.CallExpr{
							Fun: &dst.Ident{Name: "doSomething"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nrecho.DefineTxnFromEchoCtx(tt.body, tt.txnVariable, tt.ctxName)
			assert.Equal(t, tt.want, tt.body)
		})
	}
}
