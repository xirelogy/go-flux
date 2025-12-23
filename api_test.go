package flux

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

type testCustomMarshaler struct{ V string }
type testCustomUnmarshaler struct{ V string }

var _ Marshaler = (*testCustomMarshaler)(nil)
var _ Unmarshaler = (*testCustomUnmarshaler)(nil)

func (c testCustomMarshaler) MarshalFlux() (VmValue, error) {
	return NewValue(map[string]any{"v": c.V})
}

func (c *testCustomUnmarshaler) UnmarshalFlux(v VmValue) error {
	obj, ok := v.Object()
	if !ok {
		return fmt.Errorf("expected object")
	}
	val, ok := obj["v"].String()
	if !ok {
		return fmt.Errorf("missing v")
	}
	c.V = val
	return nil
}

func TestAPIScriptCall(t *testing.T) {
	vm := NewVM()
	err := vm.LoadSource("inline", `func add($a, $b) { return $a + $b }`)
	if err != nil {
		t.Fatalf("load source: %v", err)
	}
	a1, _ := NewValue(2)
	a2, _ := NewValue(3)
	res, err := vm.CallAsync(context.Background(), "add", []VmValue{a1, a2}).Await(context.Background())
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	if v, ok := res.MustRaw().(float64); !ok || v != 5 {
		t.Fatalf("expected 5, got %#v", res)
	}
}

func TestAPIHostFunctionBinding(t *testing.T) {
	vm := NewVM()
	host := NewFunction([]string{"x"}, func(ctx *Context, args map[string]VmValue) (VmValue, error) {
		val := args["x"].MustRaw().(float64)
		return NewValue(val + 1)
	})
	if err := vm.SetGlobalFunction("inc", host); err != nil {
		t.Fatalf("set global: %v", err)
	}
	err := vm.LoadSource("inline", `func run($v) { return inc($v) }`)
	if err != nil {
		t.Fatalf("load source: %v", err)
	}
	arg, _ := NewValue(4)
	res, err := vm.CallAsync(context.Background(), "run", []VmValue{arg}).Await(context.Background())
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	if v, ok := res.MustRaw().(float64); !ok || v != 5 {
		t.Fatalf("expected 5, got %#v", res)
	}
}

func TestAPIHasFunction(t *testing.T) {
	vm := NewVM()
	if vm.HasFunction("missing") {
		t.Fatalf("expected missing to be false")
	}
	if err := vm.LoadSource("inline", `func add($a, $b) { return $a + $b }`); err != nil {
		t.Fatalf("load source: %v", err)
	}
	if !vm.HasFunction("add") {
		t.Fatalf("expected add to be true")
	}
	if vm.HasFunction("missing") {
		t.Fatalf("expected missing to be false")
	}
	host := NewFunction(nil, func(ctx *Context, args map[string]VmValue) (VmValue, error) {
		return MustValue(1), nil
	})
	if err := vm.SetGlobalFunction("host", host); err != nil {
		t.Fatalf("set global: %v", err)
	}
	if !vm.HasFunction("host") {
		t.Fatalf("expected host to be true")
	}
}

func TestAPIVMDuplicateIsolation(t *testing.T) {
	base := NewVM()
	err := base.LoadSource("inline", `
func init() {
  $state = { count: 0 }
}
func bump() {
  $state.count = $state.count + 1
  return $state.count
}
`)
	if err != nil {
		t.Fatalf("load source: %v", err)
	}
	_, err = base.CallAsync(context.Background(), "init", nil).Await(context.Background())
	if err != nil {
		t.Fatalf("init call: %v", err)
	}
	dup, err := base.Duplicate()
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	dupFirst, err := dup.CallAsync(context.Background(), "bump", nil).Await(context.Background())
	if err != nil {
		t.Fatalf("dup bump: %v", err)
	}
	if v, ok := dupFirst.MustRaw().(float64); !ok || v != 1 {
		t.Fatalf("expected dup to return 1, got %#v", dupFirst)
	}
	baseFirst, err := base.CallAsync(context.Background(), "bump", nil).Await(context.Background())
	if err != nil {
		t.Fatalf("base bump: %v", err)
	}
	if v, ok := baseFirst.MustRaw().(float64); !ok || v != 1 {
		t.Fatalf("expected base to return 1, got %#v", baseFirst)
	}
	dupSecond, err := dup.CallAsync(context.Background(), "bump", nil).Await(context.Background())
	if err != nil {
		t.Fatalf("dup bump second: %v", err)
	}
	if v, ok := dupSecond.MustRaw().(float64); !ok || v != 2 {
		t.Fatalf("expected dup to return 2, got %#v", dupSecond)
	}
}

