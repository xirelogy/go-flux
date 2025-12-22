package vm

import "reflect"

// Duplicate returns a new VM with copied globals and configuration.
// Execution state (stack/frames) is reset in the duplicate.
func (vm *VM) Duplicate() *VM {
	if vm == nil {
		return nil
	}
	dup := New()
	dup.maxStack = vm.maxStack
	dup.maxFrames = vm.maxFrames
	dup.traceHook = vm.traceHook
	dup.instLimit = vm.instLimit

	clone := newCloneState()
	dup.globals = make(map[string]Value, len(vm.globals))
	for name, val := range vm.globals {
		dup.globals[name] = clone.cloneValue(val)
	}
	return dup
}

type cloneState struct {
	arrays    map[uintptr][]Value
	objects   map[uintptr]map[string]Value
	functions map[*Function]*Function
	upvalues  map[*upvalue]*upvalue
	iterators map[*Iterator]*Iterator
}

func newCloneState() *cloneState {
	return &cloneState{
		arrays:    make(map[uintptr][]Value),
		objects:   make(map[uintptr]map[string]Value),
		functions: make(map[*Function]*Function),
		upvalues:  make(map[*upvalue]*upvalue),
		iterators: make(map[*Iterator]*Iterator),
	}
}

func (cs *cloneState) cloneValue(v Value) Value {
	switch v.Kind {
	case KindArray:
		if v.Arr == nil {
			return Value{Kind: KindArray, ReadOnly: v.ReadOnly}
		}
		key := sliceKey(v.Arr)
		if key != 0 {
			if arr, ok := cs.arrays[key]; ok {
				return Value{Kind: KindArray, Arr: arr, ReadOnly: v.ReadOnly}
			}
		}
		out := make([]Value, len(v.Arr))
		if key != 0 {
			cs.arrays[key] = out
		}
		for i := range v.Arr {
			out[i] = cs.cloneValue(v.Arr[i])
		}
		return Value{Kind: KindArray, Arr: out, ReadOnly: v.ReadOnly}
	case KindObject:
		if v.Obj == nil {
			return Value{Kind: KindObject, ReadOnly: v.ReadOnly}
		}
		key := mapKey(v.Obj)
		if key != 0 {
			if obj, ok := cs.objects[key]; ok {
				return Value{Kind: KindObject, Obj: obj, ReadOnly: v.ReadOnly}
			}
		}
		out := make(map[string]Value, len(v.Obj))
		if key != 0 {
			cs.objects[key] = out
		}
		for k, val := range v.Obj {
			out[k] = cs.cloneValue(val)
		}
		return Value{Kind: KindObject, Obj: out, ReadOnly: v.ReadOnly}
	case KindFunction:
		if v.Func == nil {
			return v
		}
		return Value{Kind: KindFunction, Func: cs.cloneFunction(v.Func), ReadOnly: v.ReadOnly}
	case KindIterator:
		if v.It == nil {
			return v
		}
		return Value{Kind: KindIterator, It: cs.cloneIterator(v.It), ReadOnly: v.ReadOnly}
	default:
		return v
	}
}

func (cs *cloneState) cloneFunction(fn *Function) *Function {
	if fn == nil {
		return nil
	}
	if cloned, ok := cs.functions[fn]; ok {
		return cloned
	}
	out := &Function{
		Proto:  fn.Proto,
		Native: fn.Native,
		Name:   fn.Name,
		Source: fn.Source,
	}
	cs.functions[fn] = out
	if fn.Upvalues != nil {
		out.Upvalues = make([]*upvalue, len(fn.Upvalues))
		for i, uv := range fn.Upvalues {
			out.Upvalues[i] = cs.cloneUpvalue(uv)
		}
	}
	return out
}

func (cs *cloneState) cloneUpvalue(uv *upvalue) *upvalue {
	if uv == nil {
		return nil
	}
	if cloned, ok := cs.upvalues[uv]; ok {
		return cloned
	}
	out := &upvalue{}
	cs.upvalues[uv] = out
	if uv.location != nil {
		out.closed = cs.cloneValue(*uv.location)
	} else {
		out.closed = cs.cloneValue(uv.closed)
	}
	return out
}

func (cs *cloneState) cloneIterator(it *Iterator) *Iterator {
	if it == nil {
		return nil
	}
	if cloned, ok := cs.iterators[it]; ok {
		return cloned
	}
	out := &Iterator{index: it.index}
	cs.iterators[it] = out
	if it.arr != nil {
		arr := cs.cloneValue(Value{Kind: KindArray, Arr: it.arr})
		out.arr = arr.Arr
	}
	if it.obj != nil {
		obj := cs.cloneValue(Value{Kind: KindObject, Obj: it.obj})
		out.obj = obj.Obj
		if it.keys != nil {
			out.keys = make([]string, len(it.keys))
			copy(out.keys, it.keys)
		}
	}
	return out
}

func sliceKey(arr []Value) uintptr {
	if arr == nil {
		return 0
	}
	return reflect.ValueOf(arr).Pointer()
}

func mapKey(obj map[string]Value) uintptr {
	if obj == nil {
		return 0
	}
	return reflect.ValueOf(obj).Pointer()
}
