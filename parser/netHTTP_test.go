package parser

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/stretchr/testify/assert"
)

func Test_isNetHttpClient(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		lineNum int
		want    bool
	}{
		{
			name: "define_new_http_client",
			code: `
package main
import "net/http"
func main() {
	client := &http.Client{}
}`,
			lineNum: 0,
			want:    true,
		},
		{
			name: "define_complex_http_client",
			code: `
package main
import "net/http"
func main() {
	client := &http.Client{
		Timeout: time.Second,
	}
}`,
			lineNum: 0,
			want:    true,
		},
		{
			name: "assign_http_client",
			code: `
package main
import "net/http"
func main() {
	client = &http.Client{}
}`,
			lineNum: 0,
			want:    false,
		},
		{
			name: "reassign_http_client",
			code: `
package main
import "net/http"
func main() {
	client := &http.Client{}
	client2 := client
}`,
			lineNum: 1,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgs := unitTest(t, tt.code)
			decl, ok := pkgs[0].Syntax[0].Decls[1].(*dst.FuncDecl)
			if !ok {
				t.Fatal("code must contain only one function declaration")
			}

			stmt, ok := decl.Body.List[tt.lineNum].(*dst.AssignStmt)
			if !ok {
				t.Fatal("lineNum must point to an assignment statement")
			}

			if got := isNetHttpClientDefinition(stmt); got != tt.want {
				t.Errorf("isNetHttpClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isNetHttpMethodCannotInstrument(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		lineNum      int
		wantBool     bool
		wantFuncName string
	}{
		{
			name: "http_get",
			code: `
package main
import "net/http"
func main() {
	http.Get("http://example.com")
}`,
			lineNum:      0,
			wantBool:     true,
			wantFuncName: "Get",
		},
		{
			name: "http_post",
			code: `
package main
import "net/http"
func main() {
	http.Post("http://example.com")
}`,
			lineNum:      0,
			wantBool:     true,
			wantFuncName: "Post",
		},
		{
			name: "http_post_form",
			code: `
package main
import "net/http"
func main() {
	http.PostForm("http://example.com")
}`,
			lineNum:      0,
			wantBool:     true,
			wantFuncName: "PostForm",
		},
		{
			name: "http_head",
			code: `
package main
import "net/http"
func main() {
	http.Head("http://example.com")
}`,
			lineNum:      0,
			wantBool:     true,
			wantFuncName: "Head",
		},
		{
			name: "http_client_get",
			code: `
package main
import "net/http"
func main() {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}
	client.Get("https://example.com")
}`,
			lineNum:      2,
			wantBool:     false,
			wantFuncName: "",
		},
		{
			name: "http_client_do",
			code: `
package main
import "net/http"
func main() {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	client.Do(req)
}`,
			lineNum:      2,
			wantBool:     false,
			wantFuncName: "",
		},
		{
			name: "http_get_complex_line",
			code: `
package main
import "net/http"
func main() {
	_, err := http.Get("http://example.com"); if err != nil {
		panic(err)
	}
}`,
			lineNum:      0,
			wantBool:     true,
			wantFuncName: "Get",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgs := unitTest(t, tt.code)
			decl, ok := pkgs[0].Syntax[0].Decls[1].(*dst.FuncDecl)
			if !ok {
				t.Fatal("code must contain only one function declaration")
			}

			gotFuncName, gotBool := isNetHttpMethodCannotInstrument(decl.Body.List[tt.lineNum])
			if gotBool != tt.wantBool {
				t.Errorf("isNetHttpMethodCannotInstrument() = %v, want %v", gotBool, tt.wantBool)
			}
			if gotFuncName != tt.wantFuncName {
				t.Errorf("isNetHttpMethodCannotInstrument() = %v, want %v", gotFuncName, tt.wantFuncName)
			}
		})
	}
}

func Test_isHttpHandler(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		wantBool bool
	}{
		{
			name: "http_get",
			code: `
package main
import "net/http"
func main() {
	http.Get("http://example.com")
}`,
			wantBool: false,
		},
		{
			name: "valid_handler",
			code: `
package main
import "net/http"
func index(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "hello world")
}`,
			wantBool: true,
		},
		{
			name: "overloaded_handler",
			code: `
package main
import "net/http"
func index(w http.ResponseWriter, r *http.Request, x string) {
	io.WriteString(w, x)
}`,
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgs := unitTest(t, tt.code)

			decl, ok := pkgs[0].Syntax[0].Decls[1].(*dst.FuncDecl)
			if !ok {
				t.Fatal("code must contain only one function declaration")
			}

			gotBool := isHttpHandler(decl, pkgs[0])
			if gotBool != tt.wantBool {
				t.Errorf("isNetHttpMethodCannotInstrument() = %v, want %v", gotBool, tt.wantBool)
			}
		})
	}
}

