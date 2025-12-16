package vm

import (
	"fmt"
	"strconv"

	"github.com/xirelogy/go-flux/internal/bytecode"
)

// NativeFunc represents a host-provided callable.
type NativeFunc func(*VM, []Value) (Value, error)

// Function wraps either a compiled prototype or a native handler.
type Function struct {
	Proto    *bytecode.Prototype
	Upvalues []*upvalue
	Native   NativeFunc
	Name     string
	Source   string
}

type frame struct {
	fn     *Function
	ip     int
	locals []Value
	base   int
	lastOp int
}

// VM is a simple stack-based bytecode interpreter.
type VM struct {
	stack        []Value
	frames       []frame
	globals      map[string]Value
	openUpvalues []*upvalue
	maxStack     int
	maxFrames    int
	traceHook    TraceHook
	instLimit    int
	instCount    int
}

const (
	defaultMaxStack  = 1024
	defaultMaxFrames = 256
)

// New constructs an empty VM instance.
func New() *VM {
	return &VM{
		stack:        make([]Value, 0, 256),
		frames:       make([]frame, 0, 16),
		globals:      make(map[string]Value),
		openUpvalues: make([]*upvalue, 0),
		maxStack:     defaultMaxStack,
		maxFrames:    defaultMaxFrames,
	}
}

// SetTraceHook registers a callback for instruction-level tracing.
func (vm *VM) SetTraceHook(h TraceHook) {
	vm.traceHook = h
}

// SetInstructionLimit caps the number of instructions executed per Run/Call (0 for unlimited).
func (vm *VM) SetInstructionLimit(limit int) {
	if limit < 0 {
		limit = 0
	}
	vm.instLimit = limit
}

// ResetState clears transient execution state (stack, frames, open upvalues).
func (vm *VM) ResetState() {
	vm.stack = vm.stack[:0]
	vm.frames = vm.frames[:0]
	vm.openUpvalues = vm.openUpvalues[:0]
	vm.instCount = 0
}

// LoadModule registers compiled functions as globals for invocation.
func (vm *VM) LoadModule(mod *bytecode.Module) {
	if mod == nil {
		return
	}
	for name, proto := range mod.Functions {
		vm.globals[name] = Value{
			Kind: KindFunction,
			Func: &Function{
				Proto:    proto,
				Name:     name,
				Source:   proto.Source,
				Upvalues: make([]*upvalue, len(proto.Upvalues)),
			},
		}
	}
}

// DefineGlobal binds a value into the global environment.
func (vm *VM) DefineGlobal(name string, v Value) {
	vm.globals[name] = v
}

// Call invokes a global function by name.
func (vm *VM) Call(name string, args []Value) (Value, error) {
	val, ok := vm.globals[name]
	if !ok {
		return vm.errorf(nil, "global %s not found", name)
	}
	fn, err := toFunction(val)
	if err != nil {
		return vm.wrapError(nil, ErrorVal(err.Error()), err)
	}
	return vm.Run(fn, args)
}