func TestAPILanguageCoverage(t *testing.T) {
	run := func(t *testing.T, src, entry string, args []any) (any, error) {
		t.Helper()
		vm := NewVM()
		if err := vm.LoadSource("inline", src); err != nil {
			return nil, err
		}
		argVals := make([]VmValue, len(args))
		for i, a := range args {
			argVals[i] = MustValue(a)
		}
		val, err := vm.CallAsync(context.Background(), entry, argVals).Await(context.Background())
		if err != nil {
			return nil, err
		}
		return val.Raw()
	}

	tests := []struct {
		name  string
		src   string
		entry string
		args  []any
		check func(t *testing.T, res any, err error)
	}{
		{
			name:  "literals and typeof",
			entry: "demo",
			src: `
// comments should be ignored
func demo() {
  return [null, true, false, 123, 5.5, "hi", typeof("x")]
}
`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				want := []any{nil, true, false, float64(123), 5.5, "hi", "string"}
				if !reflect.DeepEqual(res, want) {
					t.Fatalf("expected %v, got %#v", want, res)
				}
			},
		},
		{
			name:  "arithmetic and grouping",
			entry: "demo",
			src: `
func demo() { return (2 + 3) * 4 / 2 - 1 }
`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				if res.(float64) != 9 {
					t.Fatalf("expected 9, got %#v", res)
				}
			},
		},
		{
			name:  "comparisons and branching",
			entry: "cmp",
			args:  []any{1, 2},
			src: `
func cmp($a, $b) {
  if ($a == $b) {
    return "eq"
  } elseif ($a < $b) {
    return "lt"
  } else {
    return "gt"
  }
}
`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				if res != "lt" {
					t.Fatalf("expected lt, got %#v", res)
				}
			},
		},
		{
			name:  "boolean logic",
			entry: "demo",
			args:  []any{false, true},
			src:   `func demo($a, $b) { return ($a && $b) || (!$a && $b) }`,
			check: func(t *testing.T, res any, err error) {
				if err != nil || res != true {
					t.Fatalf("expected true, got res=%#v err=%v", res, err)
				}
			},
		},
		{
			name:  "arrays and builtins",
			entry: "arr",
			src: `
func arr() {
  $a := ["example", 567, "good",]
  $exists := indexExist($a, 2)
  $safe := indexRead($a, 5, "def")
  $has := valueExist($a, "good")
  return [$a[1], $exists, $safe, $has]
}
`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				want := []any{float64(567), true, "def", true}
				if !reflect.DeepEqual(res, want) {
					t.Fatalf("expected %v, got %#v", want, res)
				}
			},
		},
		{
			name:  "objects and property access",
			entry: "obj",
			src: `
func obj() {
  $o := { "hello": "world", 6: "sample", }
  $o.extra = "x"
  $exists := indexExist($o, "hello")
  $safe := indexRead($o, "missing", "fallback")
  return [$o.hello, $o["6"], $exists, $safe]
}
`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				want := []any{"world", "sample", true, "fallback"}
				if !reflect.DeepEqual(res, want) {
					t.Fatalf("expected %v, got %#v", want, res)
				}
			},
		},
		{
			name:  "for range loop",
			entry: "sum",
			src: `
func sum() {
  $s := 0
  for ($v in [0 .. 3]) { $s = $s + $v }
  return $s
}
`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				if res.(float64) != 6 {
					t.Fatalf("expected 6, got %#v", res)
				}
			},
		},
		{
			name:  "for key/value loop",
			entry: "sum",
			src: `
func sum() {
  $o := { a:1, b:2 }
  $s := 0
  for ([$k, $v] in $o) { $s = $s + $v }
  return $s
}
`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				if res.(float64) != 3 {
					t.Fatalf("expected 3, got %#v", res)
				}
			},
		},
		{
			name:  "while loop",
			entry: "demo",
			src: `
func demo() {
  $n := 0
  $acc := 1
  while ($n < 3) {
    $acc = $acc * 2
    $n = $n + 1
  }
  return $acc
}
`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				if res.(float64) != 8 {
					t.Fatalf("expected 8, got %#v", res)
				}
			},
		},
		{
			name:  "object functions direct and indirect",
			entry: "demo",
			src: `
func demo() {
  $o := { minus: func ($a, $b) { return $a - $b }, }
  $o.plus = func ($a, $b) { return $a + $b }
  $m := $o.minus(5, 3)
  $p := $o.plus(2, 3)
  return [$m, $p]
}
`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				want := []any{float64(2), float64(5)}
				if !reflect.DeepEqual(res, want) {
					t.Fatalf("expected %v, got %#v", want, res)
				}
			},
		},
		{
			name:  "type info and error builtin",
			entry: "demo",
			src: `
func demo() {
  $a := typeof("abc")
  $b := typeof(123)
  $c := typeof(error("boom"))
  return [$a, $b, $c]
}
`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				want := []any{"string", "number", "error"}
				if !reflect.DeepEqual(res, want) {
					t.Fatalf("expected %v, got %#v", want, res)
				}
			},
		},
		{
			name:  "range literal",
			entry: "demo",
			src:   `func demo() { return [0 .. 3] }`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				want := []any{float64(0), float64(1), float64(2), float64(3)}
				if !reflect.DeepEqual(res, want) {
					t.Fatalf("expected %v, got %#v", want, res)
				}
			},
		},
		{
			name:  "default null return",
			entry: "demo",
			src:   `func demo() { }`,
			check: func(t *testing.T, res any, err error) {
				if err != nil {
					t.Fatalf("call error: %v", err)
				}
				if res != nil {
					t.Fatalf("expected nil, got %#v", res)
				}
			},
		},
		{
			name:  "array index error",
			entry: "demo",
			src: `
func demo() {
  $a := [1]
  return $a[5]
}
`,
			check: func(t *testing.T, res any, err error) {
				if err == nil {
					t.Fatalf("expected out-of-bounds error, got %#v", res)
				}
			},
		},
		{
			name:  "case sensitivity",
			entry: "demo",
			src:   `func demo() { return 1 }`,
			check: func(t *testing.T, res any, err error) {
				if err != nil || res.(float64) != 1 {
					t.Fatalf("unexpected result %#v err=%v", res, err)
				}
				vm := NewVM()
				if err := vm.LoadSource("inline", `func lower() { return 1 }`); err != nil {
					t.Fatalf("load: %v", err)
				}
				if _, err := vm.CallAsync(context.Background(), "LOWER", nil).Await(context.Background()); err == nil {
					t.Fatalf("expected case-sensitive miss to error")
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			res, err := run(t, tt.src, tt.entry, tt.args)
			tt.check(t, res, err)
		})
	}

	// Marshaling/unmarshaling of error values.
	errVal, err := NewValue(errors.New("boom"))
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if errVal.MustRaw().(error).Error() != "boom" {
		t.Fatalf("expected marshaled error, got %#v", errVal.MustRaw())
	}
}

