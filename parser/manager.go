package parser

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gopackages"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser/errorcache"
	"github.com/newrelic/go-easy-instrumentation/parser/facts"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
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
	stateless  []StatelessTracingFunction
	stateful   []StatefulTracingFunction
	dependency []FactDiscoveryFunction
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
	packages          map[string]*packageState // stores stateful information on packages by ID
	errorCache        errorcache.ErrorCache    // stores error handling status for functions
}

// PackageManager contains state relevant to tracing within a single package.
type packageState struct {
	pkg          *decorator.Package             // the package being instrumented
	tracedFuncs  map[string]*tracedFunctionDecl // maintains state of tracing for functions within the package
	importsAdded map[string]bool                // tracks imports added to the package
}

// NewInstrumentationManager initializes an InstrumentationManager cache for a given package.
func NewInstrumentationManager(pkgs []*decorator.Package, appName, agentVariableName, diffFile, userAppPath string) *InstrumentationManager {
	comment.EnableConsolePrinter(userAppPath)

	manager := &InstrumentationManager{
		userAppPath:       userAppPath,
		diffFile:          diffFile,
		appName:           appName,
		agentVariableName: agentVariableName,
		packages:          map[string]*packageState{},
		facts:             facts.NewKeeper(),
		errorCache:        errorcache.ErrorCache{},
		tracingFunctions: tracingFunctions{
			stateless:  []StatelessTracingFunction{},
			stateful:   []StatefulTracingFunction{},
			dependency: []FactDiscoveryFunction{},
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
	m.loadStatelessTracingFunctions(InstrumentMain, InstrumentHandleFunction, InstrumentHttpClient, CannotInstrumentHttpMethod, InstrumentGrpcDial, InstrumentGinFunction, InstrumentGrpcServerMethod)
	m.loadStatefulTracingFunctions(ExternalHttpCall, WrapNestedHandleFunction, InstrumentGrpcServer, InstrumentGinMiddleware)
	m.loadDependencyScans(FindGrpcServerObject, FindGrpcServerStreamInterface)
	return nil
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
}

// GetPackageFunctionInvocation returns the name of the function being invoked, and the expression containing the call
// where that invocation occurs if a function is declared in this package.
//
// If the node does not contain a function call made to a function declared in this application, this method will return nil.
func (m *InstrumentationManager) getPackageFunctionInvocation(node dst.Node, state *tracestate.State) *invocationInfo {
	var invInfo *invocationInfo

	dst.Inspect(node, func(n dst.Node) bool {
		switch v := n.(type) {
		case *dst.BlockStmt:
			return false
		case *dst.CallExpr:
			call := v
			_, ok := state.GetFuncLitVariable(m.getDecoratorPackage(), call.Fun)
			if ok {
				invInfo = &invocationInfo{
					functionName: "Function Literal",
					packageName:  m.getPackageName(),
					call:         call,
				}
			}
			functionCallIdent, ok := call.Fun.(*dst.Ident)
			if !ok {
				return true
			}
			path := functionCallIdent.Path
			if path == "" {
				path = m.getPackageName()
			}
			pkg, ok := m.packages[path]
			if ok && pkg.tracedFuncs[functionCallIdent.Name] != nil {
				invInfo = &invocationInfo{
					functionName: functionCallIdent.Name,
					packageName:  path,
					call:         call,
				}
				return false
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

// GetDeclaration returns a pointer to the location in the DST tree where a function is declared and defined.
func (m *InstrumentationManager) getDeclaration(functionName string) *dst.FuncDecl {
	if m.packages[m.currentPackage] != nil && m.packages[m.currentPackage].tracedFuncs != nil {
		v, ok := m.packages[m.currentPackage].tracedFuncs[functionName]
		if ok {
			return v.body
		}
	}
	return nil
}

// WriteDiff writes out the changes made to a file to the diff file for this package.
func (m *InstrumentationManager) WriteDiff() error {
	for _, state := range m.packages {
		r := decorator.NewRestorerWithImports(state.pkg.Dir, gopackages.New(state.pkg.Dir))

		for _, file := range state.pkg.Syntax {
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
	log.Printf("changes written to %s", m.diffFile)
	return nil
}

func (m *InstrumentationManager) AddRequiredModules() error {
	for _, state := range m.packages {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %v", err)
		}

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

		for module := range state.importsAdded {
			err := exec.Command("go", "get", module).Run()
			if err != nil {
				return fmt.Errorf("error Getting GO module %s: %v", module, err)
			}
		}
	}

	return nil
}

// InstrumentApplication applies instrumentation in place to the dst files stored in the InstrumentationManager.
// This will not generate any changes to the actual source code, just the abstract syntax tree generated from it.
// Note: only pass tracing functions to this method for testing, or if you sincerely know what you are doing.
func (m *InstrumentationManager) InstrumentApplication(instrumentationFunctions ...StatelessTracingFunction) error {
	// Create a call graph of all calls made to functions in this package
	err := tracePackageFunctionCalls(m, m.tracingFunctions.dependency...)
	if err != nil {
		return err
	}

	tracingFunctions := m.tracingFunctions.stateless
	if len(instrumentationFunctions) != 0 {
		tracingFunctions = instrumentationFunctions
	}

	instrumentPackages(m, tracingFunctions...)

	return nil
}

// traceFunctionCalls discovers and sets up tracing for all function calls in the current package
func tracePackageFunctionCalls(manager *InstrumentationManager, factDiscoveryFunctions ...FactDiscoveryFunction) error {
	hasMain := false
	var errReturn error

	for packageName, pkg := range manager.packages {
		manager.setPackage(packageName)

		for _, file := range pkg.pkg.Syntax {
			pos := util.Position(file, pkg.pkg)
			if pos != nil && strings.Contains(pos.Filename, ".pb.go") {
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
		errors.Join(errReturn, errors.New("cannot find a main method for this application; applications without main methods can not be instrumented"))
	}
	return errReturn
}

// apply instrumentation to the package
func instrumentPackages(manager *InstrumentationManager, instrumentationFunctions ...StatelessTracingFunction) {
	for pkgName, pkgState := range manager.packages {
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
}
