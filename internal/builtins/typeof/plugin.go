package typeof

import (
	"github.com/xirelogy/go-flux/internal/runtime"
	"github.com/xirelogy/go-flux/internal/vm"
)

const opcode byte = 0x80

func init() {
	runtime.Register(runtime.Spec{
		Name:    "typeof",
		Opcode:  opcode,
		Arity:   1,
		Handler: runTypeof,
	})
}

func runTypeof(rt *vm.VM) (vm.Value, error) {
	v := rt.Pop()
	rt.Push(vm.String(vm.TypeName(v)))
	return vm.Value{}, nil
}