func TestAPIHostInteropMarshaling(t *testing.T) {
	vm := NewVM()
	script := `
func read($obj) {
  return $obj.value + $obj.nums[1]
}

func callHost($fn) {
  return $fn(5, 7)
}
`
	if err := vm.LoadSource("interop", script); err != nil {
		t.Fatalf("load: %v", err)
	}

	// Host object marshaling (map + slice).
	obj := map[string]any{
		"value": 3,
		"nums":  []int{1, 4, 9},
	}
	objVal := MustValue(obj)
	res, err := vm.CallAsync(context.Background(), "read", []VmValue{objVal}).Await(context.Background())
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	if res.MustRaw().(float64) != 7 {
		t.Fatalf("expected 7, got %#v", res)
	}

	// Host function marshaling and arity validation.
	hostFn := NewFunction([]string{"a", "b"}, func(_ *Context, args map[string]VmValue) (VmValue, error) {
		return NewValue(args["a"].MustRaw().(float64) + args["b"].MustRaw().(float64))
	})
	fnVal := hostFn
	res, err = vm.CallAsync(context.Background(), "callHost", []VmValue{MustValue(fnVal)}).Await(context.Background())
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	if res.MustRaw().(float64) != 12 {
		t.Fatalf("expected 12, got %#v", res)
	}
}