// Run executes the given function with arguments on a fresh stack.
func (vm *VM) Run(fn *Function, args []Value) (Value, error) {
	vm.ResetState()
	vm.instCount = 0
	if fn == nil {
		return vm.errorf(nil, "invalid function")
	}
	if fn.Native != nil {
		val, err := fn.Native(vm, args)
		if err != nil {
			return vm.wrapError(nil, ErrorVal(err.Error()), err)
		}
		return val, nil
	}
	if _, err := vm.pushFrame(fn); err != nil {
		return vm.errorf(nil, "%s", err.Error())
	}
	fr := vm.currentFrame()
	for i := 0; i < len(args) && i < len(fr.locals); i++ {
		fr.locals[i] = args[i]
	}

	for len(vm.frames) > 0 {
		fr = vm.currentFrame()
		fr.lastOp = fr.ip
		if fr.fn.Proto == nil || fr.fn.Proto.Chunk == nil {
			return vm.errorf(fr, "function missing prototype")
		}
		code := fr.fn.Proto.Chunk.Code
		if fr.ip >= len(code) {
			ret, done := vm.finishFrame(Null())
			if done {
				return ret, nil
			}
			continue
		}
		op := code[fr.ip]
		fr.ip++
		vm.instCount++
		if vm.instLimit > 0 && vm.instCount > vm.instLimit {
			return vm.errorf(fr, "instruction limit exceeded")
		}
		vm.trace(fr, op)
		if entry, ok := lookupBuiltin(op); ok {
			if val, err := vm.runBuiltin(entry, fr); err != nil {
				return val, err
			}
			continue
		}
		switch op {
		case bytecode.OP_NOP, bytecode.OP_DEBUG:
			// no-op
		case bytecode.OP_CONST:
			idx := vm.readU16(fr)
			vm.push(constToValue(fr.fn.Proto.Chunk.Consts[idx]))
		case bytecode.OP_NULL:
			vm.push(Null())
		case bytecode.OP_TRUE:
			vm.push(Bool(true))
		case bytecode.OP_FALSE:
			vm.push(Bool(false))
		case bytecode.OP_POP:
			vm.pop()
		case bytecode.OP_ADD, bytecode.OP_SUB, bytecode.OP_MUL, bytecode.OP_DIV,
			bytecode.OP_EQ, bytecode.OP_NEQ, bytecode.OP_LT, bytecode.OP_LTE, bytecode.OP_GT, bytecode.OP_GTE:
			b := vm.pop()
			a := vm.pop()
			res, err := binaryOp(op, a, b)
			if err != nil {
				return vm.wrapError(fr, ErrorVal(err.Error()), err)
			}
			vm.push(res)
		case bytecode.OP_NEG:
			v := vm.pop()
			if v.Kind != KindNumber {
				return vm.errorf(fr, "operand must be number")
			}
			vm.push(Number(-v.Num))
		case bytecode.OP_NOT:
			v := vm.pop()
			vm.push(Bool(!Truthy(v)))
		case bytecode.OP_AND:
			b := vm.pop()
			a := vm.pop()
			vm.push(Bool(Truthy(a) && Truthy(b)))
		case bytecode.OP_OR:
			b := vm.pop()
			a := vm.pop()
			vm.push(Bool(Truthy(a) || Truthy(b)))
		case bytecode.OP_GET_LOCAL:
			slot := vm.readU8(fr)
			if int(slot) >= len(fr.locals) {
				return vm.errorf(fr, "local slot out of range")
			}
			vm.push(fr.locals[int(slot)])
		case bytecode.OP_SET_LOCAL:
			slot := vm.readU8(fr)
			if int(slot) >= len(fr.locals) {
				return vm.errorf(fr, "local slot out of range")
			}
			val := vm.pop()
			fr.locals[int(slot)] = val
		case bytecode.OP_GET_UPVALUE:
			slot := vm.readU8(fr)
			if int(slot) >= len(fr.fn.Upvalues) {
				return vm.errorf(fr, "upvalue slot out of range")
			}
			vm.push(fr.fn.Upvalues[int(slot)].get())
		case bytecode.OP_SET_UPVALUE:
			slot := vm.readU8(fr)
			if int(slot) >= len(fr.fn.Upvalues) {
				return vm.errorf(fr, "upvalue slot out of range")
			}
			val := vm.pop()
			fr.fn.Upvalues[int(slot)].set(val)
		case bytecode.OP_GET_GLOBAL:
			idx := vm.readU16(fr)
			name, ok := fr.fn.Proto.Chunk.Consts[idx].(string)
			if !ok {
				return vm.errorf(fr, "global name constant is not string")
			}
			v, exists := vm.globals[name]
			if !exists {
				return vm.errorf(fr, "global %s not found", name)
			}
			vm.push(v)
		case bytecode.OP_SET_GLOBAL:
			idx := vm.readU16(fr)
			name, ok := fr.fn.Proto.Chunk.Consts[idx].(string)
			if !ok {
				return vm.errorf(fr, "global name constant is not string")
			}
			val := vm.pop()
			vm.globals[name] = val
		case bytecode.OP_DEFINE_GLOBAL:
			idx := vm.readU16(fr)
			name, ok := fr.fn.Proto.Chunk.Consts[idx].(string)
			if !ok {
				return vm.errorf(fr, "global name constant is not string")
			}
			val := vm.pop()
			vm.globals[name] = val
		case bytecode.OP_ARRAY:
			count := vm.readU16(fr)
			elements := make([]Value, count)
			for i := count - 1; i >= 0; i-- {
				elements[i] = vm.pop()
			}
			vm.push(Array(elements))
		case bytecode.OP_OBJECT:
			count := vm.readU16(fr)
			obj := make(map[string]Value, count)
			for i := count - 1; i >= 0; i-- {
				val := vm.pop()
				key := vm.pop()
				keyStr, err := expectKeyString(key)
				if err != nil {
					return vm.wrapError(fr, ErrorVal(err.Error()), err)
				}
				obj[keyStr] = val
			}
			vm.push(Object(obj))
		case bytecode.OP_RANGE:
			end := vm.pop()
			start := vm.pop()
			startIdx, err := expectIndex(start, -1)
			if err != nil {
				return vm.wrapError(fr, ErrorVal(err.Error()), err)
			}
			endIdx, err := expectIndex(end, -1)
			if err != nil {
				return vm.wrapError(fr, ErrorVal(err.Error()), err)
			}
			arr := buildRange(startIdx, endIdx)
			vm.push(Array(arr))
		case bytecode.OP_INDEX_GET:
			index := vm.pop()
			target := vm.pop()
			val, err := indexGet(target, index)
			if err != nil {
				return vm.wrapError(fr, ErrorVal(err.Error()), err)
			}
			vm.push(val)
		case bytecode.OP_INDEX_SET:
			val := vm.pop()
			index := vm.pop()
			target := vm.pop()
			if err := indexSet(target, index, val); err != nil {
				return vm.wrapError(fr, ErrorVal(err.Error()), err)
			}
		case bytecode.OP_GET_PROP:
			idx := vm.readU16(fr)
			prop, ok := fr.fn.Proto.Chunk.Consts[idx].(string)
			if !ok {
				return vm.errorf(fr, "property name constant is not string")
			}
			obj := vm.pop()
			if obj.Kind != KindObject || obj.Obj == nil {
				return vm.errorf(fr, "property access on non-object")
			}
			val, ok := obj.Obj[prop]
			if !ok {
				return vm.errorf(fr, "missing property %s", prop)
			}
			vm.push(val)
		case bytecode.OP_SET_PROP:
			idx := vm.readU16(fr)
			prop, ok := fr.fn.Proto.Chunk.Consts[idx].(string)
			if !ok {
				return vm.errorf(fr, "property name constant is not string")
			}
			val := vm.pop()
			obj := vm.pop()
			if obj.Kind != KindObject || obj.Obj == nil {
				return vm.errorf(fr, "property set on non-object")
			}
			if obj.ReadOnly {
				return vm.errorf(fr, "cannot modify read-only value")
			}
			obj.Obj[prop] = val
		case bytecode.OP_JUMP:
			off := vm.readU16(fr)
			fr.ip = off
		case bytecode.OP_JUMP_IF_FALSE:
			off := vm.readU16(fr)
			cond := vm.pop()
			if !Truthy(cond) {
				fr.ip = off
			}
		case bytecode.OP_JUMP_IF_TRUE:
			off := vm.readU16(fr)
			cond := vm.pop()
			if Truthy(cond) {
				fr.ip = off
			}
		case bytecode.OP_CALL:
			argc := int(vm.readU8(fr))
			if len(vm.stack) < argc+1 {
				return vm.errorf(fr, "stack underflow on call: argc=%d stack=%d", argc, len(vm.stack))
			}
			args := make([]Value, argc)
			for i := argc - 1; i >= 0; i-- {
				if i >= len(args) {
					return vm.errorf(fr, "call arg index overflow i=%d argc=%d len=%d", i, argc, len(args))
				}
				args[i] = vm.pop()
			}
			callee := vm.pop()
			fn, err := toFunction(callee)
			if err != nil {
				return vm.wrapError(fr, ErrorVal(err.Error()), err)
			}
			if fn.Native != nil {
				res, err := fn.Native(vm, args)
				if err != nil {
					return vm.wrapError(fr, ErrorVal(err.Error()), err)
				}
				vm.push(res)
			} else {
				if _, err := vm.pushFrame(fn); err != nil {
					return vm.wrapError(fr, ErrorVal(err.Error()), err)
				}
				newFr := vm.currentFrame()
				for i := 0; i < len(args) && i < len(newFr.locals); i++ {
					newFr.locals[i] = args[i]
				}
			}
		case bytecode.OP_RETURN:
			ret := Null()
			if len(vm.stack) > fr.base {
				ret = vm.pop()
			}
			result, done := vm.finishFrame(ret)
			if done {
				return result, nil
			}
		case bytecode.OP_CLOSURE:
			idx := vm.readU16(fr)
			upcount := int(vm.readU8(fr))
			proto, ok := fr.fn.Proto.Chunk.Consts[idx].(*bytecode.Prototype)
			if !ok {
				return vm.errorf(fr, "closure constant is not prototype")
			}
			closure := &Function{
				Proto:    proto,
				Name:     proto.Name,
				Source:   proto.Source,
				Upvalues: make([]*upvalue, upcount),
			}
			for i := 0; i < upcount; i++ {
				isLocal := vm.readU8(fr)
				slot := vm.readU8(fr)
				if isLocal == 1 {
					if int(slot) >= len(fr.locals) {
						return vm.errorf(fr, "upvalue local slot out of range")
					}
					closure.Upvalues[i] = vm.captureUpvalue(&fr.locals[int(slot)])
				} else {
					if int(slot) >= len(fr.fn.Upvalues) {
						return vm.errorf(fr, "upvalue index out of range")
					}
					closure.Upvalues[i] = fr.fn.Upvalues[int(slot)]
				}
			}
			vm.push(Value{Kind: KindFunction, Func: closure})
		case bytecode.OP_ITER_PREP:
			iterable := vm.pop()
			it, err := toIterator(iterable)
			if err != nil {
				return vm.wrapError(fr, ErrorVal(err.Error()), err)
			}
			vm.push(IteratorVal(it))
		case bytecode.OP_ITER_NEXT:
			jump := vm.readU16(fr)
			iter := vm.peek()
			if iter.Kind != KindIterator || iter.It == nil {
				return vm.errorf(fr, "not an iterator")
			}
			key, val, ok := iter.It.Next()
			if !ok {
				fr.ip = jump
				continue
			}
			vm.push(String(key))
			vm.push(val)
		default:
			return vm.errorf(fr, "unknown opcode %d", op)
		}
	}

	return Null(), nil
}

