// testhelpers.go provides shared test utilities for parser and integration tests.
// These functions are used across multiple test files and packages.
package parser

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/guess"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
	"golang.org/x/tools/go/packages"
)

// CreateTestApp creates a test app in the given directory with the given file name and contents.
// Codegen is expensive, so this will be skipped in short mode.
func CreateTestApp(t *testing.T, testAppDir, fileName, contents string) ([]*decorator.Package, error) {
	if testing.Short() {
		t.Skip("Skipping Stateful Tracing Function Integration Tests in short mode")
	}

	err := os.Mkdir(testAppDir, 0755)
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(testAppDir, fileName)
	f, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	_, err = f.WriteString(contents)
	if err != nil {
		return nil, err
	}
	f.Close()

	// Create go.mod to support Go 1.25+ which requires modules for package loading
	if err := os.WriteFile(filepath.Join(testAppDir, "go.mod"), []byte("module testapp\n\ngo 1.24\n"), 0644); err != nil {
		return nil, err
	}

	return decorator.Load(&packages.Config{Dir: testAppDir, Mode: packages.LoadSyntax})
}

// CleanTestApp removes the test app directory.
func CleanTestApp(t *testing.T, appDirectoryName string) {
	err := os.RemoveAll(appDirectoryName)
	if err != nil {
		t.Logf("Failed to cleanup test app directory %s: %v", appDirectoryName, err)
	}
}

// PanicRecovery recovers from panics in tests and reports them with stack traces.
func PanicRecovery(t *testing.T) {
	err := recover()
	if err != nil {
		t.Fatalf("%s recovered from panic: %+v\n\n%s", t.Name(), err, debug.Stack())
	}
}

func Pseudo_uuid() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("Failed to generate random number from bytes: %v", err)
	}
	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func TestInstrumentationManager(t *testing.T, code, testAppDir string) *InstrumentationManager {
	defer PanicRecovery(t)
	fileName := "app.go"
	pkgs, err := CreateTestApp(t, testAppDir, fileName, code)
	if err != nil {
		CleanTestApp(t, testAppDir)
		t.Fatal(err)
	}

	appName := ""
	varName := "NewRelicAgent"
	diffFile := filepath.Join(testAppDir, "new-relic-instrumentation.diff")

	manager := NewInstrumentationManager(pkgs, appName, varName, diffFile, testAppDir)
	ConfigureTestInstrumentationManager(manager)
	return manager
}

func ConfigureTestInstrumentationManager(manager *InstrumentationManager) error {
	pkgs := []string{}
	for pkg := range manager.packages {
		if pkg != "" {
			pkgs = append(pkgs, pkg)
		}
	}

	if len(pkgs) == 0 {
		return fmt.Errorf("no usable packages found in manager: %+v", manager.packages)
	}
	manager.setPackage(pkgs[0])
	return nil
}

// RunStatefulTracingFunction runs a stateful tracing function against test code.
func RunStatefulTracingFunction(t *testing.T, code string, stmtFunc StatefulTracingFunction, downstream bool) string {
	id, err := Pseudo_uuid()
	if err != nil {
		t.Fatal(err)
	}

	testDir := fmt.Sprintf("tmp_%s", id)
	defer CleanTestApp(t, testDir)

	manager := TestInstrumentationManager(t, code, testDir)
	pkg := manager.getDecoratorPackage()
	if pkg == nil {
		t.Fatalf("Package was nil: %+v", manager.packages)
	}
	node := pkg.Syntax[0].Decls[1]
	tracingState := tracestate.Main("app")
	if downstream {
		tracingState = tracestate.FunctionBody("txn")
	}

	dstutil.Apply(node, nil, func(c *dstutil.Cursor) bool {
		n := c.Node()
		switch v := n.(type) {
		case dst.Stmt:
			stmtFunc(manager, v, c, tracingState)
		}
		return true
	})
	restorer := decorator.NewRestorerWithImports(testDir, guess.New())

	buf := bytes.NewBuffer([]byte{})
	err = restorer.Fprint(buf, pkg.Syntax[0])
	if err != nil {
		t.Fatalf("Failed to restore the file: %v", err)
	}

	return buf.String()
}

// RunStatelessTracingFunction runs a stateless tracing function against test code.
func RunStatelessTracingFunction(t *testing.T, code string, tracingFunc StatelessTracingFunction, statefulTracingFuncs ...StatefulTracingFunction) string {
	id, err := Pseudo_uuid()
	if err != nil {
		t.Fatal(err)
	}

	testDir := fmt.Sprintf("tmp_%s", id)
	defer CleanTestApp(t, testDir)

	manager := TestInstrumentationManager(t, code, testDir)
	pkg := manager.getDecoratorPackage()
	if pkg == nil {
		t.Fatalf("Package was nil: %+v", manager.packages)
	}

	manager.tracingFunctions.stateful = append(manager.tracingFunctions.stateful, statefulTracingFuncs...)
	manager.tracingFunctions.stateless = append(manager.tracingFunctions.stateless, tracingFunc)
	err = manager.TracePackageCalls()
	if err != nil {
		t.Fatalf("Failed to trace package calls: %v", err)
	}
	err = manager.InstrumentApplication()
	if err != nil {
		t.Fatalf("Failed to instrument packages: %v", err)
	}

	restorer := decorator.NewRestorerWithImports(testDir, guess.New())
	buf := bytes.NewBuffer([]byte{})
	err = restorer.Fprint(buf, pkg.Syntax[0])
	if err != nil {
		t.Fatalf("Failed to restore the file: %v", err)
	}

	return buf.String()
}

// UnitTest creates a temporary test package from the given code string.
func UnitTest(t *testing.T, code string) []*decorator.Package {
	id, err := Pseudo_uuid()
	if err != nil {
		t.Fatal(err)
	}

	testAppDir := fmt.Sprintf("tmp_%s", id)
	fileName := "app.go"
	pkgs, err := CreateTestApp(t, testAppDir, fileName, code)
	defer CleanTestApp(t, testAppDir)
	if err != nil {
		t.Fatal(err)
	}

	return pkgs
}
