package codegen

import (
	"slices"

	"github.com/dave/dst"
)

func WrapStatements(first, wrapped, last dst.Stmt) {
	firstDecs := first.Decorations()
	wrappedDecs := wrapped.Decorations()
	lastDecs := last.Decorations()

	firstDecs.Before = wrappedDecs.Before
	firstDecs.Start = wrappedDecs.Start

	lastDecs.After = wrappedDecs.After
	lastDecs.End = wrappedDecs.End

	wrappedDecs.Before = dst.None
	wrappedDecs.Start.Clear()
	wrappedDecs.After = dst.None
	wrappedDecs.End = nil
}

func PrependStatementToFunctionDecl(fn *dst.FuncDecl, stmt dst.Stmt) {
	if fn.Body == nil {
		return
	}

	fn.Body.List = slices.Insert(fn.Body.List, 0, stmt)
}

func PrependStatementToFunctionLit(fn *dst.FuncLit, stmt dst.Stmt) {
	if fn.Body == nil {
		return
	}

	fn.Body.List = slices.Insert(fn.Body.List, 0, stmt)
}

// CreateStatementBlock modifies the formatting of a set of statements to
// all be on separate lines, without any additional spacing between them.
//
// White space is always added after the block.
//
// If spacingBefore == true, an emptyline is added before the block.
func CreateStatementBlock(spacingBefore bool, stmts ...dst.Stmt) {
	for i, stmt := range stmts {
		stmtDecs := stmt.Decorations()
		stmtDecs.Before = dst.NewLine
		stmtDecs.After = dst.NewLine

		if i == len(stmts)-1 {
			stmtDecs.After = dst.EmptyLine
		}
		if spacingBefore && i == 0 {
			stmtDecs.Before = dst.EmptyLine
		}
	}
}
