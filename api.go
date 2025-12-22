package flux

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	_ "github.com/xirelogy/go-flux/internal/builtins"
	"github.com/xirelogy/go-flux/internal/compiler"
	"github.com/xirelogy/go-flux/internal/lexer"
	"github.com/xirelogy/go-flux/internal/parser"
	"github.com/xirelogy/go-flux/internal/vm"
)

var (
	errorType = reflect.TypeOf((*error)(nil)).Elem()
)

// VmValue is a marshaled value that is compatible with go-flux types.
// It wraps the internal vm.Value representation.
type VmValue struct {
	v     vm.Value
	owner *vm.VM
}

// ArgError represents a typed argument validation error for host functions.
type ArgError struct {
	Name string
	Want string
	Got  string
}

func (e ArgError) Error() string {
	switch {
	case e.Name != "" && e.Want != "" && e.Got != "":
		return fmt.Sprintf("argument %q: want %s, got %s", e.Name, e.Want, e.Got)
	case e.Name != "" && e.Want != "":
		return fmt.Sprintf("argument %q: want %s", e.Name, e.Want)
	default:
		return "argument error"
	}
}

// Marshaler allows custom control over Go→flux conversion.
type Marshaler interface {
	MarshalFlux() (VmValue, error)
}

// Unmarshaler allows custom control over flux→Go conversion in Unmarshal.
type Unmarshaler interface {
	UnmarshalFlux(VmValue) error
}

// MarshalOptions tunes Go→flux marshaling behavior.
type MarshalOptions struct {
	ReadOnly bool // mark array/object containers as read-only inside the VM
}

// ValueKind mirrors the flux runtime kinds for convenient inspection.
type ValueKind int

const (
	ValueNull ValueKind = iota
	ValueBool
	ValueNumber
	ValueString
	ValueArray
	ValueObject
	ValueFunction
	ValueError
	ValueIterator
)

// FrameTrace describes a single frame in a runtime error or trace.
type FrameTrace struct {
	Function string
	Source   string
	Line     int
	IP       int
}

// RuntimeError is a source-aware execution error surfaced from the VM.
type RuntimeError struct {
	Message string
	Frame   FrameTrace
	Stack   []FrameTrace
	Cause   error
}

func (e *RuntimeError) Error() string {
	parts := []string{}
	if e.Frame.Source != "" {
		if e.Frame.Line > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", e.Frame.Source, e.Frame.Line))
		} else {
			parts = append(parts, e.Frame.Source)
		}
	} else if e.Frame.Line > 0 {
		parts = append(parts, fmt.Sprintf("line %d", e.Frame.Line))
	}
	if e.Frame.Function != "" {
		parts = append(parts, fmt.Sprintf("in %s", e.Frame.Function))
	}
	loc := strings.Join(parts, " ")
	if loc != "" {
		return fmt.Sprintf("%s: %s", loc, e.Message)
	}
	return e.Message
}

// Unwrap exposes the underlying cause (if any) for errors.Is/As.
func (e *RuntimeError) Unwrap() error {
	return e.Cause
}

// TraceInfo captures execution steps for debug hooks.
type TraceInfo struct {
	Op       byte
	Function string
	Source   string
	Line     int
	IP       int
}

// TraceHook observes instruction dispatch for debugging/profiling.
type TraceHook func(TraceInfo)

func convertRuntimeError(err error) error {
	if err == nil {
		return nil
	}
	if rte, ok := err.(*vm.RuntimeError); ok {
		return &RuntimeError{
			Message: rte.Message,
			Frame:   frameTraceFromVM(rte.Frame),
			Stack:   stackTraceFromVM(rte.Stack),
			Cause:   rte.Cause,
		}
	}
	return err
}

func frameTraceFromVM(info vm.FrameInfo) FrameTrace {
	return FrameTrace{
		Function: info.Function,
		Source:   info.Source,
		Line:     info.Line,
		IP:       info.IP,
	}
}

func stackTraceFromVM(stack []vm.FrameInfo) []FrameTrace {
	if len(stack) == 0 {
		return nil
	}
	out := make([]FrameTrace, len(stack))
	for i, fr := range stack {
		out[i] = frameTraceFromVM(fr)
	}
	return out
}

// HostArgs provides typed accessors for host function arguments.
type HostArgs struct {
	args map[string]VmValue
}

