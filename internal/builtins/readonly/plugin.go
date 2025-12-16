package readonly

import (
	"github.com/xirelogy/go-flux/internal/runtime"
	"github.com/xirelogy/go-flux/internal/vm"
)

const opcode byte = 0x85

func init() {
	runtime.Register(runtime.Spec{
		Name:    "readonly",
		Opcode:  opcode,
		Arity:   1,
		Handler: runReadonly,
	})
}

func runReadonly(rt *vm.VM) (vm.Value, error) {
	v := rt.Pop()
	rt.Push(vm.Bool(v.ReadOnly))
	return vm.Value{}, nil
}
