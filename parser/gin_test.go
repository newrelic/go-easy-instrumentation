package parser

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestInstrumentGinRouter(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "detect and trace gin router in main function",
			code: `package main
	import (
		"github.com/gin-gonic/gin"
	)

	func main() {
		router := gin.Default()
		router.Run(":8000")
	}
`,
			expect: `package main

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	NewRelicAgent, err := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if err != nil {
		panic(err)
	}

	router := gin.Default()
	router.Use(nrgin.Middleware(NewRelicAgent))
	router.Run(":8000")

	NewRelicAgent.Shutdown(5 * time.Second)
}
`,
		},
		{
			name: "detect and trace gin router in setup function",
			code: `package main

import (
	"github.com/gin-gonic/gin"
)

func setupRouter(){
	router := gin.Default()
	router.Run(":8000")
}

func main() {
	setupRouter()
}
`,
			expect: `package main

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
	"github.com/newrelic/go-agent/v3/newrelic"
)

func setupRouter(nrTxn *newrelic.Transaction) {
	defer nrTxn.StartSegment("setupRouter").End()

	router := gin.Default()
	router.Use(nrgin.Middleware(nrTxn.Application()))
	router.Run(":8000")
}

func main() {
	NewRelicAgent, err := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if err != nil {
		panic(err)
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
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, InstrumentMain, InstrumentGinMiddleware)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestGinMiddlewareCall(t *testing.T) {
	tests := []struct {
		name string
		stmt dst.Stmt
		want string
	}{
		{
			name: "detect gin middleware call - Default",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "router",
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "Default",
							Path: ginImportPath,
						},
					},
				},
			},
			want: "router",
		},
		{
			name: "detect gin middleware call - New",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "router",
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "New",
							Path: ginImportPath,
						},
					},
				},
			},
			want: "router",
		},
		{
			name: "detect gin middleware call - Incorrect Import Path",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "router",
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
			name: "detect gin middleware call - Incorrect Import Path",
			stmt: &dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "router",
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "New",
							Path: ginImportPath,
						},
					},
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "Default",
							Path: ginImportPath,
						},
					},
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := ginMiddlewareCall(tt.stmt)
			assert.Equal(t, tt.want, got)
		})
	}

}
func TestGetGinHandlerContext(t *testing.T) {
	contextParamName := &dst.Ident{Name: "c"}
	astContext := &ast.Ident{Name: "c"}
	ginHandlerType := &dst.FuncType{
		Params: &dst.FieldList{
			List: []*dst.Field{
				{
					Names: []*dst.Ident{
						{Name: "c"},
					},
					Type: &dst.Ident{
						Name: "Context",
						Path: ginImportPath,
					},
				},
			},
		},
	}
	ginHandlerTypeAST := &ast.FuncType{
		Params: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{
						{Name: "c"},
					},
					Type: &ast.SelectorExpr{
						X:   &ast.Ident{Name: "gin"},
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
					ginHandlerTypeAST.Params.List[0].Type: {
						Type: types.NewPointer(types.NewNamed(types.NewTypeName(token.NoPos, types.NewPackage("github.com/gin-gonic/gin", "github.com/gin-gonic/gin"), "Context", nil), nil, nil)),
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
						contextParamName:                   astContext,
						ginHandlerType:                     ginHandlerTypeAST,
						ginHandlerType.Params.List[0].Type: ginHandlerTypeAST.Params.List[0].Type,
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
			name: "valid gin handler",
			node: ginHandlerType,
			pkg:  pkg,
			want: "c",
		},
		{
			name: "invalid gin handler with multiple params",
			node: &dst.FuncType{
				Params: &dst.FieldList{
					List: []*dst.Field{
						{
							Names: []*dst.Ident{
								{Name: "c"},
							},
							Type: &dst.Ident{
								Name: "Context",
								Path: ginImportPath,
							},
						},
						{
							Names: []*dst.Ident{
								{Name: "d"},
							},
							Type: &dst.Ident{
								Name: "Context",
								Path: ginImportPath,
							},
						},
					},
				},
			},
			pkg:  pkg,
			want: "",
		},
		{
			name: "invalid gin handler with no names",
			node: &dst.FuncType{
				Params: &dst.FieldList{
					List: []*dst.Field{
						{
							Names: []*dst.Ident{},
							Type: &dst.Ident{
								Name: "Context",
								Path: ginImportPath,
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
			ok := getGinContextFromHandler(tt.node, tt.pkg)
			if ok != tt.want {
				t.Errorf("expected %v, got %v", tt.want, ok)
			}
		})
	}
}

func TestDefineTxnFromGinCtx(t *testing.T) {
	tests := []struct {
		name        string
		body        *dst.BlockStmt
		txnVariable string
		ctxName     string
		want        *dst.BlockStmt
	}{
		{
			name: "inject transaction into gin context",
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
									Name: "Transaction",
									Path: "github.com/newrelic/go-agent/v3/integrations/nrgin",
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
			defineTxnFromGinCtx(tt.body, tt.txnVariable, tt.ctxName)
			assert.Equal(t, tt.want, tt.body)
		})
	}
}
