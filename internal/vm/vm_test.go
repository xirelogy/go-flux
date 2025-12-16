package vm_test

import (
	"testing"

	"github.com/xirelogy/go-flux/internal/ast"
	_ "github.com/xirelogy/go-flux/internal/builtins"
	"github.com/xirelogy/go-flux/internal/compiler"
	"github.com/xirelogy/go-flux/internal/lexer"
	"github.com/xirelogy/go-flux/internal/parser"
	"github.com/xirelogy/go-flux/internal/vm"
)

func compileModule(t *testing.T, src string) *compiler.Module {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	mod, err := compiler.Compile(prog, "test")
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	return mod
}

func runFunction(t *testing.T, src, entry string, args []vm.Value) vm.Value {
	t.Helper()
	mod := compileModule(t, src)
	fn := mod.Functions[entry]
	if fn == nil {
		t.Fatalf("entry function %s not found (available: %v)", entry, keys(mod.Functions))
	}
	machine := vm.New()
	machine.LoadModule(mod)
	val, err := machine.Call(entry, args)
	if err != nil {
		t.Fatalf("vm call error: %v (code=%v)", err, fn.Chunk.Code)
	}
	return val
}

func TestVMFunctionCall(t *testing.T) {
	src := `func add($a, $b) { return $a + $b }`
	v := runFunction(t, src, "add", []vm.Value{vm.Number(2), vm.Number(3)})
	if v.Kind != vm.KindNumber || v.Num != 5 {
		t.Fatalf("expected 5, got %#v", v)
	}
}

func TestVMRangeForLoop(t *testing.T) {
	src := `
func sum() {
  $s := 0
  for ($v in [0 .. 3]) {
    $s = $s + $v
  }
  return $s
}`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	fnNode, ok := prog.Statements[0].(*ast.FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl, got %T", prog.Statements[0])
	}
	if len(fnNode.Body.Statements) != 3 {
		t.Fatalf("expected 3 statements in body, got %d", len(fnNode.Body.Statements))
	}
	mod := compileModule(t, src)
	fn := mod.Functions["sum"]
	machine := vm.New()
	machine.LoadModule(mod)
	v, err := machine.Call("sum", nil)
	if err != nil {
		t.Fatalf("vm call error: %v", err)
	}
	if v.Kind != vm.KindNumber || v.Num != 6 {
		t.Fatalf("expected 6, got %#v (code=%v consts=%v)", v, fn.Chunk.Code, fn.Chunk.Consts)
	}
}

func TestVMObjectKeyValueLoop(t *testing.T) {
	src := `
func copyObj() {
  $o := { a: 1, b: 2 }
  $out := {}
  for ([$k, $v] in $o) {
    $out[$k] = $v
  }
  return $out
}`
	v := runFunction(t, src, "copyObj", nil)
	if v.Kind != vm.KindObject {
		t.Fatalf("expected object, got %#v", v)
	}
	if len(v.Obj) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(v.Obj))
	}
	if v.Obj["a"].Kind != vm.KindNumber || v.Obj["a"].Num != 1 {
		t.Fatalf("expected a:1, got %#v", v.Obj["a"])
	}
	if v.Obj["b"].Kind != vm.KindNumber || v.Obj["b"].Num != 2 {
		t.Fatalf("expected b:2, got %#v", v.Obj["b"])
	}
}

func TestVMClosureUpvalue(t *testing.T) {
	src := `
func makeAdder($x) {
  return func ($y) { return $x + $y }
}

func run() {
  $f := makeAdder(2)
  return $f(5)
}`
	mod := compileModule(t, src)
	makeAdder := mod.Functions["makeAdder"]
	runFn := mod.Functions["run"]
	if makeAdder == nil || runFn == nil {
		t.Fatalf("functions missing: makeAdder=%v run=%v", makeAdder != nil, runFn != nil)
	}
	if inner, ok := makeAdder.Chunk.Consts[0].(*compiler.Prototype); ok {
		t.Logf("inner code=%v consts=%v upvalues=%v maxLocals=%d", inner.Chunk.Code, inner.Chunk.Consts, inner.Upvalues, inner.MaxLocals)
	}
	t.Logf("makeAdder code=%v consts=%v", makeAdder.Chunk.Code, makeAdder.Chunk.Consts)
	t.Logf("run code=%v consts=%v", runFn.Chunk.Code, runFn.Chunk.Consts)
	machine := vm.New()
	machine.LoadModule(mod)
	v, err := machine.Call("run", nil)
	if err != nil {
		t.Fatalf("vm call error: %v", err)
	}
	if v.Kind != vm.KindNumber || v.Num != 7 {
		t.Fatalf("expected 7, got %#v", v)
	}
}

