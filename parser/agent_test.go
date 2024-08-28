package parser

import (
	"go/token"
	"go/types"
	"reflect"
	"testing"

	"github.com/dave/dst"
	"github.com/stretchr/testify/assert"
)

func Test_panicOnError(t *testing.T) {
	tests := []struct {
		name string
		want *dst.IfStmt
	}{
		{
			name: "Test panic on error",
			want: &dst.IfStmt{
				Cond: &dst.BinaryExpr{
					X: &dst.Ident{
						Name: "err",
					},
					Op: token.NEQ,
					Y: &dst.Ident{
						Name: "nil",
					},
				},
				Body: &dst.BlockStmt{
					List: []dst.Stmt{
						&dst.ExprStmt{
							X: &dst.CallExpr{
								Fun: &dst.Ident{
									Name: "panic",
								},
								Args: []dst.Expr{
									&dst.Ident{
										Name: "err",
									},
								},
							},
						},
					},
				},
				Decs: dst.IfStmtDecorations{
					NodeDecs: dst.NodeDecs{
						After: dst.EmptyLine,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := panicOnError(); !reflect.DeepEqual(got, tt.want) {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func Test_createAgentAST(t *testing.T) {
	type args struct {
		AppName           string
		AgentVariableName string
	}
	tests := []struct {
		name string
		args args
		want []dst.Stmt
	}{
		{
			name: "Test create agent AST",
			args: args{
				AgentVariableName: "testAgent",
			},
			want: []dst.Stmt{&dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "testAgent",
					},
					&dst.Ident{
						Name: "err",
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "NewApplication",
							Path: newrelicAgentImport,
						},
						Args: []dst.Expr{
							&dst.CallExpr{
								Fun: &dst.Ident{
									Path: newrelicAgentImport,
									Name: "ConfigFromEnvironment",
								},
							},
						},
					},
				},
			}, panicOnError()},
		},
		{
			name: "Test create agent AST with AppName",
			args: args{
				AgentVariableName: "testAgent",
				AppName:           "testApp",
			},
			want: []dst.Stmt{&dst.AssignStmt{
				Lhs: []dst.Expr{
					&dst.Ident{
						Name: "testAgent",
					},
					&dst.Ident{
						Name: "err",
					},
				},
				Tok: token.DEFINE,
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Fun: &dst.Ident{
							Name: "NewApplication",
							Path: newrelicAgentImport,
						},
						Args: []dst.Expr{
							&dst.CallExpr{
								Fun: &dst.Ident{
									Path: newrelicAgentImport,
									Name: "ConfigAppName",
								},
								Args: []dst.Expr{
									&dst.BasicLit{
										Kind:  token.STRING,
										Value: `"testApp"`,
									},
								},
							},
							&dst.CallExpr{
								Fun: &dst.Ident{
									Path: newrelicAgentImport,
									Name: "ConfigFromEnvironment",
								},
							},
						},
					},
				},
			}, panicOnError()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, createAgentAST(tt.args.AppName, tt.args.AgentVariableName))
		})
	}
}

