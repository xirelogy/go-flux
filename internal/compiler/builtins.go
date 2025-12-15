package compiler

import (
	"fmt"

	"github.com/xirelogy/go-flux/internal/ast"
	"github.com/xirelogy/go-flux/internal/runtime"
)

func builtinName(expr ast.Expression) (string, bool) {
	if ident, ok := expr.(*ast.Identifier); ok {
		if _, exists := runtime.LookupByName(ident.Name); exists {
			return ident.Name, true
		}
	}
	return "", false
}

func (fc *funcCompiler) emitBuiltin(name string, argc int) error {
	spec, ok := runtime.LookupByName(name)
	if !ok {
		return fmt.Errorf("unknown builtin %s", name)
	}
	if argc != spec.Arity {
		return errArgs(name, spec.Arity, argc)
	}
	fc.emitByte(spec.Opcode)
	return nil
}

func errArgs(name string, want, got int) error {
	return fmt.Errorf("builtin %s expects %d args, got %d", name, want, got)
}
