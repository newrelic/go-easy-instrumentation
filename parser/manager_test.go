package parser

import (
	"reflect"
	"testing"

	"github.com/dave/dst"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
	"github.com/stretchr/testify/assert"
)

func Test_AddImport(t *testing.T) {
	type fields struct {
		userAppPath       string
		diffFile          string
		appName           string
		agentVariableName string
		currentPackage    string
		packages          map[string]*packageState
	}
	type args struct {
		path string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		expect bool
	}{
		{
			name: "AddImport",
			fields: fields{
				packages:       map[string]*packageState{"foo": {importsAdded: map[string]bool{}}},
				currentPackage: "foo",
			},
			args:   args{path: "bar"},
			expect: true,
		},
		{
			name: "AddImport_nil_check",
			fields: fields{
				packages: map[string]*packageState{"foo": {importsAdded: map[string]bool{}}},
			},
			args:   args{path: "bar"},
			expect: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				userAppPath:       tt.fields.userAppPath,
				diffFile:          tt.fields.diffFile,
				appName:           tt.fields.appName,
				agentVariableName: tt.fields.agentVariableName,
				currentPackage:    tt.fields.currentPackage,
				packages:          tt.fields.packages,
			}

			defer panicRecovery(t)
			m.addImport(tt.args.path)

			if m.packages["foo"].importsAdded["bar"] != true && tt.expect {
				t.Errorf("AddImport failed to add import bar to package foo, got: %+v", m.packages["foo"].importsAdded)
			}
			if tt.expect == false && len(m.packages["foo"].importsAdded) != 0 {
				t.Errorf("AddImport added import bar to package foo, got: %+v", m.packages["foo"].importsAdded)
			}

		})
	}
}

func Test_GetImports(t *testing.T) {
	type fields struct {
		userAppPath       string
		diffFile          string
		appName           string
		agentVariableName string
		currentPackage    string
		packages          map[string]*packageState
	}

	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "GetImports_one_import",
			fields: fields{
				packages:       map[string]*packageState{"foo": {importsAdded: map[string]bool{"bar": true}}},
				currentPackage: "foo",
			},
			want: []string{"bar"},
		},
		{
			name: "GetImports_empty",
			fields: fields{
				packages:       map[string]*packageState{"foo": {importsAdded: map[string]bool{}}},
				currentPackage: "foo",
			},
			want: []string{},
		},
		{
			name: "GetImports_nil_check",
			fields: fields{
				packages: map[string]*packageState{"foo": {importsAdded: map[string]bool{}}},
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				userAppPath:       tt.fields.userAppPath,
				diffFile:          tt.fields.diffFile,
				appName:           tt.fields.appName,
				agentVariableName: tt.fields.agentVariableName,
				currentPackage:    tt.fields.currentPackage,
				packages:          tt.fields.packages,
			}
			if got := m.getImports(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InstrumentationManager.GetImports() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_CreateFunctionDeclaration(t *testing.T) {
	type fields struct {
		userAppPath       string
		diffFile          string
		appName           string
		agentVariableName string
		currentPackage    string
		packages          map[string]*packageState
	}
	type args struct {
		decl *dst.FuncDecl
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		expect bool
	}{
		{
			name: "CreateFunctionDeclaration",
			fields: fields{
				packages:       map[string]*packageState{"foo": {importsAdded: map[string]bool{}, tracedFuncs: map[string]*tracedFunctionDecl{}}},
				currentPackage: "foo",
			},
			args:   args{decl: &dst.FuncDecl{Name: &dst.Ident{Name: "bar"}}},
			expect: true,
		},
		{
			name: "CreateFunctionDeclaration_nil_check",
			fields: fields{
				packages: map[string]*packageState{"foo": {importsAdded: map[string]bool{}, tracedFuncs: map[string]*tracedFunctionDecl{}}},
			},
			args:   args{decl: &dst.FuncDecl{Name: &dst.Ident{Name: "bar"}}},
			expect: false,
		},
		{
			name: "CreateFunctionDeclaration_already_exists",
			fields: fields{
				packages:       map[string]*packageState{"foo": {importsAdded: map[string]bool{}, tracedFuncs: map[string]*tracedFunctionDecl{"bar": {}}}},
				currentPackage: "foo",
			},
			args:   args{decl: &dst.FuncDecl{Name: &dst.Ident{Name: "bar"}}},
			expect: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				userAppPath:       tt.fields.userAppPath,
				diffFile:          tt.fields.diffFile,
				appName:           tt.fields.appName,
				agentVariableName: tt.fields.agentVariableName,
				currentPackage:    tt.fields.currentPackage,
				packages:          tt.fields.packages,
			}
			defer panicRecovery(t)
			m.createFunctionDeclaration(tt.args.decl)

			if tt.expect {
				if m.packages["foo"].tracedFuncs["bar"] == nil {
					t.Errorf("CreateFunctionDeclaration failed to add new function bar to package foo, got: %+v", m.packages["foo"].tracedFuncs)
				}
				if len(m.packages["foo"].tracedFuncs) != 1 {
					t.Errorf("CreateFunctionDeclaration must not create a new entry if one already exists with that function name: %+v", m.packages["foo"].tracedFuncs)
				}
			}
			if !tt.expect {
				_, ok := m.packages["foo"].tracedFuncs["bar"]
				if ok {
					t.Errorf("CreateFunctionDeclaration added function bar to package foo when it should not have: %+v", m.packages["foo"].tracedFuncs)
				}
			}
		})
	}
}