func (vm *VM) pushFrame(fn *Function) (*frame, error) {
	if fn == nil || fn.Proto == nil {
		return nil, fmt.Errorf("invalid function")
	}
	if len(vm.frames) >= vm.maxFrames {
		return nil, fmt.Errorf("call stack overflow")
	}
	locals := make([]Value, fn.maxLocals())
	vm.frames = append(vm.frames, frame{
		fn:     fn,
		ip:     0,
		locals: locals,
		base:   len(vm.stack),
		lastOp: -1,
	})
	return &vm.frames[len(vm.frames)-1], nil
}

func (vm *VM) finishFrame(ret Value) (Value, bool) {
	fr := vm.currentFrame()
	vm.closeUpvalues(fr.locals)
	vm.frames = vm.frames[:len(vm.frames)-1]
	vm.stack = vm.stack[:fr.base]
	if len(vm.frames) == 0 {
		return ret, true
	}
	vm.push(ret)
	return ret, false
}

func (vm *VM) currentFrame() *frame {
	return &vm.frames[len(vm.frames)-1]
}

func (vm *VM) push(v Value) {
	vm.stack = append(vm.stack, v)
}

func (vm *VM) pop() Value {
	if len(vm.stack) == 0 {
		return Null()
	}
	v := vm.stack[len(vm.stack)-1]
	vm.stack = vm.stack[:len(vm.stack)-1]
	return v
}

