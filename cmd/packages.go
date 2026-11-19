package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dave/dst/decorator"
	"golang.org/x/tools/go/packages"
)

// maxReportedPackages caps how many package names a load warning lists, so a
// broken build does not bury the rest of the output.
const maxReportedPackages = 5

// packageLoadIssue describes a package that did not load cleanly.
type packageLoadIssue struct {
	// ID is the package's import path or package ID.
	ID string

	// IllTyped reports that the package failed to type-check.
	IllTyped bool

	// Errors are the load or type-check errors recorded for the package.
	Errors []string
}

// packageLoadIssues returns one entry per package that did not load cleanly,
// sorted by package ID.
//
// It walks dependencies as well as the packages named on the command line,
// because incomplete type information for a dependency is what actually breaks
// instrumentation: the pattern matching in parser/ is type-driven, so a standard
// library package that failed to load makes calls through it invisible.
func packageLoadIssues(pkgs []*decorator.Package) []packageLoadIssue {
	roots := make([]*packages.Package, 0, len(pkgs))
	for _, pkg := range pkgs {
		if pkg != nil && pkg.Package != nil {
			roots = append(roots, pkg.Package)
		}
	}

	issues := []packageLoadIssue{}
	seen := map[string]bool{}

	packages.Visit(roots, nil, func(pkg *packages.Package) {
		if len(pkg.Errors) == 0 && !pkg.IllTyped {
			return
		}
		if seen[pkg.ID] {
			return
		}
		seen[pkg.ID] = true

		messages := make([]string, 0, len(pkg.Errors))
		for _, err := range pkg.Errors {
			messages = append(messages, err.Error())
		}

		issues = append(issues, packageLoadIssue{
			ID:       pkg.ID,
			IllTyped: pkg.IllTyped,
			Errors:   messages,
		})
	})

	sort.Slice(issues, func(i, j int) bool { return issues[i].ID < issues[j].ID })

	return issues
}

// checkPackageLoad inspects a load for problems, returning a warning to show the
// user and an error when none of the packages named on the command line loaded.
//
// The warning is returned rather than printed because the TUI renders
// concurrently with package loading, and writing to stdout from the worker
// goroutine would corrupt the display. Callers print it when they have the
// terminal to themselves.
//
// Loading problems used to pass unnoticed: nothing inspected Errors or IllTyped,
// so a build failure or a toolchain mismatch produced a diff that looked
// complete but silently omitted instrumentation. Warning is the default rather
// than failing, because applications with imperfect loads can still be
// instrumented usefully, and refusing them would reject code this tool handles
// today.
func checkPackageLoad(pkgs []*decorator.Package) (warning string, err error) {
	issues := packageLoadIssues(pkgs)
	if len(issues) == 0 {
		return "", nil
	}

	if rootsAllFailed(pkgs) {
		return "", fmt.Errorf("no packages could be loaded for instrumentation:\n%s", indentedList(issues))
	}

	return fmt.Sprintf("\nWARNING: %d package(s) did not load cleanly. Instrumentation may be incomplete or missing where they are used.\n%s\n%s\n",
		len(issues),
		indentedList(issues),
		"This usually means the application does not build, a dependency is missing, or the Go toolchain and golang.org/x/tools versions are mismatched.",
	), nil
}

// rootsAllFailed reports whether every package named on the command line failed
// to load. A single clean root is enough to produce useful instrumentation.
func rootsAllFailed(pkgs []*decorator.Package) bool {
	if len(pkgs) == 0 {
		return false
	}

	for _, pkg := range pkgs {
		if pkg == nil || pkg.Package == nil {
			continue
		}
		if len(pkg.Errors) == 0 && !pkg.IllTyped {
			return false
		}
	}

	return true
}

// indentedList renders issues for console output, truncating the list so a
// badly broken build does not flood the terminal.
func indentedList(issues []packageLoadIssue) string {
	var b strings.Builder

	for i, issue := range issues {
		if i == maxReportedPackages {
			fmt.Fprintf(&b, "  ...and %d more\n", len(issues)-maxReportedPackages)
			break
		}

		fmt.Fprintf(&b, "  %s\n", issue.ID)
		if len(issue.Errors) > 0 {
			// The first error is almost always the useful one; the rest tend to
			// be knock-on type-check failures.
			fmt.Fprintf(&b, "    %s\n", issue.Errors[0])
		} else if issue.IllTyped {
			fmt.Fprintf(&b, "    %s\n", "package failed to type-check")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}