func TestAPIReadonlyMarshaledValues(t *testing.T) {
	vm := NewVM()
	script := `
func mutate($o, $a) {
  $o.k = 2
  $a[0] = 5
  return [$o.k, $a[0]]
}

func check($x) {
  return readonly($x)
}`
	if err := vm.LoadSource("readonly", script); err != nil {
		t.Fatalf("load: %v", err)
	}

	roObj := MustValueWithOptions(map[string]any{"k": 1.0}, MarshalOptions{ReadOnly: true})
	roArr := MustValueWithOptions([]any{1.0}, MarshalOptions{ReadOnly: true})
	if _, err := vm.CallAsync(context.Background(), "mutate", []VmValue{roObj, roArr}).Await(context.Background()); err == nil {
		t.Fatalf("expected mutation of read-only values to fail")
	} else if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only error, got %v", err)
	}

	// Builtin check: host-marked values report as read-only.
	roCheck, err := vm.CallAsync(context.Background(), "check", []VmValue{roObj}).Await(context.Background())
	if err != nil {
		t.Fatalf("readonly check call error: %v", err)
	}
	if val, ok := roCheck.Bool(); !ok || !val {
		t.Fatalf("expected readonly(...) to report true, got %#v", roCheck)
	}

	// Mutable values remain writable.
	obj := MustValue(map[string]any{"k": 1.0})
	arr := MustValue([]any{1.0})
	res, err := vm.CallAsync(context.Background(), "mutate", []VmValue{obj, arr}).Await(context.Background())
	if err != nil {
		t.Fatalf("mutation on mutable values failed: %v", err)
	}
	out, ok := res.Array()
	if !ok || len(out) != 2 {
		t.Fatalf("unexpected mutate return: %#v", res)
	}
	if v, _ := out[0].Number(); v != 2 {
		t.Fatalf("expected object field update to 2, got %#v", out[0])
	}
	if v, _ := out[1].Number(); v != 5 {
		t.Fatalf("expected array update to 5, got %#v", out[1])
	}
}

func TestAPIBuiltinsAutoRegistered(t *testing.T) {
	vm := NewVM()
	src := `func demo($x) { return typeof($x) }`
	if err := vm.LoadSource("inline", src); err != nil {
		t.Fatalf("load: %v", err)
	}
	res, err := vm.CallAsync(context.Background(), "demo", []VmValue{MustValue(123)}).Await(context.Background())
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	if res.MustRaw() != "number" {
		t.Fatalf("expected number, got %#v", res.MustRaw())
	}
}