func (vm *VM) peek() Value {
	if len(vm.stack) == 0 {
		return Null()
	}
	return vm.stack[len(vm.stack)-1]
}

func (vm *VM) readU16(fr *frame) int {
	hi := fr.fn.Proto.Chunk.Code[fr.ip]
	lo := fr.fn.Proto.Chunk.Code[fr.ip+1]
	fr.ip += 2
	return int(hi)<<8 | int(lo)
}

func (vm *VM) readU8(fr *frame) byte {
	b := fr.fn.Proto.Chunk.Code[fr.ip]
	fr.ip++
	return b
}

func (vm *VM) captureUpvalue(slot *Value) *upvalue {
	for _, uv := range vm.openUpvalues {
		if uv.location == slot {
			return uv
		}
	}
	uv := newUpvalue(slot)
	vm.openUpvalues = append(vm.openUpvalues, uv)
	return uv
}

func (vm *VM) closeUpvalues(locals []Value) {
	if len(locals) == 0 {
		return
	}
	filtered := vm.openUpvalues[:0]
	for _, uv := range vm.openUpvalues {
		if containsSlot(locals, uv.location) {
			uv.close()
			continue
		}
		filtered = append(filtered, uv)
	}
	vm.openUpvalues = filtered
}

func containsSlot(locals []Value, slot *Value) bool {
	for i := range locals {
		if &locals[i] == slot {
			return true
		}
	}
	return false
}