// NewHostArgs wraps the raw argument map for typed access.
func NewHostArgs(args map[string]VmValue) HostArgs {
	return HostArgs{args: args}
}

// Value returns the raw VmValue for a named argument.
func (a HostArgs) Value(name string) (VmValue, error) {
	v, ok := a.args[name]
	if !ok {
		return VmValue{}, ArgError{Name: name, Want: "present"}
	}
	return v, nil
}

// Number returns the numeric argument.
func (a HostArgs) Number(name string) (float64, error) {
	v, err := a.Value(name)
	if err != nil {
		return 0, err
	}
	if n, ok := v.Number(); ok {
		return n, nil
	}
	return 0, ArgError{Name: name, Want: "number", Got: kindName(v.Kind())}
}

// String returns the string argument.
func (a HostArgs) String(name string) (string, error) {
	v, err := a.Value(name)
	if err != nil {
		return "", err
	}
	if s, ok := v.String(); ok {
		return s, nil
	}
	return "", ArgError{Name: name, Want: "string", Got: kindName(v.Kind())}
}

// Bool returns the boolean argument.
func (a HostArgs) Bool(name string) (bool, error) {
	v, err := a.Value(name)
	if err != nil {
		return false, err
	}
	if b, ok := v.Bool(); ok {
		return b, nil
	}
	return false, ArgError{Name: name, Want: "boolean", Got: kindName(v.Kind())}
}

// Array returns the array argument.
func (a HostArgs) Array(name string) ([]VmValue, error) {
	v, err := a.Value(name)
	if err != nil {
		return nil, err
	}
	if arr, ok := v.Array(); ok {
		return arr, nil
	}
	return nil, ArgError{Name: name, Want: "array", Got: kindName(v.Kind())}
}

// Object returns the object argument.
func (a HostArgs) Object(name string) (map[string]VmValue, error) {
	v, err := a.Value(name)
	if err != nil {
		return nil, err
	}
	if obj, ok := v.Object(); ok {
		return obj, nil
	}
	return nil, ArgError{Name: name, Want: "object", Got: kindName(v.Kind())}
}

// NewValue marshals a Go value into a go-flux-compatible VmValue.
func NewValue(val any) (VmValue, error) {
	return NewValueWithOptions(val, MarshalOptions{})
}

// NewValueWithOptions marshals a Go value with extra controls such as read-only marking.
func NewValueWithOptions(val any, opts MarshalOptions) (VmValue, error) {
	v, err := marshalGoValueWithOpts(val, marshalOptions{readOnly: opts.ReadOnly})
	if err != nil {
		return VmValue{}, err
	}
	return VmValue{v: v}, nil
}

// MustValue marshals and panics on error (convenience for tests/examples).
func MustValue(val any) VmValue {
	return MustValueWithOptions(val, MarshalOptions{})
}

// MustValueWithOptions marshals with options or panics on error.
func MustValueWithOptions(val any, opts MarshalOptions) VmValue {
	v, err := NewValueWithOptions(val, opts)
	if err != nil {
		panic(err)
	}
	return v
}

// MustValueReadOnly marshals and marks containers as read-only, panicking on error.
func MustValueReadOnly(val any) VmValue {
	return MustValueWithOptions(val, MarshalOptions{ReadOnly: true})
}

// MarshalFunctionMap converts a map of Go functions into a read-only flux object of callable functions.
// Supported signatures:
//
//	func(...) T
//	func(...) (T, error)
//	func(...) error
//	func(...), which returns null
//
// Where T is any type supported by NewValue marshaling.
func MarshalFunctionMap(funcs map[string]any) (VmValue, error) {
	if funcs == nil {
		return VmValue{}, errors.New("nil function map")
	}
	obj := make(map[string]vm.Value, len(funcs))
	for name, fn := range funcs {
		hostFn, err := vmFunctionFromFunc(name, fn)
		if err != nil {
			return VmValue{}, fmt.Errorf("marshal function %s: %w", name, err)
		}
		obj[name] = hostFn.toVMValueWithName(name)
	}
	return VmValue{v: vm.Value{Kind: vm.KindObject, Obj: obj, ReadOnly: true}}, nil
}

// MustMarshalFunctionMap panics on error; convenience for tests/bootstrap.
func MustMarshalFunctionMap(funcs map[string]any) VmValue {
	v, err := MarshalFunctionMap(funcs)
	if err != nil {
		panic(err)
	}
	return v
}

