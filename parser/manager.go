package parser

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gopackages"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/codegen"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser/errorcache"
	"github.com/newrelic/go-easy-instrumentation/parser/facts"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
	"github.com/newrelic/go-easy-instrumentation/parser/transactioncache"
	godiffpatch "github.com/sourcegraph/go-diff-patch"
)

// tracedFunction contains relevant information about a function within the current package, and
// its tracing status.
//
// Please access this object's data through methods rather than directly manipulating it.
type tracedFunctionDecl struct {
	traced bool
	body   *dst.FuncDecl
}

type tracingFunctions struct {
	stateless          []StatelessTracingFunction
	stateful           []StatefulTracingFunction
	dependency         []FactDiscoveryFunction
	preinstrumentation []PreInstrumentationTracingFunction
}

// InstrumentationManager maintains state relevant to tracing across all files, packages and functions.
type InstrumentationManager struct {
	appName           string
	agentVariableName string
	userAppPath       string // path to the user's application as provided by the user
	diffFile          string
	currentPackage    string
	tracingFunctions  tracingFunctions
	facts             facts.Keeper
	packages          map[string]*packageState          // stores stateful information on packages by ID
	errorCache        errorcache.ErrorCache             // stores error handling status for functions
	transactionCache  transactioncache.TransactionCache // stores transaction status for functions
	setupFunc         *dst.FuncDecl
}

// PackageManager contains state relevant to tracing within a single package.
type packageState struct {
	pkg          *decorator.Package             // the package being instrumented
	tracedFuncs  map[string]*tracedFunctionDecl // maintains state of tracing for functions within the package
	importsAdded map[string]bool                // tracks imports added to the package
}

// NewInstrumentationManager initializes an InstrumentationManager cache for a given package.
func NewInstrumentationManager(pkgs []*decorator.Package, appName, agentVariableName, diffFile, userAppPath string) *InstrumentationManager {
	manager := &InstrumentationManager{
		userAppPath:       userAppPath,
		diffFile:          diffFile,
		appName:           appName,
		agentVariableName: agentVariableName,
		packages:          map[string]*packageState{},
		facts:             facts.NewKeeper(),
		errorCache:        errorcache.ErrorCache{},
		transactionCache:  *transactioncache.NewTransactionCache(),
		tracingFunctions: tracingFunctions{
			stateless:          []StatelessTracingFunction{},
			stateful:           []StatefulTracingFunction{},
			dependency:         []FactDiscoveryFunction{},
			preinstrumentation: []PreInstrumentationTracingFunction{},
		},
	}

	for _, pkg := range pkgs {
		manager.packages[pkg.ID] = &packageState{
			pkg:          pkg,
			tracedFuncs:  map[string]*tracedFunctionDecl{},
			importsAdded: map[string]bool{},
		}
	}

	return manager
}

// DetectDependencyIntegrations
func (m *InstrumentationManager) DetectDependencyIntegrations() error {
	m.loadPreInstrumentationTracingFunctions(DetectTransactions, DetectErrors, DetectWrappedRoutes)
	m.loadStatelessTracingFunctions(InstrumentMain, InstrumentHandleFunction, InstrumentHttpClient, CannotInstrumentHttpMethod, InstrumentGrpcDial, InstrumentGinFunction, InstrumentGrpcServerMethod, InstrumentSlogHandler)
	m.loadStatefulTracingFunctions(ExternalHttpCall, WrapNestedHandleFunction, InstrumentGrpcServer, InstrumentGinMiddleware, InstrumentChiMiddleware, InstrumentChiRouterLiteral)
	m.loadDependencyScans(FindGrpcServerObject)
	return nil
}

func (m *InstrumentationManager) loadPreInstrumentationTracingFunctions(functions ...PreInstrumentationTracingFunction) {
	m.tracingFunctions.preinstrumentation = append(m.tracingFunctions.preinstrumentation, functions...)
}