func Test_UpdateFunctionDeclaration(t *testing.T) {
	type fields struct {
		userAppPath       string
		diffFile          string
		appName           string
		agentVariableName string
		currentPackage    string
		packages          map[string]*packageState
	}
	type args struct {
		decl *dst.FuncDecl
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		updates bool
	}{
		{
			name: "UpdateFunctionDeclaration",
			fields: fields{
				packages:       map[string]*packageState{"foo": {importsAdded: map[string]bool{}, tracedFuncs: map[string]*tracedFunctionDecl{"bar": {}}}},
				currentPackage: "foo",
			},
			args:    args{decl: &dst.FuncDecl{Name: &dst.Ident{Name: "bar"}}},
			updates: true,
		},
		{
			name: "UpdateFunctionDeclaration_nil_check",
			fields: fields{
				packages: map[string]*packageState{"foo": {importsAdded: map[string]bool{}, tracedFuncs: map[string]*tracedFunctionDecl{"bar": {}}}},
			},
			args:    args{decl: &dst.FuncDecl{Name: &dst.Ident{Name: "bar"}}},
			updates: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				userAppPath:       tt.fields.userAppPath,
				diffFile:          tt.fields.diffFile,
				appName:           tt.fields.appName,
				agentVariableName: tt.fields.agentVariableName,
				currentPackage:    tt.fields.currentPackage,
				packages:          tt.fields.packages,
			}

			defer panicRecovery(t)
			m.updateFunctionDeclaration(tt.args.decl)

			if tt.updates && reflect.DeepEqual(m.packages["foo"].tracedFuncs["bar"].body, tt.args.decl) == false {
				t.Errorf("UpdateFunctionDeclaration failed to update function bar to package foo, got: %+v", m.packages["foo"].tracedFuncs)
			}

			if !tt.updates && reflect.DeepEqual(m.packages["foo"].tracedFuncs["bar"].body, tt.args.decl) == true {
				t.Errorf("UpdateFunctionDeclaration updated function bar to package foo when it should not have: %+v", m.packages["foo"].tracedFuncs)
			}
		})
	}
}