func Test_getNetHttpMethod(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		lineNum      int
		wantFuncName string
	}{
		{
			name: "http_get",
			code: `
package main
import "net/http"
func main() {
	http.Get("http://example.com")
}`,
			lineNum:      0,
			wantFuncName: "Get",
		},
		{
			name: "http_post",
			code: `
package main
import "net/http"
func main() {
	http.Post("http://example.com")
}`,
			lineNum:      0,
			wantFuncName: "Post",
		},
		{
			name: "http_get",
			code: `
package main
import "net/http"
func main() {
	http.Get("http://example.com")
}`,
			lineNum:      0,
			wantFuncName: "Get",
		},
		{
			name: "http_do",
			code: `
package main
import "net/http"
func main() {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	http.DefaultClient.Do(req)
}`,
			lineNum:      1,
			wantFuncName: "Do",
		},
		{
			name: "http_client_do",
			code: `
package main
import "net/http"
func main() {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	client.Do(req)
}`,
			lineNum:      2,
			wantFuncName: "Do",
		},
		{
			name: "complex_http_client_do",
			code: `
package main
import "net/http"
func main() {
	type clientInfo struct {
		client *http.Client
		name string
	}
	
	myClient := clientInfo{
		client: &http.Client{},
		name: "myClient",
	}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	myClient.client.Do(req)
}`,
			lineNum:      3,
			wantFuncName: "Do",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgs := unitTest(t, tt.code)

			decl, ok := pkgs[0].Syntax[0].Decls[1].(*dst.FuncDecl)
			if !ok {
				t.Fatal("code must contain only one function declaration")
			}

			expr, ok := decl.Body.List[tt.lineNum].(*dst.ExprStmt)
			if !ok {
				t.Fatal("lineNum must point to an expression statement")
			}

			call, ok := expr.X.(*dst.CallExpr)
			if !ok {
				t.Fatal("lineNum must point to an expression containing a call expression")
			}

			gotFuncName := getNetHttpMethod(call, pkgs[0])

			if gotFuncName != tt.wantFuncName {
				t.Errorf("isNetHttpMethodCannotInstrument() = %v, want %v", gotFuncName, tt.wantFuncName)
			}
		})
	}
}