// Raw returns a Go representation of the value.
// Functions and iterators are not convertible and will return an error.
func (v VmValue) Raw() (any, error) {
	return v.raw()
}

// MustRaw returns Raw() or panics on error (convenience).
func (v VmValue) MustRaw() any {
	val, err := v.raw()
	if err != nil {
		panic(err)
	}
	return val
}

// AsFunction extracts a callable handle when the value is a function.
func (v VmValue) AsFunction() (*VmFunctionHandle, bool) {
	if v.v.Kind != vm.KindFunction {
		return nil, false
	}
	return &VmFunctionHandle{owner: v.owner, fn: v.v.Func}, true
}

// AsIterator extracts an iterator handle when the value is an iterator.
func (v VmValue) AsIterator() (*VmIteratorHandle, bool) {
	if v.v.Kind != vm.KindIterator {
		return nil, false
	}
	return &VmIteratorHandle{owner: v.owner, it: v.v.It}, true
}

func (v VmValue) raw() (any, error) {
	return unmarshalToGo(v.v)
}

// Kind reports the underlying value kind.
func (v VmValue) Kind() ValueKind {
	return ValueKind(v.v.Kind)
}

// IsReadOnly reports whether the value is a read-only array/object.
func (v VmValue) IsReadOnly() bool {
	return v.v.ReadOnly
}

func kindName(k ValueKind) string {
	switch k {
	case ValueNull:
		return "null"
	case ValueBool:
		return "boolean"
	case ValueNumber:
		return "number"
	case ValueString:
		return "string"
	case ValueArray:
		return "array"
	case ValueObject:
		return "object"
	case ValueFunction:
		return "function"
	case ValueError:
		return "error"
	case ValueIterator:
		return "iterator"
	default:
		return "unknown"
	}
}

// IsNull reports whether the value is null.
func (v VmValue) IsNull() bool {
	return v.v.Kind == vm.KindNull
}

// Bool returns the boolean value when the kind matches.
func (v VmValue) Bool() (bool, bool) {
	if v.v.Kind != vm.KindBool {
		return false, false
	}
	return v.v.B, true
}

// Number returns the numeric value when the kind matches.
func (v VmValue) Number() (float64, bool) {
	if v.v.Kind != vm.KindNumber {
		return 0, false
	}
	return v.v.Num, true
}

// String returns the string value when the kind matches.
func (v VmValue) String() (string, bool) {
	if v.v.Kind != vm.KindString {
		return "", false
	}
	return v.v.Str, true
}

// ErrorString returns the error string when the kind matches.
func (v VmValue) ErrorString() (string, bool) {
	if v.v.Kind != vm.KindError {
		return "", false
	}
	return v.v.Err, true
}

// Array unwraps an array into VmValues when the kind matches.
func (v VmValue) Array() ([]VmValue, bool) {
	if v.v.Kind != vm.KindArray {
		return nil, false
	}
	out := make([]VmValue, len(v.v.Arr))
	for i, el := range v.v.Arr {
		out[i] = VmValue{v: el, owner: v.owner}
	}
	return out, true
}

// Object unwraps an object into VmValues when the kind matches.
func (v VmValue) Object() (map[string]VmValue, bool) {
	if v.v.Kind != vm.KindObject {
		return nil, false
	}
	out := make(map[string]VmValue, len(v.v.Obj))
	for k, el := range v.v.Obj {
		out[k] = VmValue{v: el, owner: v.owner}
	}
	return out, true
}

// AttachFunction assigns a marshaled function to a key on an object value.
func (v *VmValue) AttachFunction(key string, fn *VmFunction) error {
	if v == nil {
		return errors.New("nil VmValue")
	}
	if v.v.Kind != vm.KindObject {
		return errors.New("AttachFunction requires object VmValue")
	}
	if v.v.Obj == nil {
		v.v.Obj = make(map[string]vm.Value)
	}
	v.v.Obj[key] = fn.toVMValueWithName(key)
	return nil
}

// Context is the execution context provided to host functions.
type Context struct{}

// FunctionHandler is the Go-side implementation of a flux function.
// Arguments are provided by name after validation against the declared parameter list.
type FunctionHandler func(ctx *Context, args map[string]VmValue) (VmValue, error)