func Test_shutdownAgent(t *testing.T) {
	type args struct {
		AgentVariableName string
	}
	tests := []struct {
		name string
		args args
		want *dst.ExprStmt
	}{
		{
			name: "Test shutdown agent",
			args: args{
				AgentVariableName: "testAgent",
			},
			want: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "testAgent",
						},
						Sel: &dst.Ident{
							Name: "Shutdown",
						},
					},
					Args: []dst.Expr{
						&dst.BinaryExpr{
							X: &dst.BasicLit{
								Kind:  token.INT,
								Value: "5",
							},
							Op: token.MUL,
							Y: &dst.Ident{
								Name: "Second",
								Path: "time",
							},
						},
					},
				},
				Decs: dst.ExprStmtDecorations{
					NodeDecs: dst.NodeDecs{
						Before: dst.EmptyLine,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shutdownAgent(tt.args.AgentVariableName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_startTransaction(t *testing.T) {
	type args struct {
		appVariableName         string
		transactionVariableName string
		transactionName         string
		overwriteVariable       bool
	}
	tests := []struct {
		name string
		args args
		want *dst.AssignStmt
	}{
		{
			name: "Test start transaction",
			args: args{
				appVariableName:         "testApp",
				transactionVariableName: "testTxn",
				transactionName:         "testTxnName",
				overwriteVariable:       false,
			},
			want: &dst.AssignStmt{
				Lhs: []dst.Expr{dst.NewIdent("testTxn")},
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Args: []dst.Expr{
							&dst.BasicLit{
								Kind:  token.STRING,
								Value: `"testTxnName"`,
							},
						},
						Fun: &dst.SelectorExpr{
							X:   dst.NewIdent("testApp"),
							Sel: dst.NewIdent("StartTransaction"),
						},
					},
				},
				Tok: token.DEFINE,
			},
		},
		{
			name: "Test start transaction with overwrite",
			args: args{
				appVariableName:         "testApp",
				transactionVariableName: "testTxn",
				transactionName:         "testTxnName",
				overwriteVariable:       true,
			},
			want: &dst.AssignStmt{
				Lhs: []dst.Expr{dst.NewIdent("testTxn")},
				Rhs: []dst.Expr{
					&dst.CallExpr{
						Args: []dst.Expr{
							&dst.BasicLit{
								Kind:  token.STRING,
								Value: `"testTxnName"`,
							},
						},
						Fun: &dst.SelectorExpr{
							X:   dst.NewIdent("testApp"),
							Sel: dst.NewIdent("StartTransaction"),
						},
					},
				},
				Tok: token.ASSIGN,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := startTransaction(tt.args.appVariableName, tt.args.transactionVariableName, tt.args.transactionName, tt.args.overwriteVariable)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_endTransaction(t *testing.T) {
	type args struct {
		transactionVariableName string
	}
	tests := []struct {
		name string
		args args
		want *dst.ExprStmt
	}{
		{
			name: "Test end transaction",
			args: args{
				transactionVariableName: "testTxn",
			},
			want: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X:   dst.NewIdent("testTxn"),
						Sel: dst.NewIdent("End"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := endTransaction(tt.args.transactionVariableName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_txnAsParameter(t *testing.T) {
	type args struct {
		txnName string
	}
	tests := []struct {
		name string
		args args
		want *dst.Field
	}{
		{
			name: "Test txn as parameter",
			args: args{
				txnName: "testTxn",
			},
			want: &dst.Field{
				Names: []*dst.Ident{
					{
						Name: "testTxn",
					},
				},
				Type: &dst.StarExpr{
					X: &dst.Ident{
						Name: "Transaction",
						Path: newrelicAgentImport,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := txnAsParameter(tt.args.txnName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_deferSegment(t *testing.T) {
	type args struct {
		segmentName string
		txnVarName  string
	}
	tests := []struct {
		name string
		args args
		want *dst.DeferStmt
	}{
		{
			name: "Test defer segment",
			args: args{
				segmentName: "testSegment",
				txnVarName:  "testTxn",
			},
			want: &dst.DeferStmt{
				Call: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.CallExpr{
							Fun: &dst.SelectorExpr{
								X: dst.NewIdent("testTxn"),
								Sel: &dst.Ident{
									Name: "StartSegment",
								},
							},
							Args: []dst.Expr{
								&dst.BasicLit{
									Kind:  token.STRING,
									Value: `"testSegment"`,
								},
							},
						},
						Sel: &dst.Ident{
							Name: "End",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deferSegment(tt.args.segmentName, tt.args.txnVarName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_txnNewGoroutine(t *testing.T) {
	type args struct {
		txnVarName string
	}
	tests := []struct {
		name string
		args args
		want *dst.CallExpr
	}{
		{
			name: "Test txn new goroutine",
			args: args{
				txnVarName: "testTxn",
			},
			want: &dst.CallExpr{
				Fun: &dst.SelectorExpr{
					X: &dst.Ident{
						Name: "testTxn",
					},
					Sel: &dst.Ident{
						Name: "NewGoroutine",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := txnNewGoroutine(tt.args.txnVarName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_isNamedError(t *testing.T) {
	type args struct {
		n *types.Named
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test is named error",
			args: args{
				n: types.NewNamed(types.NewTypeName(0, nil, "error", nil), nil, nil),
			},
			want: true,
		},
		{
			name: "Test is not error",
			args: args{
				n: types.NewNamed(types.NewTypeName(0, nil, "foo", nil), nil, nil),
			},
			want: false,
		},
		{
			name: "Nil Named Error",
			args: args{
				n: nil,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNamedError(tt.args.n); got != tt.want {
				t.Errorf("isNamedError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isNewRelicMethod(t *testing.T) {
	type args struct {
		call *dst.CallExpr
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Decorated DST New Relic Method",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "txn",
						Path: newrelicAgentImport,
					},
				},
			},
			want: true,
		},
		{
			name: "AST Style New Relic Method",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "newrelic",
						},
						Sel: &dst.Ident{
							Name: "StartTransaction",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Non New Relic Method",
			args: args{
				call: &dst.CallExpr{
					Fun: &dst.Ident{
						Name: "Get",
						Path: netHttpPath,
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNewRelicMethod(tt.args.call); got != tt.want {
				t.Errorf("isNewRelicMethod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generateNoticeError(t *testing.T) {
	type args struct {
		errExpr  dst.Expr
		txnName  string
		nodeDecs *dst.NodeDecs
	}
	tests := []struct {
		name string
		args args
		want *dst.ExprStmt
	}{
		{
			name: "generate notice error",
			args: args{
				errExpr:  dst.NewIdent("err"),
				txnName:  "txn",
				nodeDecs: nil,
			},
			want: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "txn",
						},
						Sel: &dst.Ident{
							Name: "NoticeError",
						},
					},
					Args: []dst.Expr{
						&dst.Ident{
							Name: "err",
						},
					},
				},
			},
		},
		{
			name: "generate notice error",
			args: args{
				errExpr: dst.NewIdent("err"),
				txnName: "txn",
				nodeDecs: &dst.NodeDecs{
					After: dst.NewLine,
					End:   dst.Decorations{"// end"},
				},
			},
			want: &dst.ExprStmt{
				X: &dst.CallExpr{
					Fun: &dst.SelectorExpr{
						X: &dst.Ident{
							Name: "txn",
						},
						Sel: &dst.Ident{
							Name: "NoticeError",
						},
					},
					Args: []dst.Expr{
						&dst.Ident{
							Name: "err",
						},
					},
				},
				Decs: dst.ExprStmtDecorations{
					NodeDecs: dst.NodeDecs{
						After: dst.NewLine,
						End:   dst.Decorations{"// end"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateNoticeError(tt.args.errExpr, tt.args.txnName, tt.args.nodeDecs)
			assert.Equal(t, tt.want, got, "generated notice error expression is not correct")
			if tt.args.nodeDecs != nil {
				emptyDecoration := dst.Decorations{}
				emptyDecoration.Clear()
				assert.Equal(t, dst.None, tt.args.nodeDecs.After, "passed node decorations `After` should be `None`")
				assert.Equal(t, emptyDecoration, tt.args.nodeDecs.End, "passed node decorations `End` should be cleared")
			}
		})
	}
}

func Test_noticeError(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "notice an error",
			code: `package main

import (
	"log"
	"net/http"
)

func main() {
	_, err := http.Get("http://example.com")
	if err != nil {
		log.Fatal(err)
	}
}
`,
			expect: `package main

import (
	"log"
	"net/http"
)

func main() {
	_, err := http.Get("http://example.com")
	txn.NoticeError(err)
	if err != nil {
		log.Fatal(err)
	}
}
`,
		},
		{
			name: "error return ignored",
			code: `package main

import (
	"net/http"
)

func main() {
	_, _ := http.Get("http://example.com")
}
`,
			expect: `package main

import (
	"net/http"
)

func main() {
	_, _ := http.Get("http://example.com")
}
`,
		},
		{
			name: "error value stored in struct",
			code: `package main

import (
	"net/http"
)

func main() {
	type test struct {
		err error
	}
	t := test{}
	_, t.err = http.Get("http://example.com")
}
`,
			expect: `package main

import (
	"net/http"
)

func main() {
	type test struct {
		err error
	}
	t := test{}
	_, t.err = http.Get("http://example.com")
	txn.NoticeError(t.err)
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatefulTracingFunction(t, tt.code, NoticeError)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestInstrumentMain(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "function with tracing",
			code: `package main

import "net/http"

func myFunc() {
	_, err := http.Get("http://example.com")
	if err != nil {
		panic(err)
	}
}

func main() {
	myFunc()
}
`,
			expect: `package main

import (
	"net/http"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func myFunc(nrTxn *newrelic.Transaction) {
	_, err := http.Get("http://example.com")
	nrTxn.NoticeError(err)
	if err != nil {
		panic(err)
	}
}

func main() {
	NewRelicAgent, err := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if err != nil {
		panic(err)
	}

	nrTxn := NewRelicAgent.StartTransaction("myFunc")
	myFunc(nrTxn)
	nrTxn.End()

	NewRelicAgent.Shutdown(5 * time.Second)
}
`,
		},
		{
			name: "re-assigns transaction variable when repeated",
			code: `package main

import "net/http"

func myFunc() {
	_, err := http.Get("http://example.com")
	if err != nil {
		panic(err)
	}
}

func main() {
	myFunc()
	myFunc()
}
`,
			expect: `package main

import (
	"net/http"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func myFunc(nrTxn *newrelic.Transaction) {
	_, err := http.Get("http://example.com")
	nrTxn.NoticeError(err)
	if err != nil {
		panic(err)
	}
}

func main() {
	NewRelicAgent, err := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if err != nil {
		panic(err)
	}

	nrTxn := NewRelicAgent.StartTransaction("myFunc")
	myFunc(nrTxn)
	nrTxn.End()
	nrTxn = NewRelicAgent.StartTransaction("myFunc")
	myFunc(nrTxn)
	nrTxn.End()

	NewRelicAgent.Shutdown(5 * time.Second)
}
`,
		},
		{
			name: "ignore async functions in main",
			code: `package main

import "net/http"

func myFunc() {
	_, err := http.Get("http://example.com")
	if err != nil {
		panic(err)
	}
}

func main() {
	go myFunc()
}
`,
			expect: `package main

import (
	"net/http"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func myFunc() {
	_, err := http.Get("http://example.com")
	if err != nil {
		panic(err)
	}
}

func main() {
	NewRelicAgent, err := newrelic.NewApplication(newrelic.ConfigFromEnvironment())
	if err != nil {
		panic(err)
	}

	go myFunc()

	NewRelicAgent.Shutdown(5 * time.Second)
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, InstrumentMain)
			assert.Equal(t, tt.expect, got)
		})
	}
}
