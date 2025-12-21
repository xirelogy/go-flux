# go-flux

go-flux is a small, embeddable scripting language for Go. It compiles to bytecode, runs on a lightweight VM, and is designed to keep embedded business logic readable, sandboxable, and easy to integrate.

## Why go-flux?
- **Embeddable**: minimal dependencies, plain Go API.
- **Deterministic**: explicit control-flow and built-ins only.
- **Host interop**: bind Go functions/values as globals and objects.
- **Readable**: C/JS-inspired syntax with dynamic types.

## Documentation
- Language spec: [docs/LANGUAGE.md](docs/LANGUAGE.md)
- Bytecode draft: [docs/BYTECODE.md](docs/BYTECODE.md)
- License: LGPL-3.0 (see [LICENSE](LICENSE.md))

## Quick start
```go
import (
  "context"
  flux "github.com/xirelogy/go-flux"
)

vm := flux.NewVM()
// Bind a host function.
inc := flux.NewFunction([]string{"x"}, func(_ *flux.Context, args map[string]flux.VmValue) (flux.VmValue, error) {
  v, _ := args["x"].Number()
  return flux.NewValue(v + 1)
})
_ = vm.SetGlobalFunction("inc", inc)

// Load script (could also use LoadFile).
_ = vm.LoadSource("inline", `func add($a, $b) { return inc($a) + $b }`)

// Call script function.
res, err := vm.CallAsync(context.Background(), "add", []flux.VmValue{
  flux.MustValue(4), flux.MustValue(3),
}).Await(context.Background())
if err != nil {
  panic(err)
}
fmt.Println(res.MustRaw()) // 8
```

`MustValue` is a helper you can add locally via `v, _ := flux.NewValue(x)`.

## Public API

### NewVM
`func NewVM() *VM`  
Creates a new VM configurator/runner. Bind globals, load scripts, and issue calls through it. Not concurrency-safe; prefer one VM per goroutine or external locking.

### (*VM) SetGlobalFunction
`func (vm *VM) SetGlobalFunction(name string, fn *VmFunction) error`  
Binds a marshaled host function to a global name (same as declaring `func name(...)` in flux). Errors on nil receiver/function.

### (*VM) LoadFile
`func (vm *VM) LoadFile(path string) error`  
Reads, parses, compiles, and loads a script from disk. Returns I/O/parse/compile errors.

### (*VM) LoadSource
`func (vm *VM) LoadSource(name string, src string) error`  
Loads source provided in-memory. `name` is used for diagnostics. Returns parse/compile errors.

### (*VM) HasFunction
`func (vm *VM) HasFunction(name string) bool`  
Reports whether a global function exists with the given name. Returns false on nil VM.

### (*VM) CallAsync
`func (vm *VM) CallAsync(ctx context.Context, name string, args []VmValue) VmCallFuture`  
Resolves a global function by `name` and executes it with `args` on a fresh stack in a goroutine. Respects context cancellation before execution. Returns a future; results are obtained via `Await`.

Runtime and lookup failures surface as `*RuntimeError` (with function/source/line and stack trace).  
**Concurrency:** only one CallAsync may be in-flight per VM. If another call is issued while the VM is busy, the future yields an immediate error (“VM is busy; concurrent CallAsync not allowed”). Use separate VM instances or serialize calls if you need parallelism.

### (*VM) SetErrorResultAsError
`func (vm *VM) SetErrorResultAsError(enable bool)`  
When enabled, a script that returns an `error(...)` value will also surface that description as the Go error from `Await`, while still returning the `VmValue` of kind error.

### (*VM) SetInstructionLimit
`func (vm *VM) SetInstructionLimit(limit int)`  
Sets a per-call instruction cap (0 = unlimited; negative values are clamped to 0). Exceeding the cap stops execution and returns a `*RuntimeError` with message “instruction limit exceeded”, annotated with the triggering function/source/line and stack.

### (*VM) SetTraceHook
`func (vm *VM) SetTraceHook(h TraceHook)`  
Registers (or clears, with nil) an instruction-level debug hook. The hook observes each opcode before execution via `TraceInfo{Op, Function, Source, Line, IP}`; useful for profiling or custom tracing.

### (VmCallFuture) Await
`func (f VmCallFuture) Await(ctx context.Context) (VmValue, error)`  
Blocks until the call finishes or `ctx` is canceled. Returns the function result as `VmValue` or an error (runtime/lookup/cancellation).

### NewFunction
`func NewFunction(params []string, handler FunctionHandler) *VmFunction`  
Wraps a Go handler as a flux-callable function with a fixed parameter list. Arity is minimum-only: too few args yields an error value in the VM and an error to the caller; extra args are ignored. Handler receives `*Context` and map of param name → `VmValue`; return a `VmValue` or error. Use `NewHostArgs`/`HostArgs` for typed accessors with clear errors.