func constToValue(v interface{}) Value {
	switch val := v.(type) {
	case nil:
		return Null()
	case bool:
		return Bool(val)
	case float64:
		return Number(val)
	case string:
		return String(val)
	case *bytecode.Prototype:
		return Value{Kind: KindFunction, Func: &Function{
			Proto:    val,
			Name:     val.Name,
			Source:   val.Source,
			Upvalues: make([]*upvalue, len(val.Upvalues)),
		}}
	default:
		return Null()
	}
}

func binaryOp(op byte, a, b Value) (Value, error) {
	switch op {
	case bytecode.OP_ADD, bytecode.OP_SUB, bytecode.OP_MUL, bytecode.OP_DIV:
		if a.Kind != KindNumber || b.Kind != KindNumber {
			return Null(), fmt.Errorf("operands must be numbers")
		}
		switch op {
		case bytecode.OP_ADD:
			return Number(a.Num + b.Num), nil
		case bytecode.OP_SUB:
			return Number(a.Num - b.Num), nil
		case bytecode.OP_MUL:
			return Number(a.Num * b.Num), nil
		case bytecode.OP_DIV:
			return Number(a.Num / b.Num), nil
		}
	case bytecode.OP_EQ:
		return Bool(Equal(a, b)), nil
	case bytecode.OP_NEQ:
		return Bool(!Equal(a, b)), nil
	case bytecode.OP_LT, bytecode.OP_LTE, bytecode.OP_GT, bytecode.OP_GTE:
		if a.Kind != KindNumber || b.Kind != KindNumber {
			return Null(), fmt.Errorf("operands must be numbers")
		}
		switch op {
		case bytecode.OP_LT:
			return Bool(a.Num < b.Num), nil
		case bytecode.OP_LTE:
			return Bool(a.Num <= b.Num), nil
		case bytecode.OP_GT:
			return Bool(a.Num > b.Num), nil
		case bytecode.OP_GTE:
			return Bool(a.Num >= b.Num), nil
		}
	}
	return Null(), fmt.Errorf("unsupported op")
}

func (fn *Function) maxLocals() int {
	if fn.Proto == nil {
		return 0
	}
	if fn.Proto.MaxLocals > 0 {
		return fn.Proto.MaxLocals
	}
	// fall back to params count if MaxLocals is unset
	return fn.Proto.NumParams
}

func toFunction(v Value) (*Function, error) {
	if v.Kind != KindFunction || v.Func == nil {
		return nil, fmt.Errorf("not a function")
	}
	return v.Func, nil
}

