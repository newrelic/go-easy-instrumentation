package parser

import (
	"reflect"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/parser/facts"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/packages"
)

func TestAddImport(t *testing.T) {
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

			defer PanicRecovery(t)
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

func TestGetImports(t *testing.T) {
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

func TestCreateFunctionDeclaration(t *testing.T) {
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
			defer PanicRecovery(t)
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

func TestUpdateFunctionDeclaration(t *testing.T) {
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

			defer PanicRecovery(t)
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
func TestGetPackageFunctionInvocation(t *testing.T) {
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
			defer PanicRecovery(t)
			got := m.findInvocationInfo(tt.args.node, tracestate.FunctionBody(codegen.DefaultTransactionVariable))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldInstrumentFunction(t *testing.T) {
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
			defer PanicRecovery(t)
			got := m.shouldInstrumentFunction(tt.args.inv)
			if got != tt.want {
				t.Errorf("InstrumentationManager.ShouldInstrumentFunction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetInvocationInfoFromCall(t *testing.T) {
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
			defer PanicRecovery(t)
			got := m.getInvocationInfoFromCall(tt.args.call, tt.args.forTest)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewInstrumentationManager(t *testing.T) {
	type args struct {
		pkgs              []*decorator.Package
		appName           string
		agentVariableName string
		diffFile          string
		userAppPath       string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "creates_manager_with_single_package",
			args: args{
				pkgs: []*decorator.Package{
					{Package: &packages.Package{ID: "test/pkg"}},
				},
				appName:           "TestApp",
				agentVariableName: "agent",
				diffFile:          "test.diff",
				userAppPath:       "/test/path",
			},
		},
		{
			name: "creates_manager_with_multiple_packages",
			args: args{
				pkgs: []*decorator.Package{
					{Package: &packages.Package{ID: "test/pkg1"}},
					{Package: &packages.Package{ID: "test/pkg2"}},
					{Package: &packages.Package{ID: "test/pkg3"}},
				},
				appName:           "TestApp",
				agentVariableName: "agent",
				diffFile:          "test.diff",
				userAppPath:       "/test/path",
			},
		},
		{
			name: "creates_manager_with_no_packages",
			args: args{
				pkgs:              []*decorator.Package{},
				appName:           "TestApp",
				agentVariableName: "agent",
				diffFile:          "test.diff",
				userAppPath:       "/test/path",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewInstrumentationManager(tt.args.pkgs, tt.args.appName, tt.args.agentVariableName, tt.args.diffFile, tt.args.userAppPath)
			assert.NotNil(t, got)
			assert.Equal(t, tt.args.appName, got.appName)
			assert.Equal(t, tt.args.agentVariableName, got.agentVariableName)
			assert.Equal(t, tt.args.diffFile, got.diffFile)
			assert.Equal(t, tt.args.userAppPath, got.userAppPath)
			assert.Equal(t, len(tt.args.pkgs), len(got.packages))
			assert.NotNil(t, got.facts)
			assert.NotNil(t, got.errorCache)
			assert.NotNil(t, got.transactionCache)
			for _, pkg := range tt.args.pkgs {
				state, ok := got.packages[pkg.ID]
				assert.True(t, ok)
				assert.NotNil(t, state.tracedFuncs)
				assert.NotNil(t, state.importsAdded)
			}
		})
	}
}

func TestSetPackage(t *testing.T) {
	type fields struct {
		currentPackage string
	}
	type args struct {
		pkgName string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name:   "set_package_name",
			fields: fields{currentPackage: ""},
			args:   args{pkgName: "foo"},
		},
		{
			name:   "change_package_name",
			fields: fields{currentPackage: "bar"},
			args:   args{pkgName: "foo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				currentPackage: tt.fields.currentPackage,
			}
			m.setPackage(tt.args.pkgName)
			assert.Equal(t, tt.args.pkgName, m.currentPackage)
		})
	}
}

func TestGetPackageName(t *testing.T) {
	type fields struct {
		currentPackage string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "returns_current_package",
			fields: fields{currentPackage: "foo"},
			want:   "foo",
		},
		{
			name:   "returns_empty_string",
			fields: fields{currentPackage: ""},
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				currentPackage: tt.fields.currentPackage,
			}
			got := m.getPackageName()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetDecoratorPackage(t *testing.T) {
	testPkg := &decorator.Package{Package: &packages.Package{ID: "test"}}
	type fields struct {
		currentPackage string
		packages       map[string]*packageState
	}
	tests := []struct {
		name   string
		fields fields
		want   *decorator.Package
	}{
		{
			name: "returns_decorator_package",
			fields: fields{
				currentPackage: "foo",
				packages:       map[string]*packageState{"foo": {pkg: testPkg}},
			},
			want: testPkg,
		},
		{
			name: "returns_nil_when_package_not_found",
			fields: fields{
				currentPackage: "foo",
				packages:       map[string]*packageState{},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				currentPackage: tt.fields.currentPackage,
				packages:       tt.fields.packages,
			}
			got := m.getDecoratorPackage()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsDefinedInPackage(t *testing.T) {
	type fields struct {
		packages map[string]*packageState
	}
	type args struct {
		functionName string
		packageName  string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "function_exists_in_package",
			fields: fields{
				packages: map[string]*packageState{
					"foo": {tracedFuncs: map[string]*tracedFunctionDecl{"bar": {}}},
				},
			},
			args: args{functionName: "bar", packageName: "foo"},
			want: true,
		},
		{
			name: "function_does_not_exist_in_package",
			fields: fields{
				packages: map[string]*packageState{
					"foo": {tracedFuncs: map[string]*tracedFunctionDecl{}},
				},
			},
			args: args{functionName: "bar", packageName: "foo"},
			want: false,
		},
		{
			name: "package_does_not_exist",
			fields: fields{
				packages: map[string]*packageState{},
			},
			args: args{functionName: "bar", packageName: "foo"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				packages: tt.fields.packages,
			}
			got := m.isDefinedInPackage(tt.args.functionName, tt.args.packageName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolvePath(t *testing.T) {
	type args struct {
		identPath      string
		currentPackage string
		forTest        string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "returns_identPath_when_set",
			args: args{
				identPath:      "test/path",
				currentPackage: "current",
				forTest:        "for/test",
			},
			want: "test/path",
		},
		{
			name: "returns_forTest_when_identPath_empty",
			args: args{
				identPath:      "",
				currentPackage: "current",
				forTest:        "for/test",
			},
			want: "for/test",
		},
		{
			name: "returns_currentPackage_when_both_empty",
			args: args{
				identPath:      "",
				currentPackage: "current",
				forTest:        "",
			},
			want: "current",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePath(tt.args.identPath, tt.args.currentPackage, tt.args.forTest)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetSortedPackages(t *testing.T) {
	type fields struct {
		packages map[string]*packageState
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "sorts_packages_alphabetically",
			fields: fields{
				packages: map[string]*packageState{
					"zebra": {},
					"alpha": {},
					"beta":  {},
				},
			},
			want: []string{"alpha", "beta", "zebra"},
		},
		{
			name: "handles_single_package",
			fields: fields{
				packages: map[string]*packageState{
					"foo": {},
				},
			},
			want: []string{"foo"},
		},
		{
			name: "handles_empty_packages",
			fields: fields{
				packages: map[string]*packageState{},
			},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &InstrumentationManager{
				packages: tt.fields.packages,
			}
			got := m.getSortedPackages()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadTracingFunctions(t *testing.T) {
	mockStateless := func(m *InstrumentationManager, c *dstutil.Cursor) {}
	mockStateful := func(m *InstrumentationManager, stmt dst.Stmt, c *dstutil.Cursor, tracing *tracestate.State) bool {
		return false
	}
	mockDependency := func(pkg *decorator.Package, n dst.Node) (facts.Entry, bool) {
		return facts.Entry{}, false
	}
	mockPreInstrumentation := func(m *InstrumentationManager, c *dstutil.Cursor) {}

	tests := []struct {
		name     string
		testFunc func(*InstrumentationManager)
		verify   func(*testing.T, *InstrumentationManager)
	}{
		{
			name: "loadStatelessTracingFunctions_adds_functions",
			testFunc: func(m *InstrumentationManager) {
				m.LoadStatelessTracingFunctions(mockStateless, mockStateless)
			},
			verify: func(t *testing.T, m *InstrumentationManager) {
				assert.Equal(t, 2, len(m.tracingFunctions.stateless))
			},
		},
		{
			name: "loadStatefulTracingFunctions_adds_functions",
			testFunc: func(m *InstrumentationManager) {
				m.LoadStatefulTracingFunctions(mockStateful, mockStateful, mockStateful)
			},
			verify: func(t *testing.T, m *InstrumentationManager) {
				assert.Equal(t, 3, len(m.tracingFunctions.stateful))
			},
		},
		{
			name: "loadDependencyScans_adds_scans",
			testFunc: func(m *InstrumentationManager) {
				m.LoadDependencyScans(mockDependency)
			},
			verify: func(t *testing.T, m *InstrumentationManager) {
				assert.Equal(t, 1, len(m.tracingFunctions.dependency))
			},
		},
		{
			name: "loadPreInstrumentationTracingFunctions_adds_functions",
			testFunc: func(m *InstrumentationManager) {
				m.LoadPreInstrumentationTracingFunctions(mockPreInstrumentation, mockPreInstrumentation)
			},
			verify: func(t *testing.T, m *InstrumentationManager) {
				assert.Equal(t, 2, len(m.tracingFunctions.preinstrumentation))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewInstrumentationManager([]*decorator.Package{}, "app", "agent", "diff.txt", "/path")
			tt.testFunc(m)
			tt.verify(t, m)
		})
	}
}

func TestErrorNoMain(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "returns_error_with_path",
			args:    args{path: "/test/path"},
			wantErr: true,
		},
		{
			name:    "returns_error_with_empty_path",
			args:    args{path: ""},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errorNoMain(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("errorNoMain() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Contains(t, err.Error(), "cannot find a main method")
		})
	}
}

func TestAddImport_EmptyPath(t *testing.T) {
	m := &InstrumentationManager{
		packages:       map[string]*packageState{"foo": {importsAdded: map[string]bool{}}},
		currentPackage: "foo",
	}
	m.addImport("")
	assert.Equal(t, 0, len(m.packages["foo"].importsAdded))
}

// Test_DetectDependencyIntegrations is obsolete - integration registration moved to cmd/instrument.go
// Integration registration is now done via cmd/instrument.go's registerIntegrations() function
// which uses dependency injection to register all integration functions with the manager.
func TestDetectDependencyIntegrations(t *testing.T) {
	t.Skip("Integration registration moved to cmd/instrument.go - test no longer applicable")
}

func TestInstrumentPackages(t *testing.T) {
	type args struct {
		instrumentationFunctions []StatelessTracingFunction
	}
	tests := []struct {
		name    string
		manager *InstrumentationManager
		args    args
		wantErr bool
	}{
		{
			name: "nil_instrumentation_functions",
			manager: &InstrumentationManager{
				packages: map[string]*packageState{},
			},
			args:    args{instrumentationFunctions: nil},
			wantErr: true,
		},
		{
			name: "empty_instrumentation_functions",
			manager: &InstrumentationManager{
				packages: map[string]*packageState{
					"test": {
						pkg: &decorator.Package{
							Package: &packages.Package{ID: "test"},
							Syntax: []*dst.File{
								{
									Decls: []dst.Decl{
										&dst.FuncDecl{
											Name: &dst.Ident{Name: "test"},
										},
									},
								},
							},
						},
					},
				},
				currentPackage: "test",
			},
			args:    args{instrumentationFunctions: []StatelessTracingFunction{}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := instrumentPackages(tt.manager, tt.args.instrumentationFunctions...)
			if (err != nil) != tt.wantErr {
				t.Errorf("instrumentPackages() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestScanPackages(t *testing.T) {
	type args struct {
		instrumentationFunctions []PreInstrumentationTracingFunction
	}
	tests := []struct {
		name    string
		manager *InstrumentationManager
		args    args
		wantErr bool
	}{
		{
			name: "nil_instrumentation_functions",
			manager: &InstrumentationManager{
				packages: map[string]*packageState{},
			},
			args:    args{instrumentationFunctions: nil},
			wantErr: true,
		},
		{
			name: "empty_instrumentation_functions",
			manager: &InstrumentationManager{
				packages: map[string]*packageState{
					"test": {
						pkg: &decorator.Package{
							Package: &packages.Package{ID: "test"},
							Syntax: []*dst.File{
								{
									Decls: []dst.Decl{
										&dst.FuncDecl{
											Name: &dst.Ident{Name: "test"},
										},
									},
								},
							},
						},
					},
				},
				currentPackage: "test",
			},
			args:    args{instrumentationFunctions: []PreInstrumentationTracingFunction{}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scanPackages(tt.manager, tt.args.instrumentationFunctions...)
			if (err != nil) != tt.wantErr {
				t.Errorf("scanPackages() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTracePackageCalls(t *testing.T) {
	tests := []struct {
		name    string
		manager *InstrumentationManager
		wantErr bool
	}{
		{
			name: "errors_without_main_method",
			manager: &InstrumentationManager{
				packages: map[string]*packageState{},
				tracingFunctions: tracingFunctions{
					dependency: []FactDiscoveryFunction{},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manager.TracePackageCalls()
			if (err != nil) != tt.wantErr {
				t.Errorf("TracePackageCalls() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestScanApplication(t *testing.T) {
	tests := []struct {
		name    string
		manager *InstrumentationManager
		wantErr bool
	}{
		{
			name: "succeeds_with_empty_preinstrumentation_functions",
			manager: &InstrumentationManager{
				packages: map[string]*packageState{},
				tracingFunctions: tracingFunctions{
					preinstrumentation: []PreInstrumentationTracingFunction{},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manager.ScanApplication()
			if (err != nil) != tt.wantErr {
				t.Errorf("ScanApplication() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInstrumentApplication(t *testing.T) {
	tests := []struct {
		name    string
		manager *InstrumentationManager
		wantErr bool
	}{
		{
			name: "succeeds_with_empty_stateless_functions",
			manager: &InstrumentationManager{
				packages: map[string]*packageState{},
				tracingFunctions: tracingFunctions{
					stateless: []StatelessTracingFunction{},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manager.InstrumentApplication()
			if (err != nil) != tt.wantErr {
				t.Errorf("InstrumentApplication() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
