package main

import (
	"bytes"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gopackages"
	"github.com/dave/dst/dstutil"
	godiffpatch "github.com/sourcegraph/go-diff-patch"
)

const (
	defaultTxnName = "nrTxn"
)

// tracedFunction contains relevant information about a function within the current package, and
// its tracing status.
//
// Please access this object's data through methods rather than directly manipulating it.
type tracedFunction struct {
	traced      bool
	requiresTxn bool
	body        *dst.FuncDecl
}

// InstrumentationManager maintains state relevant to tracing across all files, packages and functions.
type InstrumentationManager struct {
	userAppPath       string // path to the user's application as provided by the user
	diffFile          string
	appName           string
	agentVariableName string
	currentPackage    string
	packages          map[string]*PackageState // stores stateful information on packages by ID
}

// PackageManager contains state relevant to tracing within a single package.
type PackageState struct {
	pkg          *decorator.Package         // the package being instrumented
	tracedFuncs  map[string]*tracedFunction // maintains state of tracing for functions within the package
	importsAdded map[string]bool            // tracks imports added to the package
}

const (
	newrelicAgentImport string = "github.com/newrelic/go-agent/v3/newrelic"
)

// NewInstrumentationManager initializes an InstrumentationManager cache for a given package.
func NewInstrumentationManager(pkgs []*decorator.Package, appName, agentVariableName, diffFile, userAppPath string) *InstrumentationManager {
	manager := &InstrumentationManager{
		userAppPath:       userAppPath,
		diffFile:          diffFile,
		appName:           appName,
		agentVariableName: agentVariableName,
		packages:          map[string]*PackageState{},
	}

	for _, pkg := range pkgs {
		manager.packages[pkg.ID] = &PackageState{
			pkg:          pkg,
			tracedFuncs:  map[string]*tracedFunction{},
			importsAdded: map[string]bool{},
		}
	}

	return manager
}

func (m *InstrumentationManager) SetPackage(pkgName string) {
	m.currentPackage = pkgName
}

func (m *InstrumentationManager) AddImport(path string) {
	state, ok := m.packages[m.currentPackage]
	if ok {
		state.importsAdded[path] = true
	}
}

func (m *InstrumentationManager) GetImports(fileName string) []string {
	i := 0
	state, ok := m.packages[m.currentPackage]
	if !ok {
		return []string{}
	}

	importsAdded := state.importsAdded
	ret := make([]string, len(importsAdded))
	for k := range importsAdded {
		ret[i] = string(k)
		i++
	}
	return ret
}

// Returns Decorator Package for the current package being instrumented
func (m *InstrumentationManager) GetDecoratorPackage() *decorator.Package {
	state, ok := m.packages[m.currentPackage]
	if !ok {
		return nil
	}

	return state.pkg
}

// Returns the string name of the current package
func (m *InstrumentationManager) GetPackageName() string {
	return m.currentPackage
}

// CreateFunctionDeclaration creates a tracking object for a function declaration that can be used
// to find tracing locations. This is for initializing and set up only.
func (m *InstrumentationManager) CreateFunctionDeclaration(decl *dst.FuncDecl) {
	state, ok := m.packages[m.currentPackage]
	if !ok {
		return
	}

	_, ok = state.tracedFuncs[decl.Name.Name]
	if !ok {
		state.tracedFuncs[decl.Name.Name] = &tracedFunction{
			body: decl,
		}
	}
}

// UpdateFunctionDeclaration replaces the declaration stored for the given function name, and marks it as traced.
func (m *InstrumentationManager) UpdateFunctionDeclaration(decl *dst.FuncDecl) {
	state, ok := m.packages[m.currentPackage]
	if ok {
		t, ok := state.tracedFuncs[decl.Name.Name]
		if ok {
			t.body = decl
			t.traced = true
		}
	}
}

type invocationInfo struct {
	functionName string
	packageName  string
	call         *dst.CallExpr
}