// VmFunction describes a host-provided function, including its parameter list and handler.
type VmFunction struct {
	Params  []string
	Handler FunctionHandler
}

// VmFunctionHandle represents a function value returned from the VM.
type VmFunctionHandle struct {
	owner *vm.VM
	fn    *vm.Function
}

// Call invokes the function handle on its owning VM.
func (h *VmFunctionHandle) Call(ctx context.Context, args ...VmValue) (VmValue, error) {
	if h == nil || h.fn == nil {
		return VmValue{}, errors.New("nil function handle")
	}
	if h.owner == nil {
		return VmValue{}, errors.New("function handle missing VM owner")
	}
	argVals := make([]vm.Value, len(args))
	for i, a := range args {
		argVals[i] = a.v
	}
	res, err := h.owner.Run(h.fn, argVals)
	err = convertRuntimeError(err)
	if err != nil {
		return VmValue{}, err
	}
	return VmValue{v: res, owner: h.owner}, nil
}

// NewFunction creates a marshaled function from a parameter list and handler.
func NewFunction(params []string, handler FunctionHandler) *VmFunction {
	return &VmFunction{
		Params:  params,
		Handler: handler,
	}
}

// VmIteratorHandle represents an iterator value returned from the VM.
type VmIteratorHandle struct {
	owner *vm.VM
	it    *vm.Iterator
}

// Next advances the iterator and returns key/value.
func (h *VmIteratorHandle) Next() (string, VmValue, bool, error) {
	if h == nil || h.it == nil {
		return "", VmValue{}, false, errors.New("nil iterator handle")
	}
	key, val, ok := h.it.Next()
	return key, VmValue{v: val, owner: h.owner}, ok, nil
}

func (fn *VmFunction) toVMValueWithName(name string) vm.Value {
	native := func(runtimeVM *vm.VM, args []vm.Value) (vm.Value, error) {
		if fn == nil || fn.Handler == nil {
			return vm.ErrorVal("null handler"), errors.New("nil function handler")
		}
		if len(args) < len(fn.Params) {
			return vm.ErrorVal("argument count mismatch"), fmt.Errorf("expected at least %d args, got %d", len(fn.Params), len(args))
		}
		argMap := make(map[string]VmValue, len(fn.Params))
		for i, name := range fn.Params {
			argMap[name] = VmValue{v: args[i], owner: runtimeVM}
		}
		res, err := fn.Handler(&Context{}, argMap)
		if err != nil {
			return vm.ErrorVal(err.Error()), err
		}
		return res.v, nil
	}
	return vm.Value{Kind: vm.KindFunction, Func: &vm.Function{Native: native, Name: name, Source: "host"}}
}

func (fn *VmFunction) toVMValue() vm.Value {
	return fn.toVMValueWithName("")
}

func vmFunctionFromFunc(name string, fn any) (*VmFunction, error) {
	if fn == nil {
		return nil, errors.New("nil function")
	}
	rv := reflect.ValueOf(fn)
	rt := rv.Type()
	if rt.Kind() != reflect.Func {
		return nil, fmt.Errorf("value of %s is not a function", name)
	}
	if rt.NumOut() > 2 {
		return nil, fmt.Errorf("function %s has too many return values (max 2)", name)
	}
	retValIndex := -1
	retErrIndex := -1
	switch rt.NumOut() {
	case 0:
	case 1:
		if rt.Out(0) == errorType {
			retErrIndex = 0
		} else {
			retValIndex = 0
		}
	case 2:
		if rt.Out(1) != errorType {
			return nil, fmt.Errorf("function %s second return value must be error", name)
		}
		retValIndex = 0
		retErrIndex = 1
	}

	paramNames := make([]string, rt.NumIn())
	for i := 0; i < len(paramNames); i++ {
		paramNames[i] = fmt.Sprintf("arg%d", i)
	}

	handler := func(_ *Context, args map[string]VmValue) (VmValue, error) {
		inputs := make([]reflect.Value, rt.NumIn())
		for i := 0; i < rt.NumIn(); i++ {
			arg, ok := args[paramNames[i]]
			if !ok {
				return VmValue{}, ArgError{Name: paramNames[i], Want: "present"}
			}
			val, err := convertVmValue(arg.v, rt.In(i))
			if err != nil {
				return VmValue{}, fmt.Errorf("argument %s: %w", paramNames[i], err)
			}
			inputs[i] = val
		}
		results := rv.Call(inputs)
		if retErrIndex >= 0 && !results[retErrIndex].IsNil() {
			return VmValue{}, results[retErrIndex].Interface().(error)
		}
		if retValIndex >= 0 {
			mv, err := marshalGoValueWithOpts(results[retValIndex].Interface(), marshalOptions{})
			if err != nil {
				return VmValue{}, err
			}
			return VmValue{v: mv}, nil
		}
		return VmValue{v: vm.Null()}, nil
	}

	return &VmFunction{
		Params:  paramNames,
		Handler: handler,
	}, nil
}