func TestAPIFunctionMapMarshal(t *testing.T) {
	vm := NewVM()
	script := `
func call($ns) {
  $a := $ns.add(2, 3)
  $b := $ns.greet("hi")
  $r := readonly($ns)
  return [$a, $b, $r]
}
func boom($ns) { return $ns.fail() }`
	if err := vm.LoadSource("rpc", script); err != nil {
		t.Fatalf("load: %v", err)
	}

	funcs := map[string]any{
		"add": func(a int, b int) int { return a + b },
		"greet": func(s string) (string, error) {
			return s + "!", nil
		},
		"fail": func() error {
			return fmt.Errorf("nope")
		},
	}
	ns := MustMarshalFunctionMap(funcs)
	if !ns.IsReadOnly() {
		t.Fatalf("expected namespace to be read-only")
	}

	callRes, err := vm.CallAsync(context.Background(), "call", []VmValue{ns}).Await(context.Background())
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	arr, ok := callRes.Array()
	if !ok || len(arr) != 3 {
		t.Fatalf("unexpected call result %#v", callRes)
	}
	if n, _ := arr[0].Number(); n != 5 {
		t.Fatalf("expected add to return 5, got %#v", arr[0])
	}
	if s, _ := arr[1].String(); s != "hi!" {
		t.Fatalf("expected greet result hi!, got %#v", arr[1])
	}
	if arr[2].Kind() != ValueBool || arr[2].MustRaw().(bool) != true {
		t.Fatalf("expected readonly($ns) to return true, got %#v", arr[2])
	}

	if _, err := vm.CallAsync(context.Background(), "boom", []VmValue{ns}).Await(context.Background()); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected boom to propagate error nope, got %v", err)
	}
}

func TestAPIHostFunctionNullInterfaceArg(t *testing.T) {
	vm := NewVM()
	script := `func run($ns) { return $ns.get("key", null) }`
	if err := vm.LoadSource("inline", script); err != nil {
		t.Fatalf("load: %v", err)
	}
	ns := MustMarshalFunctionMap(map[string]any{
		"get": func(key string, fallback any) any {
			if key != "key" {
				return "bad"
			}
			return fallback
		},
	})
	res, err := vm.CallAsync(context.Background(), "run", []VmValue{ns}).Await(context.Background())
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	if !res.IsNull() {
		t.Fatalf("expected null result, got %#v", res)
	}
}

func TestAPIScriptErrorPromotion(t *testing.T) {
	script := `func boom() { return error("boom") }`

	// Default: error value returned, Go error nil.
	vm := NewVM()
	if err := vm.LoadSource("inline", script); err != nil {
		t.Fatalf("load: %v", err)
	}
	val, err := vm.CallAsync(context.Background(), "boom", nil).Await(context.Background())
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if val.Kind() != ValueError {
		t.Fatalf("expected ValueError, got %v", val.Kind())
	}
	if msg, ok := val.ErrorString(); !ok || msg != "boom" {
		t.Fatalf("unexpected error string %q ok=%v", msg, ok)
	}

	// Promotion: both Value and Err set.
	vm2 := NewVM()
	if err := vm2.LoadSource("inline", script); err != nil {
		t.Fatalf("load: %v", err)
	}
	vm2.SetErrorResultAsError(true)
	val2, err := vm2.CallAsync(context.Background(), "boom", nil).Await(context.Background())
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected propagated Go error 'boom', got %v", err)
	}
	if val2.Kind() != ValueError {
		t.Fatalf("expected ValueError, got %v", val2.Kind())
	}
}

