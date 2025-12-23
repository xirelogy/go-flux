package bytecode

import "fmt"

// BuiltinInfo describes a registered builtin opcode.
type BuiltinInfo struct {
	Name   string
	Opcode byte
	Arity  int
}

var builtinInfo = map[byte]BuiltinInfo{}

// RegisterBuiltinInfo registers builtin opcode metadata for diagnostics/disassembly.
func RegisterBuiltinInfo(name string, opcode byte, arity int) {
	if name == "" {
		name = fmt.Sprintf("0x%02X", opcode)
	}
	if _, exists := builtinInfo[opcode]; exists {
		panic(fmt.Sprintf("builtin opcode 0x%X already registered", opcode))
	}
	builtinInfo[opcode] = BuiltinInfo{Name: name, Opcode: opcode, Arity: arity}
}

// LookupBuiltinInfo returns builtin metadata if registered.
func LookupBuiltinInfo(opcode byte) (BuiltinInfo, bool) {
	info, ok := builtinInfo[opcode]
	return info, ok
}