// VM is the configurator/executor for go-flux scripts.
// It accumulates host bindings and script sources before execution.
type VM struct {
	core            *vm.VM
	propagateErrors bool
	mu              sync.Mutex
	busy            bool
}

// NewVM constructs a new VM configurator instance.
func NewVM() *VM {
	return &VM{
		core: vm.New(),
	}
}

// Duplicate clones the VM configuration and global state into a new instance.
// The duplicate has independent memory and no in-flight execution state.
func (vmc *VM) Duplicate() (*VM, error) {
	if vmc == nil || vmc.core == nil {
		return nil, errors.New("nil VM")
	}
	vmc.mu.Lock()
	if vmc.busy {
		vmc.mu.Unlock()
		return nil, errors.New("VM is busy; cannot duplicate while running")
	}
	vmc.busy = true
	vmc.mu.Unlock()
	defer func() {
		vmc.mu.Lock()
		vmc.busy = false
		vmc.mu.Unlock()
	}()

	core := vmc.core.Duplicate()
	if core == nil {
		return nil, errors.New("VM duplicate failed")
	}
	return &VM{
		core:            core,
		propagateErrors: vmc.propagateErrors,
	}, nil
}

// SetGlobalFunction binds a marshaled function to a global name (equivalent to a function declaration).
func (vmc *VM) SetGlobalFunction(name string, fn *VmFunction) error {
	if vmc == nil || vmc.core == nil {
		return errors.New("nil VM")
	}
	if fn == nil {
		return errors.New("nil function")
	}
	vmc.core.DefineGlobal(name, fn.toVMValueWithName(name))
	return nil
}

// HasFunction reports whether a global function exists with the given name.
func (vmc *VM) HasFunction(name string) bool {
	if vmc == nil || vmc.core == nil {
		return false
	}
	return vmc.core.HasFunction(name)
}

// LoadFile loads and compiles a script from a filesystem path.
func (vmc *VM) LoadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return vmc.LoadSource(path, string(data))
}

// LoadSource loads and compiles a script from raw source text.
// The name is used in diagnostics (e.g., "inline" or a synthetic filename).
func (vmc *VM) LoadSource(name string, src string) error {
	if vmc == nil || vmc.core == nil {
		return errors.New("nil VM")
	}
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return fmt.Errorf("parse errors: %v", errs)
	}
	mod, err := compiler.Compile(prog, name)
	if err != nil {
		return fmt.Errorf("compile error: %w", err)
	}
	vmc.core.LoadModule(mod)
	_ = name // reserved for diagnostics later
	return nil
}

// SetErrorResultAsError configures whether script-returned error values should also surface as Go errors from CallAsync/Await.
// When enabled, a function that returns an `error(...)` value will produce a VmCallResult with both Value set (KindError) and Err set.
func (vmc *VM) SetErrorResultAsError(enable bool) {
	if vmc == nil {
		return
	}
	vmc.propagateErrors = enable
}

// SetInstructionLimit caps the number of instructions a single CallAsync may execute (0 for unlimited).
func (vmc *VM) SetInstructionLimit(limit int) {
	if vmc == nil || vmc.core == nil {
		return
	}
	if limit < 0 {
		limit = 0
	}
	vmc.core.SetInstructionLimit(limit)
}

// SetTraceHook attaches a debug hook that observes instruction dispatch.
func (vmc *VM) SetTraceHook(h TraceHook) {
	if vmc == nil || vmc.core == nil {
		return
	}
	if h == nil {
		vmc.core.SetTraceHook(nil)
		return
	}
	vmc.core.SetTraceHook(func(info vm.TraceInfo) {
		h(TraceInfo{
			Op:       info.Op,
			Function: info.Function,
			Source:   info.Source,
			Line:     info.Line,
			IP:       info.IP,
		})
	})
}

