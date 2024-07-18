package main

import (
	"bytes"
	"go/ast"
	"go/types"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/gopackages"
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

// InstrumentationManager maintains state relevant to tracing across all files and functions within a package.
type InstrumentationManager struct {
	diffFile          string
	appName           string
	agentVariableName string
	pkg               *decorator.Package
	tracedFuncs       map[string]*tracedFunction
	importsAdded      map[string]bool
}

const (
	newrelicAgentImport string = "github.com/newrelic/go-agent/v3/newrelic"
)

// NewInstrumentationManager initializes an InstrumentationManager cache for a given package.
func NewInstrumentationManager(pkg *decorator.Package, appName, agentVariableName, diffFile string) *InstrumentationManager {
	return &InstrumentationManager{
		diffFile:          diffFile,
		pkg:               pkg,
		appName:           appName,
		agentVariableName: agentVariableName,
		tracedFuncs:       map[string]*tracedFunction{},
		importsAdded:      map[string]bool{},
	}
}

func (d *InstrumentationManager) AddImport(path string) {
	d.importsAdded[path] = true
}

func (d *InstrumentationManager) GetImports(fileName string) []string {
	i := 0
	ret := make([]string, len(d.importsAdded))
	for k := range d.importsAdded {
		ret[i] = string(k)
		i++
	}
	return ret
}

// CreateFunctionDeclaration creates a tracking object for a function declaration that can be used
// to find tracing locations. This is for initializing and set up only.
func (d *InstrumentationManager) CreateFunctionDeclaration(decl *dst.FuncDecl) {
	_, ok := d.tracedFuncs[decl.Name.Name]
	if !ok {
		d.tracedFuncs[decl.Name.Name] = &tracedFunction{
			body: decl,
		}
	}
}

// UpdateFunctionDeclaration replaces the declaration stored for the given function name, and marks it as traced.
func (d *InstrumentationManager) UpdateFunctionDeclaration(decl *dst.FuncDecl) {
	t, ok := d.tracedFuncs[decl.Name.Name]
	if ok {
		t.body = decl
		t.traced = true
	}
}

// GetPackageFunctionInvocation returns the name of the function being invoked, and the expression containing the call
// where that invocation occurs if a function is declared in this package.
func (d *InstrumentationManager) GetPackageFunctionInvocation(node dst.Node) (string, *dst.CallExpr) {
	fnName := ""
	var pkgCall *dst.CallExpr
	dst.Inspect(node, func(n dst.Node) bool {
		switch v := n.(type) {
		case *dst.BlockStmt:
			return false
		case *dst.CallExpr:
			call := v
			functionCallIdent, ok := call.Fun.(*dst.Ident)
			if ok {
				astNode := d.pkg.Decorator.Ast.Nodes[functionCallIdent]
				switch astNodeType := astNode.(type) {
				case *ast.SelectorExpr:
					pkgID := astNodeType.X.(*ast.Ident)
					callPackage := d.pkg.TypesInfo.Uses[pkgID]
					if callPackage != nil && callPackage.Type().String() != "invalid type" {
						if callPackage.(*types.PkgName).Imported().Path() == d.pkg.PkgPath {
							fnName = astNodeType.Sel.Name
							pkgCall = call
							return false
						}
					}
				case *ast.Ident:
					pkgID := astNodeType
					callPackage := d.pkg.TypesInfo.Uses[pkgID]
					if callPackage != nil && callPackage.Type().String() != "invalid type" {
						if callPackage.Pkg().Path() == d.pkg.PkgPath {
							fnName = pkgID.Name
							pkgCall = call
							return false
						}
					}
				}
			}
			return true
		}
		return true
	})

	return fnName, pkgCall
}

// AddTxnArgumentToFuncDecl adds a transaction argument to the declaration of a function. This marks that function as needing a transaction,
// and can be looked up by name to know that the last argument is a transaction.
func (d *InstrumentationManager) AddTxnArgumentToFunctionDecl(decl *dst.FuncDecl, txnVarName, functionName string) {
	decl.Type.Params.List = append(decl.Type.Params.List, &dst.Field{
		Names: []*dst.Ident{dst.NewIdent(txnVarName)},
		Type: &dst.StarExpr{
			X: &dst.SelectorExpr{
				X:   dst.NewIdent("newrelic"),
				Sel: dst.NewIdent("Transaction"),
			},
		},
	})
	data, ok := d.tracedFuncs[functionName]
	if ok {
		data.requiresTxn = true
	}
}

// IsTracingComplete returns true if a function has all the tracing it needs added to it.
func (d *InstrumentationManager) ShouldInstrumentFunction(functionName string) bool {
	if functionName == "" {
		return false
	}
	v, ok := d.tracedFuncs[functionName]
	if ok {
		return !v.traced
	}
	return false
}

// RequiresTransactionArgument returns true if a modified function needs a transaction as an argument.
// This can be used to check if transactions should be passed by callers.
func (d *InstrumentationManager) RequiresTransactionArgument(functionName string) bool {
	if functionName == "" {
		return false
	}
	v, ok := d.tracedFuncs[functionName]
	if ok {
		return v.requiresTxn
	}
	return false
}

// GetDeclaration returns a pointer to the location in the DST tree where a function is declared and defined.
func (d *InstrumentationManager) GetDeclaration(functionName string) *dst.FuncDecl {
	v, ok := d.tracedFuncs[functionName]
	if ok {
		return v.body
	}
	return nil
}

// WriteDiff writes out the changes made to a file to the diff file for this package.
func (d *InstrumentationManager) WriteDiff() {
	r := decorator.NewRestorerWithImports(d.pkg.Dir, gopackages.New(d.pkg.Dir))

	for _, file := range d.pkg.Syntax {
		path := d.pkg.Decorator.Filenames[file]
		originalFile, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}

		basePath := strings.Split(path, d.pkg.ID)
		if len(basePath) == 0 {
			log.Fatalf("Could not find base path for file %s", path)
		}

		diffFileName, err := filepath.Rel(basePath[0], path)
		if err != nil {
			log.Fatal(err)
		}

		modifiedFile := bytes.NewBuffer([]byte{})
		if err := r.Fprint(modifiedFile, file); err != nil {
			log.Fatal(err)
		}

		f, err := os.OpenFile(d.diffFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

func (d *InstrumentationManager) AddRequiredModules() {
	err := os.Chdir(d.pkg.Dir)
	if err != nil {
		log.Fatal(err)
	}

	for module := range d.importsAdded {
		err := exec.Command("go", "get", module).Run()
		if err != nil {
			log.Fatalf("Error Getting GO module %s: %v", module, err)
		}
	}

	wd, _ := os.Getwd()
	err = os.Chdir(wd)
	if err != nil {
		log.Fatal(err)
	}
}