func TestAPIRuntimeErrorDiagnostics(t *testing.T) {
	vm := NewVM()
	src := `func inner($arr) {
  return $arr[5]
}

func outer() {
  return inner([1, 2, 3])
}`
	if err := vm.LoadSource("diag", src); err != nil {
		t.Fatalf("load: %v", err)
	}
	_, err := vm.CallAsync(context.Background(), "outer", nil).Await(context.Background())
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	rte, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if rte.Frame.Function != "inner" {
		t.Fatalf("expected top frame inner, got %q", rte.Frame.Function)
	}
	if rte.Frame.Source != "diag" {
		t.Fatalf("expected source diag, got %q", rte.Frame.Source)
	}
	if rte.Frame.Line != 2 {
		t.Fatalf("expected line 2, got %d", rte.Frame.Line)
	}
	if len(rte.Stack) < 2 {
		t.Fatalf("expected at least 2 frames, got %d", len(rte.Stack))
	}
	if rte.Stack[1].Function != "outer" {
		t.Fatalf("expected caller outer, got %q", rte.Stack[1].Function)
	}
}

func TestAPITraceHook(t *testing.T) {
	vm := NewVM()
	var traces []TraceInfo
	vm.SetTraceHook(func(info TraceInfo) {
		traces = append(traces, info)
	})
	if err := vm.LoadSource("trace", `func demo() { return 1 + 2 }`); err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, err := vm.CallAsync(context.Background(), "demo", nil).Await(context.Background()); err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(traces) == 0 {
		t.Fatalf("expected trace events")
	}
	for _, tr := range traces {
		if tr.Function != "demo" {
			t.Fatalf("expected function demo in trace, got %q", tr.Function)
		}
		if tr.Source != "trace" {
			t.Fatalf("expected trace source, got %q", tr.Source)
		}
		if tr.Line == 0 {
			t.Fatalf("expected line info in trace")
		}
	}
}

func TestAPIInstructionLimit(t *testing.T) {
	vm := NewVM()
	vm.SetInstructionLimit(50)
	if err := vm.LoadSource("limit", `func spin() { while (true) { } }`); err != nil {
		t.Fatalf("load: %v", err)
	}
	_, err := vm.CallAsync(context.Background(), "spin", nil).Await(context.Background())
	if err == nil {
		t.Fatalf("expected instruction limit error")
	}
	rte, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("expected RuntimeError, got %T", err)
	}
	if rte.Message != "instruction limit exceeded" {
		t.Fatalf("unexpected message %q", rte.Message)
	}
	if rte.Frame.Function != "spin" {
		t.Fatalf("expected frame spin, got %q", rte.Frame.Function)
	}
}

func TestAPIHostArgHelpersAndExtraArgs(t *testing.T) {
	vm := NewVM()
	script := `func run($a, $b, $c) { return host($a, $b, $c) }`
	if err := vm.LoadSource("inline", script); err != nil {
		t.Fatalf("load: %v", err)
	}

	host := NewFunction([]string{"x", "y"}, func(_ *Context, args map[string]VmValue) (VmValue, error) {
		h := NewHostArgs(args)
		x, err := h.Number("x")
		if err != nil {
			return VmValue{}, err
		}
		y, err := h.String("y")
		if err != nil {
			return VmValue{}, err
		}
		return NewValue(fmt.Sprintf("%g:%s", x, y))
	})
	if err := vm.SetGlobalFunction("host", host); err != nil {
		t.Fatalf("bind: %v", err)
	}

	val, err := vm.CallAsync(context.Background(), "run", []VmValue{
		MustValue(1),
		MustValue("two"),
		MustValue(true), // extra arg should be ignored by host binding
	}).Await(context.Background())
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	if val.MustRaw() != "1:two" {
		t.Fatalf("unexpected result %#v", val.MustRaw())
	}

	// Type mismatch yields ArgError surfaced to VM and Go caller.
	badHost := NewFunction([]string{"n"}, func(_ *Context, args map[string]VmValue) (VmValue, error) {
		h := NewHostArgs(args)
		_, err := h.Number("n")
		return VmValue{}, err
	})
	if err := vm.SetGlobalFunction("bad", badHost); err != nil {
		t.Fatalf("bind bad: %v", err)
	}
	_, err = vm.CallAsync(context.Background(), "run", []VmValue{
		MustValue("oops"), // for x
		MustValue("two"),
		MustValue(true),
	}).Await(context.Background())
	if err == nil {
		t.Fatalf("expected error from bad host arg")
	}
	var argErr ArgError
	if !errors.As(err, &argErr) {
		t.Fatalf("expected ArgError, got %T", err)
	}
}

