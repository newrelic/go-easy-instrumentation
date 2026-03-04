# Integration Development Guide

This guide explains how to create new integrations for go-easy-instrumentation and register them with the core instrumentation engine.

## Table of Contents

- [Overview](#overview)
- [Integration Structure](#integration-structure)
- [Step-by-Step: Creating a New Integration](#step-by-step-creating-a-new-integration)
- [Writing Parsing Logic](#writing-parsing-logic)
- [Writing Code Generation](#writing-code-generation)
- [Writing Tests](#writing-tests)
- [Registration](#registration)
- [Best Practices](#best-practices)
- [Examples](#examples)

---

## Overview

Each integration in go-easy-instrumentation follows a **self-contained package structure** where all parsing logic, code generation, tests, and examples live together in one directory.

### What is an Integration?

An integration detects usage of a specific Go library (like Gin, gRPC, MySQL) and automatically adds New Relic instrumentation to it through static analysis.

### Types of Tracing Functions

There are three types of tracing functions you can implement:

1. **Stateless Tracing Functions** - Run once per statement, no dependency on other tracing
2. **Stateful Tracing Functions** - Depend on tracing state from previous passes
3. **Fact Discovery Functions** - Scan for patterns to cache as "facts" for later use

---

## Integration Structure

Each integration should follow this structure:

```
integrations/nr{library}/
├── {library}.go           # Parsing/detection logic
├── codegen.go             # DST node generation
├── parsing_test.go        # Tests for parsing logic
├── codegen_test.go        # Tests for codegen
└── example/               # End-to-end examples
    ├── {example1}/
    │   ├── main.go
    │   └── expect.ref     # Expected instrumentation output
    └── go.mod             # Shared go.mod for examples
```

**File naming conventions:**
- Main parsing logic: `{library}.go` (e.g., `gin.go`, `mysql.go`)
- Codegen: `codegen.go`
- Package name: `nr{library}` (e.g., `nrgin`, `nrmysql`)

---

## Step-by-Step: Creating a New Integration

Let's create a hypothetical integration for **Redis** as an example.

### Step 1: Create the Package Structure

```bash
mkdir -p integrations/nrredis/example
touch integrations/nrredis/redis.go
touch integrations/nrredis/codegen.go
touch integrations/nrredis/parsing_test.go
touch integrations/nrredis/codegen_test.go
```

### Step 2: Define Constants

In `redis.go`, start with import path constants:

```go
package nrredis

import (
    "github.com/dave/dst"
    "github.com/dave/dst/dstutil"
    "github.com/newrelic/go-easy-instrumentation/parser"
    "github.com/newrelic/go-easy-instrumentation/parser/tracestate"
)

const (
    RedisImportPath   = "github.com/redis/go-redis/v9"
    NrRedisImportPath = "github.com/newrelic/go-agent/v3/integrations/nrredis"
)
```

### Step 3: Write Detection Functions

Create helper functions to detect library usage:

```go
// detectRedisClient checks if a statement creates a Redis client
func detectRedisClient(stmt dst.Stmt) (*dst.Ident, bool) {
    assign, ok := stmt.(*dst.AssignStmt)
    if !ok || len(assign.Rhs) != 1 {
        return nil, false
    }

    call, ok := assign.Rhs[0].(*dst.CallExpr)
    if !ok {
        return nil, false
    }

    // Check if it's redis.NewClient()
    ident, ok := call.Fun.(*dst.Ident)
    if !ok || ident.Name != "NewClient" || ident.Path != RedisImportPath {
        return nil, false
    }

    // Return the variable name
    if len(assign.Lhs) > 0 {
        if clientIdent, ok := assign.Lhs[0].(*dst.Ident); ok {
            return clientIdent, true
        }
    }

    return nil, false
}
```

### Step 4: Write Tracing Functions

Implement a tracing function that the manager will call:

```go
// InstrumentRedisClient adds New Relic hooks to Redis client creation
func InstrumentRedisClient(manager *parser.InstrumentationManager, c *dstutil.Cursor) {
    stmt, ok := c.Node().(dst.Stmt)
    if !ok {
        return
    }

    clientIdent, found := detectRedisClient(stmt)
    if !found {
        return
    }

    // Generate instrumentation using codegen
    hook := WrapRedisClient(clientIdent)

    // Insert after the original statement
    c.InsertAfter(hook)

    // Add necessary import
    manager.AddImport(NrRedisImportPath)

    // Add comment explaining the instrumentation
    comment.Debug(manager.GetDecoratorPackage(), stmt,
        "Instrumenting Redis client: "+clientIdent.Name)
}
```

---

## Writing Parsing Logic

### Key Principles

1. **Pattern Matching**: Use the DST to identify library-specific patterns
2. **Type Checking**: Use `util.TypeOf()` to verify types when needed
3. **State Management**: Use `manager` to access tracing state and facts
4. **Non-invasive**: Only detect, don't modify directly (use codegen)

### Common Patterns

**Pattern 1: Detect function calls**
```go
func isFunctionCall(node dst.Node, funcName, importPath string) bool {
    call, ok := node.(*dst.CallExpr)
    if !ok {
        return false
    }

    ident, ok := call.Fun.(*dst.Ident)
    return ok && ident.Name == funcName && ident.Path == importPath
}
```

**Pattern 2: Find variable assignments**
```go
func getAssignedVariable(stmt dst.Stmt) (*dst.Ident, bool) {
    assign, ok := stmt.(*dst.AssignStmt)
    if !ok || len(assign.Lhs) == 0 {
        return nil, false
    }

    ident, ok := assign.Lhs[0].(*dst.Ident)
    return ident, ok
}
```

**Pattern 3: Check for specific types**
```go
func isRedisClientType(ident *dst.Ident, pkg *decorator.Package) bool {
    typ := util.TypeOf(ident, pkg)
    if typ == nil {
        return false
    }
    return typ.String() == "*github.com/redis/go-redis/v9.Client"
}
```

---

## Writing Code Generation

Code generation functions create DST nodes for instrumentation code.

### Key Principles

1. **Pure Functions**: Codegen should not depend on manager state
2. **Return DST Nodes**: Always return `dst.Node` or specific types like `*dst.CallExpr`
3. **Include Decorations**: Add proper spacing/newlines with `Decs`
4. **Return Import Paths**: Return which imports are needed

### Example: Generate a Wrapper Call

```go
package nrredis

import (
    "github.com/dave/dst"
    "github.com/newrelic/go-easy-instrumentation/internal/codegen"
)

const NrRedisImportPath = "github.com/newrelic/go-agent/v3/integrations/nrredis"

// WrapRedisClient wraps a Redis client with New Relic instrumentation
// Returns: rdb.AddHook(nrredis.NewHook(txn))
func WrapRedisClient(clientVar *dst.Ident, txn dst.Expr) *dst.ExprStmt {
    return &dst.ExprStmt{
        X: &dst.CallExpr{
            Fun: &dst.SelectorExpr{
                X:   clientVar,
                Sel: &dst.Ident{Name: "AddHook"},
            },
            Args: []dst.Expr{
                &dst.CallExpr{
                    Fun: &dst.Ident{
                        Name: "NewHook",
                        Path: NrRedisImportPath,
                    },
                    Args: []dst.Expr{txn},
                },
            },
        },
        Decs: dst.ExprStmtDecorations{
            NodeDecs: dst.NodeDecs{
                Before: dst.NewLine,
            },
        },
    }
}
```

### Reusing Shared Codegen

Use helpers from `internal/codegen/` for common patterns:

```go
import "github.com/newrelic/go-easy-instrumentation/internal/codegen"

// Start a transaction
txnStart := codegen.StartTransaction("transactionName", agentVar)

// Create a context with transaction
ctx := codegen.TxnFromContext("txn", contextExpr)

// End a transaction
txnEnd := codegen.EndTransaction(txnVar)

// Add a segment
segment := codegen.DeferSegment(txnVar, "segmentName")
```

---

## Writing Tests

### Parsing Tests

Test that your detection logic works correctly:

```go
package nrredis_test

import (
    "testing"

    "github.com/newrelic/go-easy-instrumentation/integrations/nrredis"
    "github.com/newrelic/go-easy-instrumentation/parser"
    "github.com/stretchr/testify/assert"
)

func TestInstrumentRedisClient(t *testing.T) {
    tests := []struct {
        name   string
        code   string
        expect string
    }{
        {
            name: "instrument redis client creation",
            code: `package main

import "github.com/redis/go-redis/v9"

func main() {
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
}`,
            expect: `package main

import (
    "github.com/redis/go-redis/v9"
    "github.com/newrelic/go-agent/v3/integrations/nrredis"
)

func main() {
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    rdb.AddHook(nrredis.NewHook(txn))
}`,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            defer parser.PanicRecovery(t)
            got := parser.RunStatelessTracingFunction(t, tt.code, nrredis.InstrumentRedisClient)
            assert.Equal(t, tt.expect, got)
        })
    }
}
```

### Codegen Tests

Test that generated DST nodes are correct:

```go
func TestWrapRedisClient(t *testing.T) {
    clientVar := &dst.Ident{Name: "rdb"}
    txnVar := &dst.Ident{Name: "txn"}

    stmt := nrredis.WrapRedisClient(clientVar, txnVar)

    // Verify the structure
    assert.NotNil(t, stmt)
    exprStmt, ok := stmt.(*dst.ExprStmt)
    assert.True(t, ok)

    call, ok := exprStmt.X.(*dst.CallExpr)
    assert.True(t, ok)

    sel, ok := call.Fun.(*dst.SelectorExpr)
    assert.True(t, ok)
    assert.Equal(t, "AddHook", sel.Sel.Name)
}
```

---

## Registration

### Step 1: Register Your Tracing Functions

Edit `parser/manager.go` and add your integration:

```go
import (
    // ... existing imports ...
    nrredis "github.com/newrelic/go-easy-instrumentation/integrations/nrredis"
)

func (m *InstrumentationManager) DetectDependencyIntegrations() error {
    // Pre-instrumentation scanning phase
    m.loadPreInstrumentationTracingFunctions(
        // ... existing functions ...
    )

    // Stateless tracing functions
    m.loadStatelessTracingFunctions(
        // ... existing functions ...
        nrredis.InstrumentRedisClient,  // ADD YOUR FUNCTION HERE
    )

    // Stateful tracing functions (if needed)
    m.loadStatefulTracingFunctions(
        // ... existing functions ...
    )

    // Fact discovery functions (if needed)
    m.loadDependencyScans(
        // ... existing functions ...
    )

    return nil
}
```

### Step 2: Add End-to-End Tests

Create a test application in `integrations/nrredis/example/basic/`:

```go
// integrations/nrredis/example/basic/main.go
package main

import "github.com/redis/go-redis/v9"

func main() {
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    rdb.Set(ctx, "key", "value", 0)
}
```

Generate the expected output:
```bash
go run . instrument integrations/nrredis/example/basic --output /tmp/redis.diff
cp /tmp/redis.diff integrations/nrredis/example/basic/expect.ref
```

Add to `validation-tests/testcases.json`:
```json
{
  "tests": [
    {
      "name": "redis app",
      "dir": "integrations/nrredis/example/basic"
    }
  ]
}
```

---

## Best Practices

### 1. Start Simple
- Begin with the most common use case
- Add complexity incrementally
- Test each addition

### 2. Follow Existing Patterns
Look at similar integrations:
- **Simple integration**: `nrslog` (150 lines)
- **Medium integration**: `nrgin` (745 lines)
- **Complex integration**: `nrnethttp` (3,009 lines)

### 3. Documentation
- Add comments to exported functions
- Document what DST patterns you're matching
- Explain any non-obvious logic

### 4. Error Handling
- Use `comment.Debug()` to add explanatory comments to instrumented code
- Use `comment.Warn()` for limitations or caveats
- Fail gracefully - don't panic, just skip instrumentation

### 5. Testing Strategy
```
Unit Tests (codegen_test.go)
    ↓
Integration Tests (parsing_test.go)
    ↓
End-to-End Tests (example/)
    ↓
Manual Testing (real applications)
```

### 6. Code Organization

**DO:**
- Keep helper functions unexported (lowercase)
- Export only tracing functions called by manager
- Group related functions together
- Use clear, descriptive names

**DON'T:**
- Mix parsing and codegen in the same file
- Export internal helpers
- Create circular dependencies with `parser/`

### 7. Import Management

Always return import paths from codegen:
```go
// Good
func GenerateInstrumentation() (*dst.Stmt, string) {
    return stmt, NrRedisImportPath
}

// Bad - caller doesn't know what import is needed
func GenerateInstrumentation() *dst.Stmt {
    return stmt
}
```

---

## Examples

### Example 1: Simple Stateless Integration (Logging)

```go
// integrations/nrslog/slog.go
package nrslog

func InstrumentSlogHandler(manager *parser.InstrumentationManager, c *dstutil.Cursor) {
    // Detect slog.New() calls
    call, ok := detectSlogNew(c.Node())
    if !ok {
        return
    }

    // Wrap handler with New Relic
    wrappedHandler := WrapSlogHandler(call, manager.AgentVariable())
    c.Replace(wrappedHandler)

    manager.AddImport(NrslogImportPath)
}
```

### Example 2: Stateful Integration (HTTP Handler)

```go
// integrations/nrnethttp/http.go
func InstrumentHttpHandler(manager *parser.InstrumentationManager,
                          stmt dst.Stmt,
                          c *dstutil.Cursor,
                          tracing *tracestate.State) bool {

    // Check if we have transaction context
    if !tracing.HasTransaction() {
        return false
    }

    // Detect http.HandleFunc
    handler, ok := detectHandleFunc(stmt)
    if !ok {
        return false
    }

    // Wrap with transaction
    wrapped := WrapHttpHandler(handler, tracing.Transaction())
    c.Replace(wrapped)

    return true // Instrumentation applied
}
```

### Example 3: Fact Discovery (gRPC Server)

```go
// integrations/nrgrpc/grpc.go
func FindGrpcServerObject(pkg *decorator.Package, node dst.Node) (facts.Entry, bool) {
    // Look for grpc.RegisterXXXServer() calls
    call, ok := findRegisterServerCall(node)
    if !ok {
        return facts.Entry{}, false
    }

    // Extract the server implementation type
    serverType := getServerType(call, pkg)
    if serverType == "" {
        return facts.Entry{}, false
    }

    // Cache this as a fact for later use
    return facts.Entry{
        Name: serverType,
        Fact: facts.GrpcServerType,
    }, true
}
```

---

## Checklist

Before submitting your integration, ensure:

- [ ] Package name follows `nr{library}` convention
- [ ] Files organized: `{library}.go`, `codegen.go`, `*_test.go`
- [ ] Constants defined for import paths
- [ ] Tracing functions implemented
- [ ] Codegen functions return DST nodes
- [ ] Unit tests for parsing logic
- [ ] Unit tests for codegen
- [ ] At least one end-to-end example
- [ ] Registered in `parser/manager.go`
- [ ] Added to `validation-tests/testcases.json`
- [ ] All tests passing: `go test ./...`
- [ ] Validation tests passing: `./validation-tests/testrunner`
- [ ] Documentation comments on exported functions

---

## Getting Help

- **Examples**: Look at existing integrations in `integrations/`
- **Architecture**: See `/Users/mkara/.claude/projects/-Users-mkara-go-easy-instrumentation/memory/architecture.md`
- **Questions**: Check project README or open an issue

---

## Reference

### Useful Imports

```go
// Core parsing
"github.com/dave/dst"
"github.com/dave/dst/dstutil"
"github.com/dave/dst/decorator"

// Project core
"github.com/newrelic/go-easy-instrumentation/parser"
"github.com/newrelic/go-easy-instrumentation/parser/tracestate"
"github.com/newrelic/go-easy-instrumentation/parser/facts"

// Utilities
"github.com/newrelic/go-easy-instrumentation/internal/codegen"
"github.com/newrelic/go-easy-instrumentation/internal/util"
"github.com/newrelic/go-easy-instrumentation/internal/comment"
```

### Key Manager Methods

```go
manager.AddImport(importPath string)                          // Add import to file
manager.GetDecoratorPackage() *decorator.Package              // Get current package
manager.Facts() facts.Keeper                                   // Access cached facts
manager.TransactionCache() *transactioncache.TransactionCache // Access transaction state
```

### Tracing Function Signatures

```go
// Stateless
type StatelessTracingFunction func(*InstrumentationManager, *dstutil.Cursor)

// Stateful
type StatefulTracingFunction func(*InstrumentationManager, dst.Stmt, *dstutil.Cursor, *tracestate.State) bool

// Pre-instrumentation (fact discovery)
type PreInstrumentationTracingFunction func(*decorator.Package, dst.Node) (facts.Entry, bool)
```

---

**Happy instrumenting! 🎉**
