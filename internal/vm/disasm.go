package vm

import (
	"fmt"
	"io"
	"sort"

	"github.com/xirelogy/go-flux/internal/bytecode"
)

// Disassemble emits assembly-style bytecode output for compiled globals.
func (vm *VM) Disassemble(w io.Writer) error {
	if vm == nil {
		return fmt.Errorf("nil VM")
	}
	if w == nil {
		return fmt.Errorf("nil writer")
	}
	names := make([]string, 0, len(vm.globals))
	funcs := make(map[string]*Function, len(vm.globals))
	for name, val := range vm.globals {
		if val.Kind != KindFunction || val.Func == nil {
			continue
		}
		names = append(names, name)
		funcs[name] = val.Func
	}
	sort.Strings(names)
	dis := bytecode.NewDisassembler(w)
	for _, name := range names {
		fn := funcs[name]
		if fn.Proto == nil && fn.Native != nil {
			dis.PrintNative(name)
			continue
		}
		if fn.Proto == nil {
			dis.PrintMissing(name)
			continue
		}
		if err := dis.DisassemblePrototype(name, fn.Proto); err != nil {
			return err
		}
	}
	return nil
}
