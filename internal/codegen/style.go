package codegen

import (
	"fmt"

	"github.com/dave/dst"
)

// GroupStatements groups a set of statements together into
// a whitespace separated block.
func GroupStatements(stmts ...dst.Stmt) ([]dst.Stmt, error) {
	if len(stmts) < 2 {
		return nil, fmt.Errorf("must provide at least two statements to group")
	}

	final := make([]dst.Stmt, len(stmts))
	for i, stmt := range stmts {
		decs := stmt.Decorations()
		decs.Before = dst.None
		decs.After = dst.None

		if i == len(stmts)-1 {
			decs.After = dst.NewLine
		}
		final[i] = stmt
	}

	return final, nil
}
