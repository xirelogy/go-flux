package index_read

import (
	"github.com/xirelogy/go-flux/internal/runtime"
	"github.com/xirelogy/go-flux/internal/vm"
)

const opcode byte = 0x83

func init() {
	runtime.Register(runtime.Spec{
		Name:    "indexRead",
		Opcode:  opcode,
		Arity:   3,
		Handler: runIndexRead,
	})
}

func runIndexRead(rt *vm.VM) (vm.Value, error) {
	def := rt.Pop()
	index := rt.Pop()
	target := rt.Pop()
	val, err := vm.IndexGet(target, index)
	if err != nil {
		rt.Push(def)
		return vm.Value{}, nil
	}
	rt.Push(val)
	return vm.Value{}, nil
}