### NewValue / MustValue
`func NewValue(v any) (VmValue, error)` / `func MustValue(v any) VmValue`  
Marshal Go values into flux-compatible values (see marshaling rules). `MustValue` panics on error.

### (VmValue) AttachFunction
`func (v *VmValue) AttachFunction(key string, fn *VmFunction) error`  
Attaches a marshaled function as a property on an object value (e.g., to build method tables). Errors if the value is not an object or inputs are nil.

### VmValue helpers
`Kind, IsNull, Bool, Number, String, ErrorString, Array, Object, AsFunction, AsIterator, Raw, MustRaw`  
Inspect and unwrap values. `Raw`/`MustRaw` are for primitives/containers only and return an error on functions/iterators. Use `AsFunction`/`AsIterator` to obtain callable/iterable handles tied to the owning VM.

### HostArgs helpers
`NewHostArgs(args map[string]VmValue) HostArgs`  
Provides typed access to host function arguments via `Number`, `String`, `Bool`, `Array`, `Object`, and `Value`. Mismatches return `ArgError` with the parameter name and expected/actual kinds.

### RuntimeError diagnostics
`type RuntimeError struct { Message string; Frame FrameTrace; Stack []FrameTrace; Cause error }`  
`FrameTrace` holds `Function`, `Source`, `Line`, and `IP` (bytecode offset). Execution/lookup/limit errors return a `*RuntimeError`; `Cause` carries the underlying issue (e.g., a host `ArgError`) and is exposed via `errors.Is/As`. `Error()` formats the message with source/line/function for quick display.

### Marshaling customization
- Implement `Marshaler` on your types to control Go→flux conversion (`MarshalFlux() (VmValue, error)`).
- Implement `Unmarshaler` to control flux→Go conversion (`UnmarshalFlux(VmValue) error`), and/or use `Unmarshal(VmValue, targetPtr)` for reflection-based assignment into Go structs/maps/slices.

## Examples

### Host objects with methods
```go
vm := flux.NewVM()
user := flux.MustValue(map[string]any{"name": "Ada"})
greet := flux.NewFunction([]string{"self"}, func(_ *flux.Context, args map[string]flux.VmValue) (flux.VmValue, error) {
  u := args["self"]
  name, _ := u.Object()["name"].String()
  return flux.NewValue("hi " + name)
})
_ = user.AttachFunction("greet", greet)
_ = vm.SetGlobalFunction("print", flux.NewFunction([]string{"msg"}, func(_ *flux.Context, a map[string]flux.VmValue) (flux.VmValue, error) {
  fmt.Println(a["msg"].MustRaw())
  return flux.NewValue(nil)
}))
_ = vm.SetGlobalFunction("user", flux.NewFunction(nil, func(_ *flux.Context, _ map[string]flux.VmValue) (flux.VmValue, error) {
  return user, nil
}))
_ = vm.LoadSource("obj", `func run() { $u := user(); print($u.greet()) }`)
_, _ = vm.CallAsync(context.Background(), "run", nil).Await(context.Background())
```

### HostArgs and error propagation
```go
host := flux.NewFunction([]string{"n"}, func(_ *flux.Context, args map[string]flux.VmValue) (flux.VmValue, error) {
  h := flux.NewHostArgs(args)
  n, err := h.Number("n")
  if err != nil { return flux.VmValue{}, err } // ArgError surfaces as *RuntimeError
  return flux.NewValue(n * 2)
})
vm := flux.NewVM()
vm.SetGlobalFunction("twice", host)
vm.SetErrorResultAsError(true)
vm.LoadSource("arith", `func demo() { return twice("oops") }`)
_, err := vm.CallAsync(context.Background(), "demo", nil).Await(context.Background())
// err is *flux.RuntimeError wrapping ArgError with source/line info.
```

### Diagnostics and instruction limit
```go
vm := flux.NewVM()
vm.SetInstructionLimit(100)
vm.SetTraceHook(func(tr flux.TraceInfo) { fmt.Printf("%s:%d %02x\n", tr.Source, tr.Line, tr.Op) })
vm.LoadSource("spin", `func loop() { while (true) { } }`)
_, err := vm.CallAsync(context.Background(), "loop", nil).Await(context.Background())
if rte, ok := err.(*flux.RuntimeError); ok {
  fmt.Println("stopped at", rte.Frame.Source, rte.Frame.Line)
}
```

### Script-returned function handle
```go
vm := flux.NewVM()
vm.LoadSource("fn", `func makeAdder($n) { return func($x) { return $n + $x } }`)
v, _ := vm.CallAsync(context.Background(), "makeAdder", []flux.VmValue{flux.MustValue(10)}).Await(context.Background())
fn, ok := v.AsFunction()
if !ok { panic("not a function") }
res, _ := fn.Call(context.Background(), flux.MustValue(5))
fmt.Println(res.MustRaw()) // 15
```