// What if there are two instrumentable function invocations in a statement?
func Test_GetPackageFunctionInvocation(t *testing.T) {
	testFuncDecl := &dst.FuncDecl{}
	state := map[string]*packageState{"foo": {
		tracedFuncs: map[string]*tracedFunctionDecl{
			"bar": {body: testFuncDecl},
			"bax": {body: testFuncDecl},
		},
	}}

	type fields struct {
		userAppPath       string
		diffFile          string
		appName           string
		agentVariableName string
		currentPackage    string
		packages          map[string]*packageState
	}
	type args struct {
		node dst.Node
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*invocationInfo
	}{
		{
			name: "basic_passing_case",
			fields: fields{
				packages:       state,
				currentPackage: "foo",
			},
			args: args{node: &dst.CallExpr{Fun: &dst.Ident{Name: "bar", Path: "foo"}}},
			want: []*invocationInfo{{packageName: "foo", functionName: "bar", call: &dst.CallExpr{Fun: &dst.Ident{Name: "bar", Path: "foo"}}, decl: testFuncDecl}},
		},
		{
			name: "empty_path_passes",
			fields: fields{
				packages:       state,
				currentPackage: "foo",
			},
			args: args{node: &dst.CallExpr{Fun: &dst.Ident{Name: "bar"}}},
			want: []*invocationInfo{{packageName: "foo", functionName: "bar", call: &dst.CallExpr{Fun: &dst.Ident{Name: "bar"}}, decl: testFuncDecl}},
		},
		{
			name: "finds_call_in_complex_node",
			fields: fields{
				packages:       state,
				currentPackage: "foo",
			},
			args: args{node: &dst.ExprStmt{X: &dst.CallExpr{Fun: &dst.Ident{Name: "Sprintf", Path: "fmt"}, Args: []dst.Expr{&dst.CallExpr{Fun: &dst.Ident{Name: "bar"}}}}}},
			want: []*invocationInfo{{packageName: "foo", functionName: "bar", call: &dst.CallExpr{Fun: &dst.Ident{Name: "bar"}}, decl: testFuncDecl}},
		},
		{
			name: "ignore_functions_not_in_package",
			fields: fields{
				packages:       state,
				currentPackage: "foo",
			},
			args: args{node: &dst.CallExpr{Fun: &dst.Ident{Name: "bar", Path: "fmt"}}},
			want: []*invocationInfo{},
		},
		{
			name: "ignore_functions_not_declared_in_app",
			fields: fields{
				packages:       state,
				currentPackage: "foo",
			},
			args: args{node: &dst.CallExpr{Fun: &dst.Ident{Name: "baz", Path: "foo"}}},
			want: []*invocationInfo{},
		},
		{
			name: "do_not_traverse_block_statements",
			fields: fields{
				packages:       state,
				currentPackage: "foo",
			},
			args: args{node: &dst.BlockStmt{List: []dst.Stmt{&dst.ExprStmt{X: &dst.CallExpr{Fun: &dst.Ident{Name: "bar"}}}}}},
			want: []*invocationInfo{},
		},
		// Chained Methods: bax().bar() should return an invocation for bax and bar
		{
			name: "chain_of_methods",
			fields: fields{
				packages:       state,
				currentPackage: "foo",
			},
			args: args{
				node: &dst.ExprStmt{
					X: &dst.CallExpr{Fun: &dst.SelectorExpr{
						X:   &dst.CallExpr{Fun: &dst.Ident{Name: "bax"}},
						Sel: &dst.Ident{Name: "bar", Path: "foo"},
					}},
				},
			},
			want: []*invocationInfo{
				{
					packageName:  "foo",
					functionName: "bar",
					call: &dst.CallExpr{Fun: &dst.SelectorExpr{
						X:   &dst.CallExpr{Fun: &dst.Ident{Name: "bax"}},
						Sel: &dst.Ident{Name: "bar", Path: "foo"},
					}},
					decl: testFuncDecl},
				{packageName: "foo", functionName: "bax", call: &dst.CallExpr{Fun: &dst.Ident{Name: "bax"}}, decl: testFuncDecl},
			},
		},
		// Nested Methods: bax(bar()) should return an invocation for bax and bar
		{
			name: "nested_invocations",
			fields: fields{
				packages:       state,
				currentPackage: "foo",
			},
			args: args{
				node: &dst.ExprStmt{
					X: &dst.CallExpr{
						Fun: &dst.Ident{
							Name: "bax", Path: "foo", // This is the outer function call
						},
						Args: []dst.Expr{
							&dst.CallExpr{ // This is the inner function call
								Fun: &dst.Ident{Name: "bar", Path: "foo"}, // This should be resolved to bar
							},
						},
					},
				},
			},
			want: []*invocationInfo{
				{
					packageName:  "foo",
					functionName: "bax",
					call: &dst.CallExpr{
						Fun: &dst.Ident{
							Name: "bax", Path: "foo",
						},
						Args: []dst.Expr{
							&dst.CallExpr{
								Fun: &dst.Ident{Name: "bar", Path: "foo"},
							},
						},
					},
					decl: testFuncDecl},
				{packageName: "foo", functionName: "bar", call: &dst.CallExpr{Fun: &dst.Ident{Name: "bar", Path: "foo"}}, decl: testFuncDecl},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				userAppPath:       tt.fields.userAppPath,
				diffFile:          tt.fields.diffFile,
				appName:           tt.fields.appName,
				agentVariableName: tt.fields.agentVariableName,
				currentPackage:    tt.fields.currentPackage,
				packages:          tt.fields.packages,
			}
			defer panicRecovery(t)
			got := m.findInvocationInfo(tt.args.node, tracestate.FunctionBody(codegen.DefaultTransactionVariable))
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_ShouldInstrumentFunction(t *testing.T) {
	type fields struct {
		userAppPath       string
		diffFile          string
		appName           string
		agentVariableName string
		currentPackage    string
		packages          map[string]*packageState
	}
	type args struct {
		inv *invocationInfo
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "function_should_be_instrumented",
			fields: fields{
				packages:       map[string]*packageState{"foo": {tracedFuncs: map[string]*tracedFunctionDecl{"bar": {}}}},
				currentPackage: "foo",
			},
			args: args{inv: &invocationInfo{packageName: "foo", functionName: "bar"}},
			want: true,
		},
		{
			name: "nil_invocation",
			fields: fields{
				packages:       map[string]*packageState{"foo": {tracedFuncs: map[string]*tracedFunctionDecl{"bar": {}}}},
				currentPackage: "foo",
			},
			args: args{inv: nil},
			want: false,
		},
		{
			name: "already_instrumented",
			fields: fields{
				packages:       map[string]*packageState{"foo": {tracedFuncs: map[string]*tracedFunctionDecl{"bar": {traced: true}}}},
				currentPackage: "foo",
			},
			args: args{inv: &invocationInfo{packageName: "foo", functionName: "bar"}},
			want: false,
		},
		{
			name: "package_not_found",
			fields: fields{
				packages:       map[string]*packageState{},
				currentPackage: "foo",
			},
			args: args{inv: &invocationInfo{packageName: "foo", functionName: "bar"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				userAppPath:       tt.fields.userAppPath,
				diffFile:          tt.fields.diffFile,
				appName:           tt.fields.appName,
				agentVariableName: tt.fields.agentVariableName,
				currentPackage:    tt.fields.currentPackage,
				packages:          tt.fields.packages,
			}
			defer panicRecovery(t)
			got := m.shouldInstrumentFunction(tt.args.inv)
			if got != tt.want {
				t.Errorf("InstrumentationManager.ShouldInstrumentFunction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetInvocationInfoFromCall(t *testing.T) {
	testFuncDecl := &dst.FuncDecl{}
	state := map[string]*packageState{"foo": {
		tracedFuncs: map[string]*tracedFunctionDecl{"bar": {body: testFuncDecl}},
	}}
	type fields struct {
		userAppPath       string
		diffFile          string
		appName           string
		agentVariableName string
		currentPackage    string
		packages          map[string]*packageState
	}
	type args struct {
		call    *dst.CallExpr
		forTest string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *invocationInfo
	}{
		{
			name: "basic_passing_case",
			fields: fields{
				packages:       state,
				currentPackage: "foo",
			},
			args: args{call: &dst.CallExpr{Fun: &dst.Ident{Name: "bar", Path: "foo"}}, forTest: ""},
			want: &invocationInfo{packageName: "foo", functionName: "bar", call: &dst.CallExpr{Fun: &dst.Ident{Name: "bar", Path: "foo"}}, decl: testFuncDecl},
		},
		{
			name: "ignore_functions_not_in_package",
			fields: fields{
				packages:       state,
				currentPackage: "foo",
			},
			args: args{call: &dst.CallExpr{Fun: &dst.Ident{Name: "bar", Path: "fmt"}}, forTest: ""},
			want: nil,
		},
		{
			name: "forTest_path_passes",
			fields: fields{
				packages:       state,
				currentPackage: "foo",
			},
			args: args{call: &dst.CallExpr{Fun: &dst.Ident{Name: "bar"}}, forTest: "foo"},
			want: &invocationInfo{packageName: "foo", functionName: "bar", call: &dst.CallExpr{Fun: &dst.Ident{Name: "bar"}}, decl: testFuncDecl},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				userAppPath:       tt.fields.userAppPath,
				diffFile:          tt.fields.diffFile,
				appName:           tt.fields.appName,
				agentVariableName: tt.fields.agentVariableName,
				currentPackage:    tt.fields.currentPackage,
				packages:          tt.fields.packages,
			}
			defer panicRecovery(t)
			got := m.getInvocationInfoFromCall(tt.args.call, tt.args.forTest)
			assert.Equal(t, tt.want, got)
		})
	}
}
