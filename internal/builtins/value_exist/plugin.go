package value_exist

import (
	"github.com/xirelogy/go-flux/internal/runtime"
	"github.com/xirelogy/go-flux/internal/vm"
)

const opcode byte = 0x84

func init() {
	runtime.Register(runtime.Spec{
		Name:    "valueExist",
		Opcode:  opcode,
		Arity:   2,
		Handler: runValueExist,
	})
}

func runValueExist(rt *vm.VM) (vm.Value, error) {
	val := rt.Pop()
	arr := rt.Pop()
	ok := vm.ValueExists(arr, val)
	rt.Push(vm.Bool(ok))
	return vm.Value{}, nil
}