func Test_GetNetHttpClientVariableName(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		lineNum  int
		wantName string
	}{
		{
			name: "no client",
			code: `
package main
import "net/http"
func main() {
	http.Get("http://example.com")
}`,
			lineNum:  0,
			wantName: "",
		},
		{
			name: "http_do",
			code: `
package main
import "net/http"
func main() {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	http.DefaultClient.Do(req)
}`,
			lineNum:  1,
			wantName: "DefaultClient",
		},
		{
			name: "http_client_do",
			code: `
package main
import "net/http"
func main() {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	client.Do(req)
}`,
			lineNum:  2,
			wantName: "",
		},
		{
			name: "complex_http_client_do",
			code: `
package main
import "net/http"
func main() {
	type clientInfo struct {
		client *http.Client
		name string
	}
	
	myClient := clientInfo{
		client: &http.Client{},
		name: "myClient",
	}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	myClient.client.Do(req)
}`,
			lineNum:  3,
			wantName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgs := unitTest(t, tt.code)

			decl, ok := pkgs[0].Syntax[0].Decls[1].(*dst.FuncDecl)
			if !ok {
				t.Fatal("code must contain only one function declaration")
			}

			expr, ok := decl.Body.List[tt.lineNum].(*dst.ExprStmt)
			if !ok {
				t.Fatal("lineNum must point to an expression statement")
			}

			call, ok := expr.X.(*dst.CallExpr)
			if !ok {
				t.Fatal("lineNum must point to an expression containing a call expression")
			}

			gotFuncName := getNetHttpClientVariableName(call, pkgs[0])

			if gotFuncName != tt.wantName {
				t.Errorf("isNetHttpMethodCannotInstrument() = %v, want %v", gotFuncName, tt.wantName)
			}
		})
	}
}

func Test_cannotTraceOutboundHttp(t *testing.T) {
	type args struct {
		method string
		decs   *dst.NodeDecs
	}
	tests := []struct {
		name       string
		args       args
		wantBuffer bool
	}{
		{
			name: "http_get",
			args: args{
				method: "Get",
				decs:   &dst.NodeDecs{},
			},
			wantBuffer: false,
		},
		{
			name: "http_get",
			args: args{
				method: "Get",
				decs: &dst.NodeDecs{
					Start: []string{"// this is a comment"},
				},
			},
			wantBuffer: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cannotTraceOutboundHttp(tt.args.method, tt.args.decs)
			if tt.wantBuffer && got[len(got)-1] != "//" {
				t.Errorf("cannotTraceOutboundHttp() should add a comment ending in \"//\" but did NOT for method %s with decs %+v", tt.args.method, tt.args.decs)
			}
			if !tt.wantBuffer && got[len(got)-1] == "//" {
				t.Errorf("cannotTraceOutboundHttp() should NOT add a comment ending in \"//\" but did for method %s with decs %+v", tt.args.method, tt.args.decs)
			}
		})
	}
}

func Test_TxnFromCtx(t *testing.T) {
	type args struct {
		fn          *dst.FuncDecl
		txnVariable string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "txn_from_ctx",
			args: args{
				fn: &dst.FuncDecl{
					Body: &dst.BlockStmt{
						List: []dst.Stmt{},
					},
				},
				txnVariable: "txn",
			},
		},
		{
			name: "txn_from_ctx",
			args: args{
				fn: &dst.FuncDecl{
					Body: &dst.BlockStmt{
						List: []dst.Stmt{
							&dst.ReturnStmt{},
						},
					},
				},
				txnVariable: "txn",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectStmt := codegen.TxnFromContext(tt.args.txnVariable, codegen.HttpRequestContext())
			defineTxnFromCtx(tt.args.fn, tt.args.txnVariable)
			if !reflect.DeepEqual(tt.args.fn.Body.List[0], expectStmt) {
				t.Errorf("expected the function body to contain the statement %v but got %v", expectStmt, tt.args.fn.Body.List[0])
			}
		})
	}
}