func TestVMBuiltins(t *testing.T) {
	cases := []struct {
		name string
		src  string
		test func(vm.Value) bool
		desc string
	}{
		{
			name: "typeof",
			src:  `func demo() { return typeof(123) }`,
			test: func(v vm.Value) bool { return v.Kind == vm.KindString && v.Str == "number" },
			desc: "expected typeof to return number",
		},
		{
			name: "indexExist",
			src:  `func demo() { return indexExist([1], 0) }`,
			test: func(v vm.Value) bool { return v.Kind == vm.KindBool && v.B },
			desc: "expected indexExist to return true",
		},
		{
			name: "indexRead",
			src:  `func demo() { return indexRead([1], 5, "def") }`,
			test: func(v vm.Value) bool { return v.Kind == vm.KindString && v.Str == "def" },
			desc: "expected indexRead to return default",
		},
		{
			name: "valueExist",
			src:  `func demo() { return valueExist([1, 2, 3], 2) }`,
			test: func(v vm.Value) bool { return v.Kind == vm.KindBool && v.B },
			desc: "expected valueExist to find value",
		},
		{
			name: "error",
			src:  `func demo() { return error("boom") }`,
			test: func(v vm.Value) bool { return v.Kind == vm.KindError && v.Err == "boom" },
			desc: "expected error builtin",
		},
	}

	for _, tc := range cases {
		v := runFunction(t, tc.src, "demo", nil)
		if !tc.test(v) {
			t.Fatalf("%s: %s, got %#v", tc.name, tc.desc, v)
		}
	}
}

func TestVMBuiltinsIndividual(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		validate func(t *testing.T, v vm.Value)
	}{
		{
			name: "typeof",
			src:  `func demo() { return typeof(123) }`,
			validate: func(t *testing.T, v vm.Value) {
				if v.Kind != vm.KindString || v.Str != "number" {
					t.Fatalf("typeof result mismatch: %#v", v)
				}
			},
		},
		{
			name: "error",
			src:  `func demo() { return error("boom") }`,
			validate: func(t *testing.T, v vm.Value) {
				if v.Kind != vm.KindError || v.Err != "boom" {
					t.Fatalf("error result mismatch: %#v", v)
				}
			},
		},
		{
			name: "indexExist",
			src:  `func demo() { return indexExist([1], 0) }`,
			validate: func(t *testing.T, v vm.Value) {
				if v.Kind != vm.KindBool || !v.B {
					t.Fatalf("indexExist mismatch: %#v", v)
				}
			},
		},
		{
			name: "indexRead",
			src:  `func demo() { return indexRead([1], 5, "def") }`,
			validate: func(t *testing.T, v vm.Value) {
				if v.Kind != vm.KindString || v.Str != "def" {
					t.Fatalf("indexRead mismatch: %#v", v)
				}
			},
		},
		{
			name: "valueExist",
			src:  `func demo() { return valueExist([1, 2, 3], 2) }`,
			validate: func(t *testing.T, v vm.Value) {
				if v.Kind != vm.KindBool || !v.B {
					t.Fatalf("valueExist mismatch: %#v", v)
				}
			},
		},
		{
			name: "readonly false by default",
			src:  `func demo() { return readonly({}) }`,
			validate: func(t *testing.T, v vm.Value) {
				if v.Kind != vm.KindBool || v.B {
					t.Fatalf("readonly default mismatch: %#v", v)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := runFunction(t, tt.src, "demo", nil)
			tt.validate(t, v)
		})
	}
}

func TestVMReadonlyBuiltinTrue(t *testing.T) {
	src := `func demo($o) { return readonly($o) }`
	obj := vm.Object(map[string]vm.Value{"a": vm.Number(1)})
	obj.ReadOnly = true
	v := runFunction(t, src, "demo", []vm.Value{obj})
	if v.Kind != vm.KindBool || !v.B {
		t.Fatalf("readonly expected true, got %#v", v)
	}
}

func TestVMReadonlyPreventsMutation(t *testing.T) {
	src := `
func mutate($o, $a) {
  $o.a = 2
  $a[0] = 9
}`
	obj := vm.Object(map[string]vm.Value{"a": vm.Number(1)})
	obj.ReadOnly = true
	arr := vm.Array([]vm.Value{vm.Number(1)})
	arr.ReadOnly = true
	mod := compileModule(t, src)
	machine := vm.New()
	machine.LoadModule(mod)
	if _, err := machine.Call("mutate", []vm.Value{obj, arr}); err == nil {
		t.Fatalf("expected mutation to fail on read-only values")
	}
}

func TestVMHandlesNop(t *testing.T) {
	src := `func demo() { return 42 }`
	mod := compileModule(t, src)
	fn := mod.Functions["demo"]
	if fn == nil {
		t.Fatalf("demo not found")
	}
	// Prepend an OP_NOP to ensure interpreter skips it.
	fn.Chunk.Code = append([]byte{compiler.OP_NOP}, fn.Chunk.Code...)

	machine := vm.New()
	machine.LoadModule(mod)
	v, err := machine.Call("demo", nil)
	if err != nil {
		t.Fatalf("call error: %v", err)
	}
	if v.Kind != vm.KindNumber || v.Num != 42 {
		t.Fatalf("expected 42, got %#v", v)
	}
}

func keys(m map[string]*compiler.Prototype) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