func indexGet(target Value, index Value) (Value, error) {
	switch target.Kind {
	case KindArray:
		i, err := expectIndex(index, len(target.Arr))
		if err != nil {
			return Null(), err
		}
		return target.Arr[i], nil
	case KindObject:
		key, err := expectKeyString(index)
		if err != nil {
			return Null(), err
		}
		val, ok := target.Obj[key]
		if !ok {
			return Null(), fmt.Errorf("missing key")
		}
		return val, nil
	default:
		return Null(), fmt.Errorf("not indexable")
	}
}

func indexSet(target Value, index Value, val Value) error {
	switch target.Kind {
	case KindArray:
		if target.ReadOnly {
			return fmt.Errorf("cannot modify read-only value")
		}
		i, err := expectIndex(index, len(target.Arr))
		if err != nil {
			return err
		}
		target.Arr[i] = val
		return nil
	case KindObject:
		if target.ReadOnly {
			return fmt.Errorf("cannot modify read-only value")
		}
		k, err := expectKeyString(index)
		if err != nil {
			return err
		}
		if target.Obj == nil {
			return fmt.Errorf("not indexable")
		}
		target.Obj[k] = val
		return nil
	default:
		return fmt.Errorf("not indexable")
	}
}

func indexExists(target Value, index Value) bool {
	switch target.Kind {
	case KindArray:
		_, err := expectIndex(index, len(target.Arr))
		return err == nil
	case KindObject:
		k, err := expectKeyString(index)
		if err != nil {
			return false
		}
		_, ok := target.Obj[k]
		return ok
	default:
		return false
	}
}

func valueExists(arr Value, v Value) bool {
	if arr.Kind != KindArray {
		return false
	}
	for _, el := range arr.Arr {
		if Equal(el, v) {
			return true
		}
	}
	return false
}

func toIterator(v Value) (*Iterator, error) {
	switch v.Kind {
	case KindArray:
		return NewArrayIterator(v.Arr), nil
	case KindObject:
		return NewObjectIterator(v.Obj), nil
	case KindIterator:
		if v.It == nil {
			return nil, fmt.Errorf("iterator is nil")
		}
		return v.It, nil
	default:
		return nil, fmt.Errorf("not iterable")
	}
}

func typeName(v Value) string {
	switch v.Kind {
	case KindNull:
		return "null"
	case KindBool:
		return "boolean"
	case KindNumber:
		return "number"
	case KindString:
		return "string"
	case KindArray:
		return "array"
	case KindObject:
		return "object"
	case KindFunction:
		return "function"
	case KindError:
		return "error"
	case KindIterator:
		return "iterator"
	default:
		return "unknown"
	}
}

func buildRange(start, end int) []Value {
	step := 1
	if end < start {
		step = -1
	}
	size := (end-start)/step + 1
	if size < 0 {
		size = 0
	}
	out := make([]Value, 0, size)
	for i := start; ; i += step {
		out = append(out, Number(float64(i)))
		if i == end {
			break
		}
	}
	return out
}

func indexKeyString(index Value) string {
	switch index.Kind {
	case KindString:
		return index.Str
	case KindNumber:
		if float64(int(index.Num)) == index.Num {
			return strconv.FormatInt(int64(index.Num), 10)
		}
		return fmt.Sprintf("%g", index.Num)
	default:
		return ""
	}
}

func expectIndex(index Value, length int) (int, error) {
	if index.Kind != KindNumber {
		return 0, fmt.Errorf("index must be number")
	}
	i := int(index.Num)
	if float64(i) != index.Num {
		return 0, fmt.Errorf("index must be integer")
	}
	if length >= 0 && (i < 0 || i >= length) {
		return 0, fmt.Errorf("index out of bounds")
	}
	return i, nil
}

func expectKeyString(index Value) (string, error) {
	switch index.Kind {
	case KindString:
		return index.Str, nil
	case KindNumber:
		return indexKeyString(index), nil
	default:
		return "", fmt.Errorf("key must be string or number")
	}
}