// GetPackageFunctionInvocation returns the name of the function being invoked, and the expression containing the call
// where that invocation occurs if a function is declared in this package.
func (m *InstrumentationManager) GetPackageFunctionInvocation(node dst.Node) *invocationInfo {
	var invInfo *invocationInfo

	dst.Inspect(node, func(n dst.Node) bool {
		switch v := n.(type) {
		case *dst.BlockStmt:
			return false
		case *dst.CallExpr:
			call := v
			functionCallIdent, ok := call.Fun.(*dst.Ident)
			if ok {
				path := functionCallIdent.Path
				if path == "" {
					path = m.GetPackageName()
				}
				_, ok := m.packages[path]
				if ok {
					invInfo = &invocationInfo{
						functionName: functionCallIdent.Name,
						packageName:  path,
						call:         call,
					}
					return false
				}
			}
			return true
		}
		return true
	})

	return invInfo
}

// AddTxnArgumentToFuncDecl adds a transaction argument to the declaration of a function. This marks that function as needing a transaction,
// and can be looked up by name to know that the last argument is a transaction.
func (m *InstrumentationManager) AddTxnArgumentToFunctionDecl(decl *dst.FuncDecl, txnVarName string) {
	if decl == nil {
		return
	}

	if decl.Type.Params == nil {
		decl.Type.Params = &dst.FieldList{
			List: []*dst.Field{{
				Names: []*dst.Ident{dst.NewIdent(txnVarName)},
				Type: &dst.StarExpr{
					X: &dst.SelectorExpr{
						X:   dst.NewIdent("newrelic"),
						Sel: dst.NewIdent("Transaction"),
					},
				},
			}},
		}
	} else {
		decl.Type.Params.List = append(decl.Type.Params.List, &dst.Field{
			Names: []*dst.Ident{dst.NewIdent(txnVarName)},
			Type: &dst.StarExpr{
				X: &dst.SelectorExpr{
					X:   dst.NewIdent("newrelic"),
					Sel: dst.NewIdent("Transaction"),
				},
			},
		})
	}
	state, ok := m.packages[m.currentPackage]
	if ok {
		fn, ok := state.tracedFuncs[decl.Name.Name]
		if ok {
			fn.requiresTxn = true
		}
	}
}

// IsTracingComplete returns true if a function has all the tracing it needs added to it.
func (m *InstrumentationManager) ShouldInstrumentFunction(inv *invocationInfo) bool {
	if inv == nil {
		return false
	}

	state, ok := m.packages[inv.packageName]
	if ok {
		v, ok := state.tracedFuncs[inv.functionName]
		if ok {
			return !v.traced
		}
	}

	return false
}

// conatinsTransactionArgument returns true if a function call contains a transaction argument.
// This function works for async functions as well.
func containsTransactionArgument(call *dst.CallExpr, txnName string) bool {
	if call == nil || call.Args == nil {
		return false
	}

	for _, arg := range call.Args {
		switch v := arg.(type) {
		case *dst.Ident:
			if v.Name == txnName {
				return true
			}
		case *dst.CallExpr:
			sel, ok := v.Fun.(*dst.SelectorExpr)
			if ok {
				if sel.Sel.Name == "NewGoroutine" {
					ident, ok := sel.X.(*dst.Ident)
					if ok && ident.Name == txnName {
						return true
					}
				}
			}
		}
	}
	return false
}

// RequiresTransactionArgument returns true if a modified function needs a transaction as an argument.
// This can be used to check if transactions should be passed by callers.
func (m *InstrumentationManager) RequiresTransactionArgument(inv *invocationInfo, txnVariableName string) bool {
	if inv == nil {
		return false
	}

	state, ok := m.packages[m.currentPackage]
	if ok {
		v, ok := state.tracedFuncs[inv.functionName]
		if ok && !containsTransactionArgument(inv.call, txnVariableName) {
			return v.requiresTxn
		}
	}
	return false
}

