package vm

import "fmt"

// BuiltinHandler executes a built-in opcode using the VM stack.
// It should push its result (if any) onto the stack.
// On error, return the error value and Go error to stop execution.
type BuiltinHandler func(*VM) (Value, error)

type builtinEntry struct {
	name    string
	opcode  byte
	arity   int
	handler BuiltinHandler
}

var builtinRegistry = map[byte]builtinEntry{}

// RegisterBuiltin installs a built-in handler for a given opcode.
func RegisterBuiltin(name string, opcode byte, arity int, handler BuiltinHandler) {
	if handler == nil {
		panic("nil builtin handler")
	}
	if _, exists := builtinRegistry[opcode]; exists {
		panic(fmt.Sprintf("builtin opcode 0x%X already registered", opcode))
	}
	builtinRegistry[opcode] = builtinEntry{
		name:    name,
		opcode:  opcode,
		arity:   arity,
		handler: handler,
	}
}

func lookupBuiltin(op byte) (builtinEntry, bool) {
	entry, ok := builtinRegistry[op]
	return entry, ok
}

func (vm *VM) runBuiltin(entry builtinEntry, fr *frame) (Value, error) {
	if len(vm.stack) < entry.arity {
		return vm.errorf(fr, "builtin %s expects %d args, stack has %d", entry.name, entry.arity, len(vm.stack))
	}
	val, err := entry.handler(vm)
	if err != nil {
		return vm.wrapError(fr, val, err)
	}
	return Value{}, nil
}