// VmCallFuture represents an in-flight VM call.
type VmCallFuture struct {
	ch <-chan VmCallResult
}

// VmCallResult is the outcome of a VM call.
type VmCallResult struct {
	Value VmValue
	Err   error
}

// Await waits for completion or context cancellation.
func (f VmCallFuture) Await(ctx context.Context) (VmValue, error) {
	select {
	case <-ctx.Done():
		return VmValue{}, ctx.Err()
	case res := <-f.ch:
		return res.Value, res.Err
	}
}

// CallAsync resolves a function by name, marshals arguments, and executes it on the VM asynchronously.
func (vmc *VM) CallAsync(ctx context.Context, name string, args []VmValue) VmCallFuture {
	vmc.mu.Lock()
	if vmc.busy {
		vmc.mu.Unlock()
		ch := make(chan VmCallResult, 1)
		ch <- VmCallResult{Err: errors.New("VM is busy; concurrent CallAsync not allowed")}
		close(ch)
		return VmCallFuture{ch: ch}
	}
	vmc.busy = true
	vmc.mu.Unlock()

	ch := make(chan VmCallResult, 1)
	go func() {
		defer close(ch)
		defer func() {
			vmc.mu.Lock()
			vmc.busy = false
			vmc.mu.Unlock()
		}()
		select {
		case <-ctx.Done():
			ch <- VmCallResult{Err: ctx.Err()}
			return
		default:
		}
		argVals := make([]vm.Value, len(args))
		for i, a := range args {
			argVals[i] = a.v
		}
		res, err := vmc.core.Call(name, argVals)
		err = convertRuntimeError(err)
		if err != nil {
			ch <- VmCallResult{Err: err}
			return
		}
		outVal := VmValue{v: res, owner: vmc.core}
		if vmc.propagateErrors && res.Kind == vm.KindError {
			ch <- VmCallResult{Value: outVal, Err: errors.New(res.Err)}
			return
		}
		ch <- VmCallResult{Value: outVal}
	}()
	return VmCallFuture{ch: ch}
}

func convertVmValue(src vm.Value, targetType reflect.Type) (reflect.Value, error) {
	ptr := reflect.New(targetType)
	if err := assignValue(src, ptr.Elem()); err != nil {
		return reflect.Value{}, err
	}
	return ptr.Elem(), nil
}

type marshalOptions struct {
	readOnly bool
}

// marshalGoValue converts common Go types into vm.Value.
func marshalGoValue(val any) (vm.Value, error) {
	return marshalGoValueWithOpts(val, marshalOptions{})
}

