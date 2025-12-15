package errorbuiltin

import (
	"github.com/xirelogy/go-flux/internal/runtime"
	"github.com/xirelogy/go-flux/internal/vm"
)

const opcode byte = 0x81

func init() {
	runtime.Register(runtime.Spec{
		Name:    "error",
		Opcode:  opcode,
		Arity:   1,
		Handler: runError,
	})
}

func runError(rt *vm.VM) (vm.Value, error) {
	v := rt.Pop()
	if v.Kind != vm.KindString {
		return vm.RuntimeErrorf(rt, "error expects string")
	}
	rt.Push(vm.ErrorVal(v.Str))
	return vm.Value{}, nil
}
