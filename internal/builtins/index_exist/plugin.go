package index_exist

import (
	"github.com/xirelogy/go-flux/internal/runtime"
	"github.com/xirelogy/go-flux/internal/vm"
)

const opcode byte = 0x82

func init() {
	runtime.Register(runtime.Spec{
		Name:    "indexExist",
		Opcode:  opcode,
		Arity:   2,
		Handler: runIndexExist,
	})
}

func runIndexExist(rt *vm.VM) (vm.Value, error) {
	index := rt.Pop()
	target := rt.Pop()
	ok := vm.IndexExists(target, index)
	rt.Push(vm.Bool(ok))
	return vm.Value{}, nil
}