func marshalGoValueWithOpts(val any, opts marshalOptions) (vm.Value, error) {
	if m, ok := val.(Marshaler); ok {
		custom, err := m.MarshalFlux()
		if err != nil {
			return vm.Value{}, err
		}
		return applyReadOnly(custom.v, opts), nil
	}
	switch v := val.(type) {
	case VmValue:
		return applyReadOnly(v.v, opts), nil
	case nil:
		return vm.Null(), nil
	case bool:
		return vm.Bool(v), nil
	case int:
		return vm.Number(float64(v)), nil
	case int64:
		return vm.Number(float64(v)), nil
	case float64:
		return vm.Number(v), nil
	case string:
		return vm.String(v), nil
	case error:
		return vm.ErrorVal(v.Error()), nil
	case json.Number:
		n, err := v.Float64()
		if err != nil {
			return vm.Value{}, err
		}
		return vm.Number(n), nil
	case []any:
		out := make([]vm.Value, len(v))
		for i, el := range v {
			mv, err := marshalGoValueWithOpts(el, opts)
			if err != nil {
				return vm.Value{}, err
			}
			out[i] = mv
		}
		return applyReadOnly(vm.Array(out), opts), nil
	case []VmValue:
		out := make([]vm.Value, len(v))
		for i, el := range v {
			out[i] = applyReadOnly(el.v, opts)
		}
		return applyReadOnly(vm.Array(out), opts), nil
	case map[string]any:
		out := make(map[string]vm.Value, len(v))
		for k, el := range v {
			mv, err := marshalGoValueWithOpts(el, opts)
			if err != nil {
				return vm.Value{}, err
			}
			out[k] = mv
		}
		return applyReadOnly(vm.Object(out), opts), nil
	case *VmFunction:
		return applyReadOnly(v.toVMValueWithName(""), opts), nil
	case int8:
		return vm.Number(float64(v)), nil
	case int16:
		return vm.Number(float64(v)), nil
	case int32:
		return vm.Number(float64(v)), nil
	case uint:
		return vm.Number(float64(v)), nil
	case uint8:
		return vm.Number(float64(v)), nil
	case uint16:
		return vm.Number(float64(v)), nil
	case uint32:
		return vm.Number(float64(v)), nil
	case uint64:
		return vm.Number(float64(v)), nil
	case float32:
		return vm.Number(float64(v)), nil
	case uintptr:
		return vm.Number(float64(v)), nil
	default:
		rv := reflect.ValueOf(val)
		if !rv.IsValid() {
			return vm.Null(), nil
		}
		if rv.Kind() == reflect.Pointer {
			if rv.IsNil() {
				return vm.Null(), nil
			}
			return marshalGoValueWithOpts(rv.Elem().Interface(), opts)
		}
		if rv.Kind() == reflect.Interface && !rv.IsNil() {
			return marshalGoValueWithOpts(rv.Elem().Interface(), opts)
		}
		switch rv.Kind() {
		case reflect.Bool:
			return vm.Bool(rv.Bool()), nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return vm.Number(float64(rv.Int())), nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return vm.Number(float64(rv.Uint())), nil
		case reflect.Float32, reflect.Float64:
			return vm.Number(rv.Float()), nil
		case reflect.String:
			return vm.String(rv.String()), nil
		case reflect.Slice, reflect.Array:
			out := make([]vm.Value, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				mv, err := marshalGoValueWithOpts(rv.Index(i).Interface(), opts)
				if err != nil {
					return vm.Value{}, err
				}
				out[i] = mv
			}
			return applyReadOnly(vm.Array(out), opts), nil
		case reflect.Map:
			out := make(map[string]vm.Value, rv.Len())
			iter := rv.MapRange()
			for iter.Next() {
				key := iter.Key().Interface()
				var keyStr string
				switch k := key.(type) {
				case string:
					keyStr = k
				case fmt.Stringer:
					keyStr = k.String()
				default:
					keyStr = fmt.Sprint(k)
				}
				mv, err := marshalGoValueWithOpts(iter.Value().Interface(), opts)
				if err != nil {
					return vm.Value{}, err
				}
				out[keyStr] = mv
			}
			return applyReadOnly(vm.Object(out), opts), nil
		case reflect.Struct:
			out := make(map[string]vm.Value, rv.NumField())
			rt := rv.Type()
			for i := 0; i < rv.NumField(); i++ {
				field := rt.Field(i)
				if field.PkgPath != "" { // unexported
					continue
				}
				mv, err := marshalGoValueWithOpts(rv.Field(i).Interface(), opts)
				if err != nil {
					return vm.Value{}, err
				}
				out[field.Name] = mv
			}
			return applyReadOnly(vm.Object(out), opts), nil
		}
		return vm.Value{}, fmt.Errorf("unsupported value type %T", val)
	}
}

func applyReadOnly(v vm.Value, opts marshalOptions) vm.Value {
	if !opts.readOnly {
		return v
	}
	switch v.Kind {
	case vm.KindArray:
		v.ReadOnly = true
		for i := range v.Arr {
			v.Arr[i] = applyReadOnly(v.Arr[i], opts)
		}
	case vm.KindObject:
		v.ReadOnly = true
		for k, el := range v.Obj {
			v.Obj[k] = applyReadOnly(el, opts)
		}
	}
	return v
}