### Custom marshaling/unmarshaling
```go
type ID string
func (i ID) MarshalFlux() (flux.VmValue, error) { return flux.NewValue(string(i)) }
type User struct{ Name string; Age int }
func (u *User) UnmarshalFlux(v flux.VmValue) error { return flux.Unmarshal(v, u) }

vm := flux.NewVM()
vm.LoadSource("user", `func wrap($id) { return { id: $id, active: true } }`)
val, _ := vm.CallAsync(context.Background(), "wrap", []flux.VmValue{flux.MustValue(ID("abc-123"))}).Await(context.Background())
var out User
_ = flux.Unmarshal(val, &out) // populates from object fields
```

### Busy-call guard
```go
vm := flux.NewVM()
vm.LoadSource("slow", `func wait() { return host() }`)
vm.SetGlobalFunction("host", flux.NewFunction(nil, func(*flux.Context, map[string]flux.VmValue) (flux.VmValue, error) {
  time.Sleep(time.Second)
  return flux.NewValue("done")
}))
f1 := vm.CallAsync(context.Background(), "wait", nil)
f2 := vm.CallAsync(context.Background(), "wait", nil) // immediately errors: VM busy
_, _ = f1.Await(context.Background())
_, err := f2.Await(context.Background())
fmt.Println(err) // "VM is busy; concurrent CallAsync not allowed"
```

### Iterating arrays/objects
```go
vm := flux.NewVM()
src := "func stats($arr, $obj) {\n" +
  "  $sum := 0\n" +
  "  for ($v in $arr) { $sum = $sum + $v }\n" +
  "  $keys := []\n" +
  "  for ([$k, $v] in $obj) { $keys = append($keys, $k) }\n" +
  "  return { sum: $sum, keys: $keys }\n" +
  "}\n"
vm.LoadSource("iter", src)
arr := flux.MustValue([]int{1, 2, 3})
obj := flux.MustValue(map[string]any{"a": 10, "b": 20})
res, _ := vm.CallAsync(context.Background(), "stats", []flux.VmValue{arr, obj}).Await(context.Background())
fmt.Println(res.MustRaw()) // map[string]any{"keys":[]any{"a","b"},"sum":6}
```

### Iterator handle from Go
```go
vm := flux.NewVM()
vm.LoadSource("iters", "func makeIter() { return [0 .. 2] }")
v, _ := vm.CallAsync(context.Background(), "makeIter", nil).Await(context.Background())
it, ok := v.AsIterator()
if !ok { panic("not an iterator") }
for {
  key, val, ok, _ := it.Next()
  if !ok { break }
  fmt.Printf("%s=%v\n", key, val.MustRaw())
}
// prints:
// 0=0
// 1=1
// 2=2
```

## Value marshaling (Go ↔ flux)
- Numbers: any Go int/uint/float/json.Number is converted to `number` (float64).
- Bools/strings map directly; errors become `error` with the message.
- Slices/arrays marshal to flux arrays; maps (any key type) marshal to objects with keys stringified via `fmt.Sprint`; struct fields marshal to objects using exported field names.
- Pointers/interfaces are dereferenced; nil pointers/interfaces become `null`.
- Functions: `*flux.VmFunction` marshals to a callable flux function; script-side functions cannot be flattened with `Raw()` (it errors) but can be inspected via `AsFunction` (handle, callable on the owning VM). No round-trip of closures to Go-native funcs.
- Iterators likewise cannot be flattened with `Raw()`; use `AsIterator` for handle-style access.
- `VmValue` helpers: `Kind`, `IsNull`, `Bool/Number/String/ErrorString`, `Array`, `Object`, `Raw()`/`MustRaw()` for primitives, and `AsFunction`/`AsIterator` for handles.
- Host can mark marshaled arrays/objects as read-only with `flux.NewValueWithOptions(val, flux.MarshalOptions{ReadOnly: true})` (or `MustValueWithOptions`). Scripts can query with `readonly($x)`; attempts to mutate throw a runtime error.

### Marshaling function maps (RPC-style namespaces)
```go
ns := flux.MustMarshalFunctionMap(map[string]any{
  "add":  func(a int, b int) int { return a + b },
  "ping": func(s string) (string, error) { return s + "!", nil },
  "fail": func() error { return fmt.Errorf("boom") },
})
vm.LoadSource("rpc", `func use($ns) { return [$ns.add(1,2), $ns.ping("ok")] }`)
out, _ := vm.CallAsync(context.Background(), "use", []flux.VmValue{ns}).Await(context.Background())
fmt.Println(out.MustRaw()) // [3 "ok!"]
```
- Functions must be real Go funcs (non-nil), with up to two return values (if two, the second must be `error`; if one, it can be a value or `error`).
- Arguments and return values are marshaled/unmarshaled automatically using the same rules as `NewValue`/`Unmarshal`.
- The produced namespace object is read-only in the VM.

## Notes
- The API surface may evolve as diagnostics, limits, and tooling mature.
