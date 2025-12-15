package vm

import "fmt"

// Pop removes and returns the top of the value stack (or null if empty).
func (vm *VM) Pop() Value {
	return vm.pop()
}

// Push adds a value onto the stack.
func (vm *VM) Push(v Value) {
	vm.push(v)
}

// Peek inspects the top of the stack without popping (or null if empty).
func (vm *VM) Peek() Value {
	return vm.peek()
}

// RuntimeErrorf produces an error value and Go error with formatted message, capturing source context when possible.
func RuntimeErrorf(rt *VM, format string, args ...interface{}) (Value, error) {
	if rt == nil {
		err := fmt.Errorf(format, args...)
		return ErrorVal(err.Error()), err
	}
	var fr *frame
	if len(rt.frames) > 0 {
		fr = rt.currentFrame()
	}
	return rt.errorf(fr, format, args...)
}

// TypeName reports the dynamic type name for a value.
func TypeName(v Value) string {
	return typeName(v)
}

// IndexExists checks whether target contains the given index/key.
func IndexExists(target Value, index Value) bool {
	return indexExists(target, index)
}

// IndexGet reads the value at index/key from target.
func IndexGet(target Value, index Value) (Value, error) {
	return indexGet(target, index)
}

// ValueExists checks whether the array contains the given value.
func ValueExists(arr Value, val Value) bool {
	return valueExists(arr, val)
}
