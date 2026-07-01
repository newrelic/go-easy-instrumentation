package nrlogrus

import (
	"fmt"
	"go/token"

	"github.com/dave/dst"
	"github.com/dave/dst/dstutil"
	"github.com/newrelic/go-easy-instrumentation/internal/comment"
	"github.com/newrelic/go-easy-instrumentation/internal/util"
	"github.com/newrelic/go-easy-instrumentation/parser"
)

// LogrusNewVarName returns the variable name bound to a `logrus.New()` call,
// supporting both `name := logrus.New()` and `var name = logrus.New()`.
// Returns "" if stmt is not such an assignment, or if the LHS is `_`.
func LogrusNewVarName(stmt dst.Stmt) string {
	name, value := nameAndValue(stmt)
	if name == "" || name == "_" || !isLogrusNewCall(value) {
		return ""
	}
	return name
}

// nameAndValue extracts a single (name, value) pair from `name := value` or
// `var name = value`. Returns ("", nil) for any other shape.
func nameAndValue(stmt dst.Stmt) (string, dst.Expr) {
	switch s := stmt.(type) {
	case *dst.AssignStmt:
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return "", nil
		}
		lhs, ok := s.Lhs[0].(*dst.Ident)
		if !ok {
			return "", nil
		}
		return lhs.Name, s.Rhs[0]
	case *dst.DeclStmt:
		gen, ok := s.Decl.(*dst.GenDecl)
		if !ok || gen.Tok != token.VAR || len(gen.Specs) != 1 {
			return "", nil
		}
		spec, ok := gen.Specs[0].(*dst.ValueSpec)
		if !ok || len(spec.Names) != 1 || len(spec.Values) != 1 {
			return "", nil
		}
		return spec.Names[0].Name, spec.Values[0]
	}
	return "", nil
}

// isLogrusNewCall reports whether expr is a call to logrus.New().
func isLogrusNewCall(expr dst.Expr) bool {
	call, ok := expr.(*dst.CallExpr)
	if !ok {
		return false
	}
	fn, ok := call.Fun.(*dst.Ident)
	return ok && fn.Name == "New" && fn.Path == LogrusImportPath
}

// SetFormatterCall returns the SetFormatter CallExpr in stmt and the logger
// var name it was called on. The name is "" for the package-level
// logrus.SetFormatter; otherwise it is the receiver var (only matched against
// knownLoggers). Returns (nil, "") if stmt is not a SetFormatter call we
// recognize.
func SetFormatterCall(stmt dst.Stmt, knownLoggers map[string]dst.Stmt) (*dst.CallExpr, string) {
	exprStmt, ok := stmt.(*dst.ExprStmt)
	if !ok {
		return nil, ""
	}
	call, ok := exprStmt.X.(*dst.CallExpr)
	if !ok || util.FunctionName(call) != "SetFormatter" {
		return nil, ""
	}
	switch fn := call.Fun.(type) {
	case *dst.Ident:
		if fn.Path == LogrusImportPath {
			return call, ""
		}
	case *dst.SelectorExpr:
		if ident, ok := fn.X.(*dst.Ident); ok {
			if _, tracked := knownLoggers[ident.Name]; tracked {
				return call, ident.Name
			}
		}
	}
	return nil, ""
}

// referencesLogrus reports whether stmt references the logrus package anywhere.
func referencesLogrus(stmt dst.Stmt) bool {
	found := false
	dst.Inspect(stmt, func(n dst.Node) bool {
		if found {
			return false
		}
		if id, ok := n.(*dst.Ident); ok && id.Path == LogrusImportPath {
			found = true
			return false
		}
		return true
	})
	return found
}

// alreadyWrapped reports whether call.Args[0] is already nrlogrus.NewFormatter(...).
func alreadyWrapped(call *dst.CallExpr) bool {
	if len(call.Args) < 1 {
		return false
	}
	inner, ok := call.Args[0].(*dst.CallExpr)
	if !ok {
		return false
	}
	id, ok := inner.Fun.(*dst.Ident)
	return ok && id.Name == "NewFormatter" && id.Path == NrlogrusImportPath
}

