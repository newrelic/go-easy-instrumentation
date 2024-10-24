package parser

import (
	"go/types"
	"testing"

	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/stretchr/testify/assert"
)

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
						Path: codegen.NewRelicAgentImportPath,
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
						Path: codegen.HttpImportPath,
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
	if err != nil {
		txn.NoticeError(err)
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
	_, t.err := http.Get("http://example.com")
	if t.err != nil {
		log.Fatal(t.err)
	}	
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
	_, t.err := http.Get("http://example.com")
	if t.err != nil {
		txn.NoticeError(t.err)
		log.Fatal(t.err)
	}
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			got := testStatefulTracingFunction(t, tt.code, NoticeError, true)
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
	defer nrTxn.StartSegment("myFunc").End()
	_, err := http.Get("http://example.com")
	if err != nil {
		nrTxn.NoticeError(err)
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
	defer nrTxn.StartSegment("myFunc").End()
	_, err := http.Get("http://example.com")
	if err != nil {
		nrTxn.NoticeError(err)
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