// GetDeclaration returns a pointer to the location in the DST tree where a function is declared and defined.
func (m *InstrumentationManager) GetDeclaration(functionName string) *dst.FuncDecl {
	if m.packages[m.currentPackage] != nil && m.packages[m.currentPackage].tracedFuncs != nil {
		v, ok := m.packages[m.currentPackage].tracedFuncs[functionName]
		if ok {
			return v.body
		}
	}
	return nil
}

// WriteDiff writes out the changes made to a file to the diff file for this package.
func (m *InstrumentationManager) WriteDiff() {
	for _, state := range m.packages {
		r := decorator.NewRestorerWithImports(state.pkg.Dir, gopackages.New(state.pkg.Dir))

		for _, file := range state.pkg.Syntax {
			path := state.pkg.Decorator.Filenames[file]
			originalFile, err := os.ReadFile(path)
			if err != nil {
				log.Fatal(err)
			}

			// what this file will be named in the diff file
			var diffFileName string

			absAppPath, err := filepath.Abs(m.userAppPath)
			if err != nil {
				log.Fatal(err)
			}
			diffFileName, err = filepath.Rel(absAppPath, path)
			if err != nil {
				log.Fatal(err)
			}

			modifiedFile := bytes.NewBuffer([]byte{})
			if err := r.Fprint(modifiedFile, file); err != nil {
				log.Fatal(err)
			}

			f, err := os.OpenFile(m.diffFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Println(err)
			}
			defer f.Close()
			patch := godiffpatch.GeneratePatch(diffFileName, string(originalFile), modifiedFile.String())
			if _, err := f.WriteString(patch); err != nil {
				log.Println(err)
			}
		}
	}
	log.Printf("changes written to %s", m.diffFile)
}

func (m *InstrumentationManager) AddRequiredModules() {
	for _, state := range m.packages {
		wd, _ := os.Getwd()
		err := os.Chdir(state.pkg.Dir)
		if err != nil {
			log.Fatal(err)
		}

		for module := range state.importsAdded {
			err := exec.Command("go", "get", module).Run()
			if err != nil {
				log.Fatalf("Error Getting GO module %s: %v", module, err)
			}
		}

		err = os.Chdir(wd)
		if err != nil {
			log.Fatal(err)
		}
	}
}

// InstrumentPackages applies instrumentation to all functions in the package.
func (m *InstrumentationManager) InstrumentPackages(instrumentationFunctions ...StatelessTracingFunction) error {
	// Create a call graph of all calls made to functions in this package
	err := tracePackageFunctionCalls(m)
	if err != nil {
		return err
	}

	instrumentPackages(m, instrumentationFunctions...)

	return nil
}

// traceFunctionCalls discovers and sets up tracing for all function calls in the current package
func tracePackageFunctionCalls(manager *InstrumentationManager) error {
	hasMain := false
	for packageName, pkg := range manager.packages {
		manager.SetPackage(packageName)
		for _, file := range pkg.pkg.Syntax {
			for _, decl := range file.Decls {
				if fn, isFn := decl.(*dst.FuncDecl); isFn {
					manager.CreateFunctionDeclaration(fn)
					if fn.Name.Name == "main" {
						hasMain = true
					}
				}
			}
		}
	}

	if !hasMain {
		return errors.New("cannot find a main method for this application")
	}
	return nil
}

// apply instrumentation to the package
func instrumentPackages(manager *InstrumentationManager, instrumentationFunctions ...StatelessTracingFunction) {
	for pkgName, pkgState := range manager.packages {
		manager.SetPackage(pkgName)
		for _, file := range pkgState.pkg.Syntax {
			for _, decl := range file.Decls {
				if fn, isFn := decl.(*dst.FuncDecl); isFn {
					dstutil.Apply(fn, nil, func(c *dstutil.Cursor) bool {
						n := c.Node()
						for _, instFunc := range instrumentationFunctions {
							instFunc(n, manager, c)
						}
						return true
					})
				}
			}
		}
	}
}
