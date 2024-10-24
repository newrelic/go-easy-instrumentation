package codegen

import "github.com/dave/dst"

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