// InstrumentLogrusHandler ensures every logrus log entry passes through
// nrlogrus.NewFormatter so it carries New Relic trace context. Four patterns
// are handled idempotently:
//
//  1. logger.SetFormatter(&logrus.JSONFormatter{}) → wrap the arg in place
//  2. logger := logrus.New() with no SetFormatter → inject a default after
//  3. logrus.SetFormatter(...) → wrap the arg in place
//  4. only logrus.Info / etc. → inject logrus.SetFormatter before the first call
func InstrumentLogrusHandler(manager *parser.InstrumentationManager, c *dstutil.Cursor) {
	decl, ok := c.Node().(*dst.FuncDecl)
	if !ok || decl.Body == nil {
		return
	}
	appVar := manager.AgentVariableName()
	body := decl.Body.List

	knownLoggers := map[string]dst.Stmt{} // var name → `x := logrus.New()` decl stmt
	injectAfter := map[dst.Stmt]string{}  // decl stmt → var name; pending default SetFormatter after it
	var firstLogrusStmt dst.Stmt          // first stmt touching logrus; pattern-4 injection anchor
	pkgSetFormatter := false              // saw `logrus.SetFormatter(...)`; suppresses pattern 4
	mutated := false                      // any wrap or inject occurred

	for _, stmt := range body {
		loggerName := LogrusNewVarName(stmt)
		call, recvName := SetFormatterCall(stmt, knownLoggers)
		switch {
		case loggerName != "": // pattern 1: `x := logrus.New()`
			knownLoggers[loggerName] = stmt
			injectAfter[stmt] = loggerName // queue default SetFormatter after this decl
		case call != nil: // pattern 2 or 3: SetFormatter — wrap arg in place
			if len(call.Args) >= 1 && !alreadyWrapped(call) {
				call.Args[0] = wrapWithNewFormatter(appVar, call.Args[0])
				mutated = true
			}
			if recvName == "" {
				pkgSetFormatter = true // pattern 3 seen — pattern 4 no longer needed
			} else {
				delete(injectAfter, knownLoggers[recvName]) // pattern 2 satisfied — cancel default injection
			}
		case !referencesLogrus(stmt):
			continue // stmt doesn't touch logrus — skip pattern-4 anchor candidacy
		}
		if firstLogrusStmt == nil {
			firstLogrusStmt = stmt
		}
	}

	injectPackage := firstLogrusStmt != nil && len(knownLoggers) == 0 && !pkgSetFormatter
	if injectPackage || len(injectAfter) > 0 {
		decl.Body.List = insertDefaults(body, injectAfter, firstLogrusStmt, injectPackage, appVar)
		mutated = true
	}
	if !mutated {
		return
	}
	comment.Debug(manager.GetDecoratorPackage(), decl, fmt.Sprintf("Instrumented logrus formatters in %s", decl.Name.Name))
	manager.AddImport(NrlogrusImportPath)
}

// insertDefaults returns a new body with default SetFormatter stmts inserted:
//   - a package-level `logrus.SetFormatter(...)` before packageAnchor, when
//     injectPackage is true (pattern 4);
//   - a `<name>.SetFormatter(...)` after each stmt still in injectAfter
//     (pattern 2 default).
func insertDefaults(body []dst.Stmt, injectAfter map[dst.Stmt]string, packageAnchor dst.Stmt, injectPackage bool, appVar string) []dst.Stmt {
	out := make([]dst.Stmt, 0, len(body)+len(injectAfter)+1)
	for _, stmt := range body {
		if injectPackage && stmt == packageAnchor {
			out = append(out, defaultSetFormatterStmt(packageSetFormatterFun(), appVar))
		}
		out = append(out, stmt)
		if name, ok := injectAfter[stmt]; ok {
			out = append(out, defaultSetFormatterStmt(loggerSetFormatterFun(name), appVar))
		}
	}
	return out
}
