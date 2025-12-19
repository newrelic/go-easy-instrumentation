package errorcache

import (
	"slices"

	"github.com/dave/dst"
)

type ErrorCache struct {
	errorexpr      dst.Expr
	errorstmt      dst.Stmt
	ExistingErrors []*dst.Ident
}

func (ec *ErrorCache) Load(errorexpr dst.Expr, errorstmt dst.Stmt) {
	ec.errorexpr = errorexpr
	ec.errorstmt = errorstmt

}

func (ec *ErrorCache) LoadExistingErrors(err *dst.Ident) {
	ec.ExistingErrors = append(ec.ExistingErrors, err)
}

func (ec *ErrorCache) IsExistingError(err dst.Expr) bool {
	// can we check if an Expr is part of dst.Ident existing?
	if ident, ok := err.(*dst.Ident); ok {
		if slices.Contains(ec.ExistingErrors, ident) {
			return true
		}
	}

	return false
}

func (ec *ErrorCache) GetExpression() dst.Expr {
	return ec.errorexpr
}
func (ec *ErrorCache) GetStatement() dst.Stmt {
	return ec.errorstmt
}

func (ec *ErrorCache) Clear() {
	ec.errorexpr = nil
	ec.errorstmt = nil
}

// Testing Functions
func (ec *ErrorCache) ExtractExistingErrors() []string {
	var errors []string
	for _, errIdent := range ec.ExistingErrors {
		errors = append(errors, errIdent.Name)
	}
	return errors
}
