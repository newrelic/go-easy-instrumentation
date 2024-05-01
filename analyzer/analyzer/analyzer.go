package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var Analyzer = &analysis.Analyzer{
	Name:      "NewRelicGoAutoAgent",
	Doc:       "Automatically injects go agent code into your application",
	Run:       InstrumentPackage,
	FactTypes: []analysis.Fact{},
}

func InstrumentPackage(p *analysis.Pass) (any, error) {
	appName := "auto-instrumentation-test"
	agentName := "newrelicAgent"
	for _, f := range p.Files {
		InstrumentFile(p, f, appName, agentName)
	}

	return nil, nil
}

type InstrumentationData struct {
	P                 *analysis.Pass
	Fset              *token.FileSet
	AstFile           *ast.File
	AppName           string
	AgentVariableName string
}

type InstrumentationFunc func(n ast.Node, data *InstrumentationData) string

func InstrumentFile(p *analysis.Pass, astFile *ast.File, appName, agentVariableName string) {
	data := InstrumentationData{
		P:                 p,
		Fset:              p.Fset,
		AstFile:           astFile,
		AgentVariableName: agentVariableName,
		AppName:           appName,
	}

	// Pre Instrumentation Steps
	// 	- import the agent
	//	- initialize the agent
	//	- shutdown the agent
	downstreamFuncs := preInstrumentation(&data, InitializeAgent, InstrumentHandleFunc)
	fmt.Printf("Downstream funcs: %+v\n", downstreamFuncs)
	/*
	   // Main Instrumentation Loop
	   //	- any instrumentation that consumes the agent
	   //mainInstrumentationLoop(&data, InstrumentHandleFunc)

	   modifiedFile := bytes.NewBuffer([]byte{})
	   printer.Fprint(modifiedFile, p.Fset, astFile)
	*/
}

func preInstrumentation(data *InstrumentationData, instrumentationFunctions ...InstrumentationFunc) []string {
	downstreamFuncs := []string{}

	for i, d := range data.AstFile.Decls {
		newNode := astutil.Apply(d, nil, func(c *astutil.Cursor) bool {
			n := c.Node()
			if n != nil {
				for _, instFunc := range instrumentationFunctions {
					downstream := instFunc(n, data)
					if downstream != "" {
						downstreamFuncs = append(downstreamFuncs, downstream)
					}
				}
			}
			return true
		})

		if n, ok := newNode.(*ast.FuncDecl); ok {
			data.AstFile.Decls[i] = n
		}
	}

	return downstreamFuncs
}

/*
func mainInstrumentationLoop(data *InstrumentationData, instrumentationFunctions ...InstrumentationFunc) {
	for i, d := range data.AstFile.Decls {
		if fn, isFn := d.(*ast.FuncDecl); isFn {
			modifiedFunc := astutil.Apply(fn, nil, func(c *astutil.Cursor) bool {
				n := c.Node()
				for _, instFunc := range instrumentationFunctions {
					instFunc(n, data)
				}
				return true
			})
			if modifiedFunc != nil {
				data.AstFile.Decls[i] = modifiedFunc.(*ast.FuncDecl)
			}
		}
	}
}
*/