func TestAPIHostFunctionBlocksVM(t *testing.T) {
	vm := NewVM()
	script := `func slowCall($x) { return host($x) }`
	if err := vm.LoadSource("inline", script); err != nil {
		t.Fatalf("load: %v", err)
	}

	// Host function sleeps; VM call should not finish before sleep elapses (synchronous behavior).
	hostFn := NewFunction([]string{"v"}, func(_ *Context, args map[string]VmValue) (VmValue, error) {
		time.Sleep(30 * time.Millisecond)
		return NewValue(args["v"].MustRaw())
	})
	if err := vm.SetGlobalFunction("host", hostFn); err != nil {
		t.Fatalf("bind host: %v", err)
	}

	start := time.Now()
	res, err := vm.CallAsync(context.Background(), "slowCall", []VmValue{MustValue(42)}).Await(context.Background())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	if res.MustRaw().(float64) != 42 {
		t.Fatalf("unexpected result %#v", res.MustRaw())
	}
	if elapsed < 25*time.Millisecond {
		t.Fatalf("expected blocking host call; elapsed %v too short", elapsed)
	}
}

func TestAPICallAsyncBusyProtection(t *testing.T) {
	vm := NewVM()
	script := `func slow() { return host() }`
	if err := vm.LoadSource("inline", script); err != nil {
		t.Fatalf("load: %v", err)
	}
	hostFn := NewFunction(nil, func(_ *Context, _ map[string]VmValue) (VmValue, error) {
		time.Sleep(50 * time.Millisecond)
		return NewValue(1)
	})
	if err := vm.SetGlobalFunction("host", hostFn); err != nil {
		t.Fatalf("bind host: %v", err)
	}

	fut1 := vm.CallAsync(context.Background(), "slow", nil)
	fut2 := vm.CallAsync(context.Background(), "slow", nil)

	_, err := fut2.Await(context.Background())
	if err == nil {
		t.Fatalf("expected busy error on concurrent CallAsync")
	}

	val, err := fut1.Await(context.Background())
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if num, ok := val.Number(); !ok || num != 1 {
		t.Fatalf("unexpected result %v ok=%v", num, ok)
	}
}

