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
	"golang.org/x/tools/go/packages"
)

// createTestApp creates a test app in the given directory with the given file name and contents
// codegen is expensive, so this will be skipped in short mode
func createTestApp(t *testing.T, testAppDir, fileName, contents string) ([]*decorator.Package, error) {
	// integration tests are slow, so we skip them in short mode
	if testing.Short() {
		t.Skip("Skipping Stateful Tracing Function Integration Tests in short mode")
	}

	err := os.Mkdir(testAppDir, 0755)
	if err != nil {
		return nil, err
	}

	filepath := filepath.Join(testAppDir, fileName)

	f, err := os.Create(filepath)
	if err != nil {
		return nil, err
	}

	_, err = f.WriteString(contents)
	if err != nil {
		return nil, err
	}
	return decorator.Load(&packages.Config{Dir: testAppDir, Mode: packages.LoadSyntax})
}

func cleanTestApp(t *testing.T, appDirectoryName string) {
	err := os.RemoveAll(appDirectoryName)
	if err != nil {
		t.Logf("Failed to cleanup test app directory %s: %v", appDirectoryName, err)
	}
}

func panicRecovery(t *testing.T) {
	err := recover()
	if err != nil {
		t.Fatalf("%s recovered from panic: %+v\n\n%s", t.Name(), err, debug.Stack())
	}
}

func pseudo_uuid() (uuid string) {

	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	uuid = fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

	return
}

func testInstrumentationManager(t *testing.T, code, testAppDir string) *InstrumentationManager {
	defer panicRecovery(t)
	fileName := "app.go"
	pkgs, err := createTestApp(t, testAppDir, fileName, code)
	if err != nil {
		cleanTestApp(t, testAppDir)
		t.Fatal(err)
	}

	appName := ""
	varName := "NewRelicAgent"
	diffFile := filepath.Join(testAppDir, "new-relic-instrumentation.diff")

	manager := NewInstrumentationManager(pkgs, appName, varName, diffFile, testAppDir)
	configureTestInstrumentationManager(manager)
	return manager
}

func configureTestInstrumentationManager(manager *InstrumentationManager) error {
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

func testStatefulTracingFunction(t *testing.T, code string, stmtFunc StatefulTracingFunction, downstream bool) string {
	testDir := fmt.Sprintf("tmp_%s", pseudo_uuid())
	defer cleanTestApp(t, testDir)

	manager := testInstrumentationManager(t, code, testDir)
	pkg := manager.getDecoratorPackage()
	if pkg == nil {
		t.Fatalf("Package was nil: %+v", manager.packages)
	}
	node := pkg.Syntax[0].Decls[1]
	tracingState := TraceMain("app", "txn")
	if downstream {
		tracingState = TraceDownstreamFunction("txn")
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
	err := restorer.Fprint(buf, pkg.Syntax[0])
	if err != nil {
		t.Fatalf("Failed to restore the file: %v", err)
	}

	return buf.String()
}

func testStatelessTracingFunction(t *testing.T, code string, tracingFunc StatelessTracingFunction) string {
	testDir := fmt.Sprintf("tmp_%s", pseudo_uuid())
	defer cleanTestApp(t, testDir)

	manager := testInstrumentationManager(t, code, testDir)
	pkg := manager.getDecoratorPackage()
	if pkg == nil {
		t.Fatalf("Package was nil: %+v", manager.packages)
	}

	err := manager.InstrumentApplication(tracingFunc)
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
