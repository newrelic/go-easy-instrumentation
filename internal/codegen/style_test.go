package codegen

import (
	"fmt"
	"testing"

	"github.com/dave/dst"
)

func TestGroupStatements(t *testing.T) {
	type args struct {
		stmts []dst.Stmt
	}
	tests := []struct {
		name string
		args args
		err  error
	}{
		{
			name: "empty input",
			args: args{
				stmts: []dst.Stmt{},
			},
			err: fmt.Errorf("must provide at least two statements to group"),
		},
		{
			name: "three statment group",
			args: args{
				stmts: []dst.Stmt{
					TxnFromContext("txn", dst.NewIdent("ctx")),
					DeferSegment("mysegment", "txn"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := GroupStatements(tt.args.stmts...)
			switch len(tt.args.stmts) {

			case 0, 1:
				if got != nil {
					t.Errorf("when no statements passed, nil should be returned; got %v", got)
				}
				if gotErr == nil {
					t.Errorf("when no statements passed, an error should be returned; got %v", gotErr)
				}
				if gotErr.Error() != tt.err.Error() {
					t.Errorf("expected error to be %v; got %v", tt.err, gotErr)
				}

			default:
				if len(got) != len(tt.args.stmts) {
					t.Errorf("the output should be the same length as the input; got %d, want %d", len(got), len(tt.args.stmts))
					t.FailNow()
				}
				for i, stmt := range got {
					decs := stmt.Decorations()
					if decs.Before != dst.None {
						t.Errorf("expected Before decoration to be EmptyLine; got %#+v: %v", decs.Before, stmt)
					}
					if i == len(got)-1 {
						if decs.After != dst.NewLine {
							t.Errorf("expected After decoration of the last element to be NewLine; got %s", decs.After)
						}
					} else {
						if decs.After != dst.None {
							t.Errorf("expected After decoration of all but the final element of the group to be None; After spacing for statement %d: %s", i, decs.After)
						}
					}
				}
			}
		})
	}
}