func TestAPIBroaderMarshalingAndAccessors(t *testing.T) {
	type myInt int64
	type sample struct {
		Name  string
		Count uint8
	}
	marshalFunc := func(c testCustomMarshaler) VmValue {
		v, err := c.MarshalFlux()
		if err != nil {
			t.Fatalf("marshal flux: %v", err)
		}
		return v
	}

	t.Run("numeric aliases and json.Number", func(t *testing.T) {
		v := MustValue(myInt(99))
		if num, ok := v.Number(); !ok || num != 99 {
			t.Fatalf("expected 99 number, got %v ok=%v", num, ok)
		}
		j := MustValue(json.Number("5.25"))
		if num, ok := j.Number(); !ok || num != 5.25 {
			t.Fatalf("expected 5.25 number, got %v ok=%v", num, ok)
		}
	})

	t.Run("pointers", func(t *testing.T) {
		x := 7
		v, err := NewValue(&x)
		if err != nil {
			t.Fatalf("marshal pointer: %v", err)
		}
		if num, ok := v.Number(); !ok || num != 7 {
			t.Fatalf("expected 7 number from pointer, got %v ok=%v", num, ok)
		}
		var nilPtr *int
		nullVal, err := NewValue(nilPtr)
		if err != nil {
			t.Fatalf("marshal nil pointer: %v", err)
		}
		if !nullVal.IsNull() {
			t.Fatalf("expected null for nil pointer, got %#v", nullVal.MustRaw())
		}
	})

	t.Run("slices arrays maps", func(t *testing.T) {
		sliceVal := MustValue([]uint16{1, 2, 3})
		arr, ok := sliceVal.Array()
		if !ok || len(arr) != 3 {
			t.Fatalf("expected array length 3 ok=%v", ok)
		}
		if first, ok := arr[0].Number(); !ok || first != 1 {
			t.Fatalf("expected first element 1, got %v ok=%v", first, ok)
		}

		arrayVal := MustValue([2]int{4, 5})
		arr2, ok := arrayVal.Array()
		if !ok || len(arr2) != 2 {
			t.Fatalf("expected array length 2 ok=%v", ok)
		}
		if second, ok := arr2[1].Number(); !ok || second != 5 {
			t.Fatalf("expected second element 5, got %v ok=%v", second, ok)
		}

		m := map[any]any{1: "one", "two": 2}
		mapVal := MustValue(m)
		raw, err := mapVal.Raw()
		if err != nil {
			t.Fatalf("raw map: %v", err)
		}
		rawMap := raw.(map[string]any)
		if rawMap["1"] != "one" {
			t.Fatalf("expected key \"1\" => one, got %#v", rawMap["1"])
		}
		if rawMap["two"] != float64(2) {
			t.Fatalf("expected key \"two\" => 2, got %#v", rawMap["two"])
		}
	})

	t.Run("structs", func(t *testing.T) {
		s := sample{Name: "demo", Count: 4}
		val := MustValue(s)
		obj, ok := val.Object()
		if !ok {
			t.Fatalf("expected object")
		}
		if name, ok := obj["Name"].String(); !ok || name != "demo" {
			t.Fatalf("expected Name=demo, got %v ok=%v", name, ok)
		}
		if count, ok := obj["Count"].Number(); !ok || count != 4 {
			t.Fatalf("expected Count=4, got %v ok=%v", count, ok)
		}
	})

	t.Run("accessor mismatches", func(t *testing.T) {
		val := MustValue(true)
		if _, ok := val.String(); ok {
			t.Fatalf("expected string accessor to fail")
		}
		if val.Kind() != ValueBool {
			t.Fatalf("expected kind bool, got %v", val.Kind())
		}
	})

	t.Run("custom marshaler/unmarshaler and unmarshal helper", func(t *testing.T) {
		cm := testCustomMarshaler{V: "hello"}
		val := marshalFunc(cm)
		obj, ok := val.Object()
		if !ok {
			t.Fatalf("expected object")
		}
		if vstr, ok := obj["v"].String(); !ok || vstr != "hello" {
			t.Fatalf("expected marshaled object with v=hello, got %#v ok=%v", obj, ok)
		}

		var cu testCustomUnmarshaler
		if err := Unmarshal(val, &cu); err != nil {
			t.Fatalf("unmarshal custom: %v", err)
		}
		if cu.V != "hello" {
			t.Fatalf("expected cu.V=hello, got %q", cu.V)
		}

		var arr []int
		if err := Unmarshal(MustValue([]any{1.0, 2.0}), &arr); err != nil {
			t.Fatalf("unmarshal slice: %v", err)
		}
		if !reflect.DeepEqual(arr, []int{1, 2}) {
			t.Fatalf("unexpected slice %v", arr)
		}

		var objMap map[string]int
		if err := Unmarshal(MustValue(map[string]any{"a": 3.0}), &objMap); err != nil {
			t.Fatalf("unmarshal map: %v", err)
		}
		if objMap["a"] != 3 {
			t.Fatalf("unexpected map %v", objMap)
		}

		var s sample
		if err := Unmarshal(MustValue(map[string]any{"Name": "x", "Count": 5.0}), &s); err != nil {
			t.Fatalf("unmarshal struct: %v", err)
		}
		if s.Name != "x" || s.Count != 5 {
			t.Fatalf("unexpected struct %+v", s)
		}
	})
}