func (m *InstrumentationManager) loadStatefulTracingFunctions(functions ...StatefulTracingFunction) {
	m.tracingFunctions.stateful = append(m.tracingFunctions.stateful, functions...)
}

func (m *InstrumentationManager) loadStatelessTracingFunctions(functions ...StatelessTracingFunction) {
	m.tracingFunctions.stateless = append(m.tracingFunctions.stateless, functions...)
}

func (m *InstrumentationManager) loadDependencyScans(scans ...FactDiscoveryFunction) {
	m.tracingFunctions.dependency = append(m.tracingFunctions.dependency, scans...)
}

func (m *InstrumentationManager) CreateDiffFile() error {
	f, err := os.Create(m.diffFile)
	f.Close()
	return err
}

func (m *InstrumentationManager) DebugTransactionCache() {
	m.transactionCache.Print()
}
func (m *InstrumentationManager) setPackage(pkgName string) {
	m.currentPackage = pkgName
}

func (m *InstrumentationManager) addImport(path string) {
	if path == "" {
		return
	}
	state, ok := m.packages[m.currentPackage]
	if ok {
		state.importsAdded[path] = true
	}
}

func (m *InstrumentationManager) getImports() []string {
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
func (m *InstrumentationManager) getDecoratorPackage() *decorator.Package {
	state, ok := m.packages[m.currentPackage]
	if !ok {
		return nil
	}

	return state.pkg
}

// Returns the string name of the current package
func (m *InstrumentationManager) getPackageName() string {
	return m.currentPackage
}

// createFunctionDeclaration creates a tracking object for a function declaration that can be used
// to find tracing locations. This is for initializing and set up only.
func (m *InstrumentationManager) createFunctionDeclaration(decl *dst.FuncDecl) {
	state, ok := m.packages[m.currentPackage]
	if !ok {
		return
	}

	_, ok = state.tracedFuncs[decl.Name.Name]
	if !ok {
		state.tracedFuncs[decl.Name.Name] = &tracedFunctionDecl{
			body: decl,
		}
	}
}

// UpdateFunctionDeclaration replaces the declaration stored for the given function name, and marks it as traced.
func (m *InstrumentationManager) updateFunctionDeclaration(decl *dst.FuncDecl) {
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
	decl         *dst.FuncDecl
}

func resolvePath(identPath, currentPackage, forTest string) string {
	if identPath != "" {
		return identPath
	}

	if forTest != "" {
		return forTest
	}

	return currentPackage
}

func (m *InstrumentationManager) isDefinedInPackage(functionName, packageName string) bool {
	state, ok := m.packages[packageName]
	if ok {
		_, ok = state.tracedFuncs[functionName]
		return ok
	}

	return false
}

// getInvocationInfoFromCall returns a collection of data about a function call if it was defined
// in the scope of this application. If the expression passed does not contain a valid function
// call, this method will return nil.
//
// TODO: support function literals
//
// NOTE: unlike getInvocationInfo, this method does not recursively search for invocations.
func (m *InstrumentationManager) getInvocationInfoFromCall(call *dst.CallExpr, forTest string) *invocationInfo {
	functionCallIdent, ok := call.Fun.(*dst.Ident)
	if !ok {
		return nil
	}

	path := resolvePath(functionCallIdent.Path, m.getPackageName(), forTest)
	pkg, ok := m.packages[path]
	if ok && pkg.tracedFuncs[functionCallIdent.Name] != nil {
		return &invocationInfo{
			functionName: functionCallIdent.Name,
			packageName:  path,
			call:         call,
			decl:         pkg.tracedFuncs[functionCallIdent.Name].body,
		}
	}

	return nil
}

// findInvocationInfo post order traverses the node and searches for function calls that are defined
// within the current application and is reachable by tracing. This method
// returns a slice of data about the discovered function call(s) that can have tracing propagated
// to them automatically. If no matches are found, an empty list will be returned.
//
// The node passed is a dst node that you want to search for a function call in.
// The state passed is the current state of the application, and is used to resolve function literals.
// The packageOverride is an optional parameter that allows you to override the package name of the function call,
// and should only be used if you know the package the declaration lives in. This is useful for resolving
// function calls in unit tests becasue the package linking is not handled the same way as in the main application.
//
// If the node does not contain a function call made to a function declared in this application, this method will return nil.
//
// Possible cases:
// 1. A function call to a function in the current package: f()
// 2. A function call chain of functions: f().g().x()
// 3. A function call containing nested function calls: f(g(x()))
func (m *InstrumentationManager) findInvocationInfo(node dst.Node, state *tracestate.State) []*invocationInfo {
	invInfo := []*invocationInfo{}

	dst.Inspect(node, func(n dst.Node) bool {
		switch v := n.(type) {
		case *dst.BlockStmt:
			return false
		case *dst.CallExpr:
			call := v
			_, ok := state.GetFuncLitVariable(m.getDecoratorPackage(), call.Fun)
			if ok {
				invInfo = append(invInfo, &invocationInfo{
					functionName: "Function Literal",
					packageName:  m.getPackageName(),
					call:         call,
				})
			}
			switch fun := call.Fun.(type) {
			case *dst.Ident:
				path := resolvePath(fun.Path, m.getPackageName(), "")
				pkg, ok := m.packages[path]
				if ok && pkg.tracedFuncs[fun.Name] != nil {
					invInfo = append(invInfo, &invocationInfo{
						functionName: fun.Name,
						packageName:  path,
						call:         call,
						decl:         pkg.tracedFuncs[fun.Name].body,
					})
				}
			case *dst.SelectorExpr:
				// Handle selector expressions like `f().g().x()`
				pkgName := util.PackagePath(fun.Sel, m.getDecoratorPackage())
				path := resolvePath(pkgName, m.getPackageName(), "")
				pkg, ok := m.packages[path]
				functionName := fun.Sel.Name

				// Check if the function is defined in a package of this application.
				// If true, tracing can be passed into it.
				if ok && pkg.tracedFuncs[functionName] != nil {
					invInfo = append(invInfo, &invocationInfo{
						functionName: functionName,
						packageName:  path,
						call:         call,
						decl:         pkg.tracedFuncs[functionName].body,
					})
				}
			}
			return true
		}
		return true
	})

	return invInfo
}

// IsTracingComplete returns true if a function has all the tracing it needs added to it.
func (m *InstrumentationManager) shouldInstrumentFunction(inv *invocationInfo) bool {
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

// getSortedPackages returns a sorted list of package names.
// this performs a log(n) sort
func (m *InstrumentationManager) getSortedPackages() []string {
	keys := make([]string, 0, len(m.packages))
	for k := range m.packages {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// WriteDiff writes out the changes made to a file to the diff file for this package.
// onProgress is a callback function that is invoked before writing each file diff.
// This allows the caller (e.g., the CLI UI) to receive granular progress updates
// containing the name of the file currently being processed.
func (m *InstrumentationManager) WriteDiff(onProgress func(string)) error {
	pkgs := m.getSortedPackages()
	for _, pkg := range pkgs {
		state := m.packages[pkg]
		r := decorator.NewRestorerWithImports(state.pkg.Dir, gopackages.New(state.pkg.Dir))

		for _, file := range state.pkg.Syntax {
			if util.IsGenerated(state.pkg.Decorator, file) { // never alter generated files, and do not include them in the diff
				continue
			}
			path := state.pkg.Decorator.Filenames[file]
			originalFile, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// what this file will be named in the diff file
			var diffFileName string

			absAppPath, err := filepath.Abs(m.userAppPath)
			if err != nil {
				return err
			}
			diffFileName, err = filepath.Rel(absAppPath, path)
			if err != nil {
				return err
			}

			if onProgress != nil {
				onProgress(fmt.Sprintf("Writing diff for %s", diffFileName))
			}

			modifiedFile := bytes.NewBuffer([]byte{})
			if err := r.Fprint(modifiedFile, file); err != nil {
				return err
			}

			f, err := os.OpenFile(m.diffFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				f.Close()
				return err
			}

			defer f.Close()

			patch := godiffpatch.GeneratePatch(diffFileName, string(originalFile), modifiedFile.String())
			if _, err := f.WriteString(patch); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *InstrumentationManager) AddRequiredModules() error {
	for _, state := range m.packages {
		if util.IsTestPackage(state.pkg) {
			continue
		}
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %v", err)
		}

		// change to the directory of the package being traced so that go get adds dependencies to the right place
		if state.pkg.Dir != "" {
			defer func() {
				err := os.Chdir(wd)
				if err != nil {
					log.Printf("error changing back to working directory: %v", err)
				}
			}()

			err = os.Chdir(state.pkg.Dir)
			if err != nil {
				return err
			}
		}

		for module := range state.importsAdded {
			if module == "" {
				continue
			}

			goGet := exec.Command("go", "get", module)
			err := goGet.Run()
			if err != nil {
				return fmt.Errorf("failed to execute \"%s\": %v", goGet.String(), err)
			}
		}
	}

	return nil
}

func (m *InstrumentationManager) TracePackageCalls() error {
	return tracePackageFunctionCalls(m, m.tracingFunctions.dependency...)

}

// ScanApplication scans the existing Go application without adding instrumentation to the source code.
// This will not generate any changes to the actual source code, just the abstract syntax tree generated from it.
func (m *InstrumentationManager) ScanApplication() error {

	tracingFunctions := m.tracingFunctions.preinstrumentation

	return scanPackages(m, tracingFunctions...)

}

// InstrumentApplication applies instrumentation in place to the dst files stored in the InstrumentationManager.
// This will not generate any changes to the actual source code, just the abstract syntax tree generated from it.
// Note: only pass tracing functions to this method for testing, or if you sincerely know what you are doing.
func (m *InstrumentationManager) InstrumentApplication() error {
	tracingFunctions := m.tracingFunctions.stateless

	return instrumentPackages(m, tracingFunctions...)
}

func errorNoMain(path string) error {
	return fmt.Errorf("cannot find a main method in %s; instrumenting applications without a main method is not supported", path)
}

// traceFunctionCalls discovers and sets up tracing for all function calls in the current package
func tracePackageFunctionCalls(manager *InstrumentationManager, factDiscoveryFunctions ...FactDiscoveryFunction) error {
	hasMain := false
	var errReturn error

	for packageName, pkg := range manager.packages {
		if util.IsTestPackage(pkg.pkg) {
			continue
		}
		manager.setPackage(packageName)

		for _, file := range pkg.pkg.Syntax {
			pos := util.Position(file, pkg.pkg)
			if pos != nil && util.IsGenerated(pkg.pkg.Decorator, file) {
				continue
			}

			for _, decl := range file.Decls {
				if fn, isFn := decl.(*dst.FuncDecl); isFn {
					manager.createFunctionDeclaration(fn)
					if fn.Name.Name == "main" {
						hasMain = true
					}
				}
				if len(factDiscoveryFunctions) > 0 {
					dst.Inspect(decl, func(n dst.Node) bool {
						for _, scan := range factDiscoveryFunctions {
							entry, ok := scan(manager.getDecoratorPackage(), n)
							if ok {
								err := manager.facts.AddFact(entry)
								if err != nil {
									errReturn = fmt.Errorf("error adding fact entry %s: %v", entry, err)
									return false
								}
							}
						}
						return true
					})
				}
			}
		}
	}

	if !hasMain {
		noMain := errorNoMain(manager.userAppPath)
		if errReturn != nil {
			return fmt.Errorf("%w; %w", errReturn, noMain)
		}
		return noMain
	}
	return errReturn
}

// apply instrumentation to the package
func instrumentPackages(manager *InstrumentationManager, instrumentationFunctions ...StatelessTracingFunction) error {
	if instrumentationFunctions == nil {
		return fmt.Errorf("error instrumenting packages: instrumentation functions are nil")
	}
	for pkgName, pkgState := range manager.packages {
		if util.IsTestPackage(pkgState.pkg) {
			continue
		}
		manager.setPackage(pkgName)
		for _, file := range pkgState.pkg.Syntax {
			for _, decl := range file.Decls {
				if fn, isFn := decl.(*dst.FuncDecl); isFn {
					dstutil.Apply(fn, nil, func(c *dstutil.Cursor) bool {
						for _, instFunc := range instrumentationFunctions {
							instFunc(manager, c)
						}
						return true
					})
				}
			}
		}
	}
	return nil
}

// Does not apply instrumentation to the package, only scans it.
func scanPackages(manager *InstrumentationManager, instrumentationFunctions ...PreInstrumentationTracingFunction) error {
	if instrumentationFunctions == nil {
		return fmt.Errorf("error scanning packages: instrumentation functions are nil")
	}
	for pkgName, pkgState := range manager.packages {
		if util.IsTestPackage(pkgState.pkg) {
			continue
		}
		manager.setPackage(pkgName)
		for _, file := range pkgState.pkg.Syntax {
			for _, decl := range file.Decls {
				if fn, isFn := decl.(*dst.FuncDecl); isFn {
					dstutil.Apply(fn, nil, func(c *dstutil.Cursor) bool {
						for _, instFunc := range instrumentationFunctions {
							instFunc(manager, c)
						}
						return true
					})
				}
			}
		}
	}
	return nil
}

func (m *InstrumentationManager) ResolveUnitTests() error {
	for _, pkgState := range m.packages {
		// vet that this is a package created to test another package
		pkg := pkgState.pkg
		// NOTE: do not switch this to util.IsTestPackage(), it will not work
		if pkg.ForTest == "" {
			continue
		}

		for _, file := range pkg.Syntax {
			if util.IsGenerated(pkg.Decorator, file) {
				continue
			}
			for i, decl := range file.Decls {
				fn, ok := decl.(*dst.FuncDecl)
				if !ok {
					continue
				}

				// pointers to decls from the package being tested are coppied in test packages
				// and will modify the original decl if incorrectly modified by this function
				if m.isDefinedInPackage(fn.Name.Name, pkg.ForTest) {
					continue
				}

				// find all function calls and check if they are any of the functions we modified the parameters for
				// and update them accordingly.
				newDecl := dstutil.Apply(decl, func(c *dstutil.Cursor) bool {
					switch call := c.Node().(type) {
					case *dst.CallExpr:
						inv := m.getInvocationInfoFromCall(call, pkg.ForTest)
						if inv != nil && inv.decl != nil && inv.decl.Type.Params.List != nil && len(inv.decl.Type.Params.List) > 0 {
							// if the function declaration has a transaction as its last parameter, the test needs to be updated to do the same
							star, ok := inv.decl.Type.Params.List[len(inv.decl.Type.Params.List)-1].Type.(*dst.StarExpr)
							if !ok {
								return true
							}

							// guard agains duplicate parameters being added
							numParams := inv.decl.Type.Params.NumFields()
							if len(call.Args) == numParams {
								return true
							}

							txnIdent, ok := star.X.(*dst.Ident)
							if ok && txnIdent.Name == "Transaction" && txnIdent.Path == codegen.NewRelicAgentImportPath {
								// we have a match, now we need to update the call to include a transaction
								inv.call.Args = append(inv.call.Args, &dst.Ident{Name: "nil"})
							}
						}
					}
					return true
				}, nil)

				file.Decls[i] = newDecl.(dst.Decl)
			}
		}
	}

	return nil
}