func Test_getHttpResponseVariable(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		linenum  int
		wantExpr dst.Expr
	}{
		{
			name: "basic response assignment",
			code: `
package main
import "net/http"
func main() {
	a := &http.Response{}
}`,
			linenum:  0,
			wantExpr: dst.NewIdent("a"),
		},
		{
			name: "capture assignment from http.Get",
			code: `
package main
import "net/http"
func main() {
	resp, err := http.Get("http://example.com")
}`,
			linenum:  0,
			wantExpr: dst.NewIdent("resp"),
		},
		{
			name: "no response assigned",
			code: `
package main
import "net/http"
func main() {
	a := &http.Client{}
}`,
			linenum:  0,
			wantExpr: nil,
		},
		{
			name: "response is assigned to complex object",
			code: `
package main
import "net/http"
func main() {
	type respInfo struct {
		response *http.Response
		notes string
	}
	info := respInfo{}
	info.response := &http.Client{}
}`,
			linenum: 2,
			wantExpr: &dst.SelectorExpr{
				X:   dst.NewIdent("info"),
				Sel: dst.NewIdent("response"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := pseudo_uuid()
			if err != nil {
				t.Fatal(err)
			}

			testDir := fmt.Sprintf("tmp_%s", id)
			defer cleanTestApp(t, testDir)

			manager := testInstrumentationManager(t, tt.code, testDir)
			pkg := manager.getDecoratorPackage()
			stmt := pkg.Syntax[0].Decls[1].(*dst.FuncDecl).Body.List[tt.linenum]
			gotExpr := getHttpResponseVariable(manager, stmt)
			switch expect := tt.wantExpr.(type) {
			case *dst.Ident:
				got, ok := gotExpr.(*dst.Ident)
				if !ok {
					t.Fatalf("expected expression to be an identifier but got %T", gotExpr)
				}
				if got.Name != expect.Name {
					t.Errorf("expected getHttpResponseVariable() to return an identifier with the name \"%s\" but got \"%s\"", expect.Name, got.Name)
				}
			case *dst.SelectorExpr:
				got, ok := gotExpr.(*dst.SelectorExpr)
				if !ok {
					t.Fatalf("expected expression to be a selector expression but got %T", gotExpr)
				}
				if got.Sel.Name != expect.Sel.Name {
					t.Errorf("expected getHttpResponseVariable() to return a selector expression with the selector \"%s\" but got \"%s\"", expect.Sel.Name, got.Sel.Name)
				}
				x, ok := got.X.(*dst.Ident)
				if !ok {
					t.Fatalf("expected the returned selector expression to have an identifier as the X but got %T", got.X)
				}
				if x.Name != expect.X.(*dst.Ident).Name {
					t.Errorf("expected getHttpResponseVariable() to return a selector expression with the X identifier named \"%s\" but got \"%s\"", expect.X.(*dst.Ident).Name, x.Name)
				}
			case nil:
				if gotExpr != nil {
					t.Errorf("expected getHttpResponseVariable() to return nil but got %T", gotExpr)
				}
			default:
				// catch all
				assert.Equal(t, tt.wantExpr, gotExpr)
			}
		})
	}
}

