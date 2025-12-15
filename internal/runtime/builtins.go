package runtime

import (
	"fmt"

	"github.com/xirelogy/go-flux/internal/vm"
)

// Spec describes a built-in function opcode and handler.
type Spec struct {
	Name    string
	Opcode  byte
	Arity   int
	Handler vm.BuiltinHandler
}

var (
	byName   = map[string]Spec{}
	byOpcode = map[byte]Spec{}
)

// Register installs a built-in for both lookup tables and the VM.
func Register(spec Spec) {
	if spec.Handler == nil {
		panic(fmt.Sprintf("builtin %s has nil handler", spec.Name))
	}
	if _, exists := byName[spec.Name]; exists {
		panic(fmt.Sprintf("builtin %s already registered", spec.Name))
	}
	if _, exists := byOpcode[spec.Opcode]; exists {
		panic(fmt.Sprintf("builtin opcode 0x%X already registered", spec.Opcode))
	}
	byName[spec.Name] = spec
	byOpcode[spec.Opcode] = spec
	vm.RegisterBuiltin(spec.Name, spec.Opcode, spec.Arity, spec.Handler)
}

// LookupByName finds a builtin by its script-visible name.
func LookupByName(name string) (Spec, bool) {
	spec, ok := byName[name]
	return spec, ok
}

// LookupByOpcode finds a builtin by opcode.
func LookupByOpcode(op byte) (Spec, bool) {
	spec, ok := byOpcode[op]
	return spec, ok
}

// All returns all registered builtins.
func All() []Spec {
	out := make([]Spec, 0, len(byName))
	for _, spec := range byName {
		out = append(out, spec)
	}
	return out
}