// unmarshalToGo converts a vm.Value into a Go value for RawStrict().
func unmarshalToGo(v vm.Value) (any, error) {
	switch v.Kind {
	case vm.KindNull:
		return nil, nil
	case vm.KindBool:
		return v.B, nil
	case vm.KindNumber:
		return v.Num, nil
	case vm.KindString:
		return v.Str, nil
	case vm.KindArray:
		out := make([]any, len(v.Arr))
		for i, el := range v.Arr {
			val, err := unmarshalToGo(el)
			if err != nil {
				return nil, err
			}
			out[i] = val
		}
		return out, nil
	case vm.KindObject:
		out := make(map[string]any, len(v.Obj))
		for k, el := range v.Obj {
			val, err := unmarshalToGo(el)
			if err != nil {
				return nil, err
			}
			out[k] = val
		}
		return out, nil
	case vm.KindError:
		return errors.New(v.Err), nil
	case vm.KindFunction:
		return nil, errors.New("Raw() not supported on function values; use AsFunction")
	case vm.KindIterator:
		return nil, errors.New("Raw() not supported on iterator values; use AsIterator")
	default:
		return nil, fmt.Errorf("unsupported value kind %v", v.Kind)
	}
}

// Unmarshal assigns a flux VmValue into a Go target using reflection.
// Supports primitives, slices, maps (string keys), structs, and Unmarshaler.
func Unmarshal(val VmValue, target any) error {
	if target == nil {
		return errors.New("nil target")
	}
	if u, ok := target.(Unmarshaler); ok {
		return u.UnmarshalFlux(val)
	}
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("target must be non-nil pointer")
	}
	return assignValue(val.v, rv.Elem())
}

func assignValue(src vm.Value, dst reflect.Value) error {
	if !dst.CanSet() {
		return errors.New("cannot set target")
	}
	switch dst.Kind() {
	case reflect.Interface:
		raw, err := unmarshalToGo(src)
		if err != nil {
			return err
		}
		dst.Set(reflect.ValueOf(raw))
		return nil
	case reflect.Bool:
		if src.Kind != vm.KindBool {
			return ArgError{Want: "boolean", Got: kindName(ValueKind(src.Kind))}
		}
		dst.SetBool(src.B)
		return nil
	case reflect.String:
		if src.Kind != vm.KindString {
			return ArgError{Want: "string", Got: kindName(ValueKind(src.Kind))}
		}
		dst.SetString(src.Str)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if src.Kind != vm.KindNumber {
			return ArgError{Want: "number", Got: kindName(ValueKind(src.Kind))}
		}
		dst.SetInt(int64(src.Num))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if src.Kind != vm.KindNumber {
			return ArgError{Want: "number", Got: kindName(ValueKind(src.Kind))}
		}
		dst.SetUint(uint64(src.Num))
		return nil
	case reflect.Float32, reflect.Float64:
		if src.Kind != vm.KindNumber {
			return ArgError{Want: "number", Got: kindName(ValueKind(src.Kind))}
		}
		dst.SetFloat(src.Num)
		return nil
	case reflect.Slice:
		if src.Kind != vm.KindArray {
			return ArgError{Want: "array", Got: kindName(ValueKind(src.Kind))}
		}
		l := len(src.Arr)
		dst.Set(reflect.MakeSlice(dst.Type(), l, l))
		for i := 0; i < l; i++ {
			if err := assignValue(src.Arr[i], dst.Index(i)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Array:
		if src.Kind != vm.KindArray {
			return ArgError{Want: "array", Got: kindName(ValueKind(src.Kind))}
		}
		l := len(src.Arr)
		if l != dst.Len() {
			return fmt.Errorf("array length mismatch: have %d want %d", l, dst.Len())
		}
		for i := 0; i < l; i++ {
			if err := assignValue(src.Arr[i], dst.Index(i)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		if src.Kind != vm.KindObject {
			return ArgError{Want: "object", Got: kindName(ValueKind(src.Kind))}
		}
		if dst.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("map keys must be string")
		}
		dst.Set(reflect.MakeMapWithSize(dst.Type(), len(src.Obj)))
		for k, v := range src.Obj {
			elem := reflect.New(dst.Type().Elem()).Elem()
			if err := assignValue(v, elem); err != nil {
				return err
			}
			dst.SetMapIndex(reflect.ValueOf(k), elem)
		}
		return nil
	case reflect.Struct:
		if src.Kind != vm.KindObject {
			return ArgError{Want: "object", Got: kindName(ValueKind(src.Kind))}
		}
		rt := dst.Type()
		for i := 0; i < rt.NumField(); i++ {
			field := rt.Field(i)
			if field.PkgPath != "" { // unexported
				continue
			}
			name := field.Name
			if val, ok := src.Obj[name]; ok {
				if err := assignValue(val, dst.Field(i)); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported unmarshal target kind %s", dst.Kind())
	}
}