func TestExternalHttpCall(t *testing.T) {

	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "no http do method",
			code: `
package main

import "net/http"

func main() {
	a := &http.Response{}
}
`,
			expect: `package main

import "net/http"

func main() {
	a := &http.Response{}
}
`,
		},
		{
			name: "default client do",
			code: `
package main

import "net/http"

func main() {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	http.DefaultClient.Do(req)
}
`,
			expect: `package main

import (
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	externalSegment := newrelic.StartExternalSegment(txn, req)
	http.DefaultClient.Do(req)
	externalSegment.End()
}
`,
		},
		{
			name: "default client do captures http response",
			code: `
package main

import "net/http"

func main() {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, _ := http.DefaultClient.Do(req)
}
`,
			expect: `package main

import (
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	externalSegment := newrelic.StartExternalSegment(txn, req)
	resp, _ := http.DefaultClient.Do(req)
	externalSegment.Response = resp
	externalSegment.End()
}
`,
		},
		{
			name: "custom client do",
			code: `
package main

import "net/http"

func main() {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	client.Do(req)
}
`,
			expect: `package main

import (
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req = newrelic.RequestWithTransactionContext(req, txn)
	client.Do(req)
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatefulTracingFunction(t, tt.code, ExternalHttpCall, true)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestWrapNestedHandleFunction(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "trace nested handle function",
			code: `
package main

import (
	"net/http"
)

func main() {
	http.HandleFunc("/", index)
`,
			expect: `package main

import (
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() { http.HandleFunc(newrelic.WrapHandleFunc(txn.Application(), "/", index)) }
`,
		},
		{
			name: "trace nested mux handle function",
			code: `
package main

import (
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.Handle("/", index)
}
`,
			expect: `package main

import (
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	mux := http.NewServeMux()
	mux.Handle(newrelic.WrapHandleFunc(txn.Application(), "/", index))
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatefulTracingFunction(t, tt.code, WrapNestedHandleFunction, true)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestCannotInstrumentHttpMethod(t *testing.T) {

	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "http get",
			code: `package main

import "net/http"

func main() {
	http.Get("http://example.com")
}
`,
			expect: `package main

import "net/http"

func main() {
	// the "http.Get()" net/http method can not be instrumented and its outbound traffic can not be traced
	// please see these examples of code patterns for external http calls that can be instrumented:
	// https://docs.newrelic.com/docs/apm/agents/go-agent/configuration/distributed-tracing-go-agent/#make-http-requests
	http.Get("http://example.com")
}
`,
		},
		{
			name: "http post",
			code: `package main

import "net/http"

func main() {
	http.Post("http://example.com")
}
`,
			expect: `package main

import "net/http"

func main() {
	// the "http.Post()" net/http method can not be instrumented and its outbound traffic can not be traced
	// please see these examples of code patterns for external http calls that can be instrumented:
	// https://docs.newrelic.com/docs/apm/agents/go-agent/configuration/distributed-tracing-go-agent/#make-http-requests
	http.Post("http://example.com")
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, CannotInstrumentHttpMethod)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestInstrumentHttpClient(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "basic client definition",
			code: `package main

import "net/http"

func main() {
	client := &http.Client{}
}
`,
			expect: `package main

import (
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	client := &http.Client{}
	client.Transport = newrelic.NewRoundTripper(client.Transport)
}
`,
		},
		{
			name: "complex client definition",
			code: `package main

import "net/http"

func main() {
	type clientInfo struct {
		client *http.Client
	}
	info := clientInfo{}
	info.client := &http.Client{}
}
`,
			expect: `package main

import (
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	type clientInfo struct {
		client *http.Client
	}
	info := clientInfo{}
	info.client := &http.Client{}
	info.client.Transport = newrelic.NewRoundTripper(info.client.Transport)
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, InstrumentHttpClient)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestInstrumentHandleFunction(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "do not modify handle funcs without additional tracing",
			code: `package main

import "net/http"

func myHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello world"))
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
			expect: `package main

import "net/http"

func myHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello world"))
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
		},
		{
			name: "handle funcs with tracing get transaction pulled out of request object",
			code: `package main

import "net/http"

func myHandler(w http.ResponseWriter, r *http.Request) {
	_, err := http.Get("http://example.com"); if err != nil {
		panic(err)
	}
	w.Write([]byte("hello world"))
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
			expect: `package main

import (
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func myHandler(w http.ResponseWriter, r *http.Request) {
	nrTxn := newrelic.FromContext(r.Context())

	_, err := http.Get("http://example.com")
	if err != nil {
		nrTxn.NoticeError(err)
		panic(err)
	}
	w.Write([]byte("hello world"))
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, InstrumentHandleFunction)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestDownstreamTracingFromHandleFunction(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect string
	}{
		{
			name: "tracing propogated to all downstream calls",
			code: `package main

import "net/http"

func myHelperFunction(url string) error {
	_, err := http.Get(url)
	if err != nil {
		return err
	}
	return nil
}

func myHandler(w http.ResponseWriter, r *http.Request) {
	err := myHelperFunction("http://example.com")
	if err != nil {
		panic(err)
	}
	
	w.Write([]byte("hello world"))
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
			expect: `package main

import (
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func myHelperFunction(url string, nrTxn *newrelic.Transaction) error {
	defer nrTxn.StartSegment("myHelperFunction").End()
	_, err := http.Get(url)
	if err != nil {
		nrTxn.NoticeError(err)
		return err
	}
	return nil
}

func myHandler(w http.ResponseWriter, r *http.Request) {
	nrTxn := newrelic.FromContext(r.Context())

	err := myHelperFunction("http://example.com", nrTxn)
	if err != nil {
		panic(err)
	}
	w.Write([]byte("hello world"))
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
		},
		{
			name: "tracing propogated to downstream calls without captures",
			code: `package main

import "net/http"

func myHelperFunction(url string) bool {
	if url == "http://error.com" {
		return false
	} 
	return true
}

func myHandler(w http.ResponseWriter, r *http.Request) {
	if myHelperFunction("http://example.com") {
		w.Write([]byte("hello world"))
	}
	http.Error(w, "I am an error", 400)
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
			expect: `package main

import (
	"net/http"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func myHelperFunction(url string, nrTxn *newrelic.Transaction) bool {
	defer nrTxn.StartSegment("myHelperFunction").End()
	if url == "http://error.com" {
		return false
	}
	return true
}

func myHandler(w http.ResponseWriter, r *http.Request) {
	nrTxn := newrelic.FromContext(r.Context())

	if myHelperFunction("http://example.com", nrTxn) {
		w.Write([]byte("hello world"))
	}
	http.Error(w, "I am an error", 400)
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
		},
		{
			name: "tracing propogated to async downstream calls",
			code: `package main

import (
	"net/http"
	"sync"
)

func myHelperFunction(url string, wg *sync.WaitGroup){
	defer wg.Done()
	_, err := http.Get(url)
	if err != nil {
		panic(err)
	}
}

func myHandler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go myHelperFunction("http://example.com", wg)
	}
	wg.Wait()

	w.Write([]byte("hello world"))
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
			expect: `package main

import (
	"net/http"
	"sync"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func myHelperFunction(url string, wg *sync.WaitGroup, nrTxn *newrelic.Transaction) {
	defer nrTxn.StartSegment("async myHelperFunction").End()
	defer wg.Done()
	_, err := http.Get(url)
	if err != nil {
		nrTxn.NoticeError(err)
		panic(err)
	}
}

func myHandler(w http.ResponseWriter, r *http.Request) {
	nrTxn := newrelic.FromContext(r.Context())

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go myHelperFunction("http://example.com", wg, nrTxn.NewGoroutine())
	}
	wg.Wait()

	w.Write([]byte("hello world"))
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
		},
		{
			name: "tracing propogated to async literal downstream calls",
			code: `package main

import (
	"net/http"
	"sync"
)

func myHelperFunction(url string) {
	_, err := http.Get(url)
	if err != nil {
		panic(err)
	}
}

func myHandler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			myHelperFunction("http://example.com")
		}()
	}
	wg.Wait()

	w.Write([]byte("hello world"))
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
			expect: `package main

import (
	"net/http"
	"sync"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func myHelperFunction(url string, nrTxn *newrelic.Transaction) {
	defer nrTxn.StartSegment("myHelperFunction").End()
	_, err := http.Get(url)
	if err != nil {
		nrTxn.NoticeError(err)
		panic(err)
	}
}

func myHandler(w http.ResponseWriter, r *http.Request) {
	nrTxn := newrelic.FromContext(r.Context())

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(nrTxn *newrelic.Transaction) {
			defer nrTxn.StartSegment("async literal").End()
			defer wg.Done()
			myHelperFunction("http://example.com", nrTxn)
		}(nrTxn.NewGoroutine())
	}
	wg.Wait()

	w.Write([]byte("hello world"))
}

func main() {
	http.HandleFunc("/", myHandler)
	http.ListenAndServe(":8080", nil)
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatelessTracingFunction(t, tt.code, InstrumentHandleFunction)
			assert.Equal(t, tt.expect, got)
		})
	}
}
