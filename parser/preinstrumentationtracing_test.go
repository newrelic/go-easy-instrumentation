package parser

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/dave/dst/decorator"
	"github.com/dave/dst/decorator/resolver/guess"
	"github.com/stretchr/testify/assert"
)

func TestCodeModifications(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect map[string][]string
	}{
		{
			name: "Detect Transactions in Main",
			code: `package main

import (
	"fmt"
	"os"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)
func main() {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("Short Lived App"),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE_KEY")),
		newrelic.ConfigDebugLogger(os.Stdout),
	)
	if nil != err {
		fmt.Println(err)
		os.Exit(1)
	}

	tasks := []string{"white", "black", "red", "blue", "green", "yellow"}
	for _, task := range tasks {
		txn := app.StartTransaction("task")
		time.Sleep(10 * time.Millisecond)
		fmt.Println("Task:", task)
		txn.End()
		app.RecordCustomEvent("task", map[string]interface{}{
			"color": task,
		})
	}
	app.Shutdown(10 * time.Second)
}
`,

			expect: map[string][]string{
				"txn": {"Sleep", "Println", "txn.End"},
			},
		},
		{
			name: "Detect Transactions passed through function",
			code: `package main

import (
	"fmt"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func processTask(txn *newrelic.Transaction, task string) {
	time.Sleep(10 * time.Millisecond)
	fmt.Println("Processing task:", task)
	txn.End()
}

func main() {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("Short Lived App"),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE_KEY")),
		newrelic.ConfigDebugLogger(os.Stdout),
	)
	if nil != err {
		fmt.Println(err)
		return
	}

	tasks := []string{"task1", "task2", "task3"}
	for _, task := range tasks {
		txn := app.StartTransaction("task")
		processTask(txn, task)
	}
	app.Shutdown(10 * time.Second)
}`,

			expect: map[string][]string{
				"txn": {"processTask", "Sleep", "Println", "txn.End"},
			},
		},
		{
			name: "Logic after transaction ends should not be captured",
			code: `package main

import (
	"fmt"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func main() {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("Short Lived App"),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE_KEY")),
		newrelic.ConfigDebugLogger(os.Stdout),
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	txn := app.StartTransaction("task")
	fmt.Println("Starting task")
	txn.End()

	fmt.Println("Task ended. Performing cleanup.") // This should not be part of the transaction expressions
	time.Sleep(5 * time.Second)
	app.Shutdown(10 * time.Second)
}`,
			expect: map[string][]string{
				"txn": {"Println", "txn.End"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			id, err := pseudo_uuid()
			if err != nil {
				t.Fatal(err)
			}

			testDir := fmt.Sprintf("tmp_%s", id)
			defer cleanTestApp(t, testDir)

			manager := testInstrumentationManager(t, tt.code, testDir)
			pkg := manager.getDecoratorPackage()
			if pkg == nil {
				t.Fatalf("Package was nil: %+v", manager.packages)
			}

			manager.loadPreInstrumentationTracingFunctions(DetectTransactions)
			err = manager.ScanApplication()
			if err != nil {
				t.Fatalf("Failed to instrument packages: %v", err)
			}

			restorer := decorator.NewRestorerWithImports(testDir, guess.New())
			buf := bytes.NewBuffer([]byte{})
			err = restorer.Fprint(buf, pkg.Syntax[0])
			if err != nil {
				t.Fatalf("Failed to restore the file: %v", err)
			}
			_, exprs := manager.transactionCache.ExtractNames()
			assert.Equal(t, tt.expect, exprs)
		})
	}
}

func TestErrorDetection(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		expect map[string][]string
	}{
		{
			name: "Detect Existing Error in Function",
			code: `package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func hello(txn *newrelic.Transaction) {
	defer txn.StartSegment("hello").End()
	_, errFromHTTPCall := http.Get("https://example.com")
	if errFromHTTPCall != nil {
		txn.NoticeError(errFromHTTPCall)
	}
	txn.End()
}
func main() {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("Short Lived App"),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE_KEY")),
		newrelic.ConfigDebugLogger(os.Stdout),
	)
	if nil != err {
		fmt.Println(err)
		os.Exit(1)
	}

	// Wait for the application to connect.
	if err := app.WaitForConnection(5 * time.Second); nil != err {
		fmt.Println(err)
	}

	txn := app.StartTransaction("hello")
	hello(txn)
	// Shut down the application to flush data to New Relic.
	app.Shutdown(10 * time.Second)
}
`,
			expect: map[string][]string{
				"errors": {"errFromHTTPCall"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer panicRecovery(t)
			id, err := pseudo_uuid()
			if err != nil {
				t.Fatal(err)
			}

			testDir := fmt.Sprintf("tmp_%s", id)
			defer cleanTestApp(t, testDir)

			manager := testInstrumentationManager(t, tt.code, testDir)
			pkg := manager.getDecoratorPackage()
			if pkg == nil {
				t.Fatalf("Package was nil: %+v", manager.packages)
			}

			manager.loadPreInstrumentationTracingFunctions(DetectTransactions, DetectErrors)
			err = manager.ScanApplication()
			if err != nil {
				t.Fatalf("Failed to instrument packages: %v", err)
			}

			restorer := decorator.NewRestorerWithImports(testDir, guess.New())
			buf := bytes.NewBuffer([]byte{})
			err = restorer.Fprint(buf, pkg.Syntax[0])
			if err != nil {
				t.Fatalf("Failed to restore the file: %v", err)
			}
			exprs := manager.errorCache.ExtractExistingErrors()
			assert.Equal(t, tt.expect["errors"], exprs)
		})
	}
}
