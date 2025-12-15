package compiler

import (
	"testing"

	"github.com/xirelogy/go-flux/internal/lexer"
	"github.com/xirelogy/go-flux/internal/parser"
	"github.com/xirelogy/go-flux/internal/runtime"
)

func compileSource(t *testing.T, src string) *Module {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	mod, err := Compile(prog, "test")
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	return mod
}

func TestCompileSimpleFunction(t *testing.T) {
	src := `func add($a, $b) {
  return $a + $b
}`
	mod := compileSource(t, src)
	fn := mod.Functions["add"]
	if fn == nil {
		t.Fatalf("function add not found")
	}
	expectedOps := []byte{
		OP_GET_LOCAL, 0x00,
		OP_GET_LOCAL, 0x01,
		OP_ADD,
		OP_RETURN,
	}
	if len(fn.Chunk.Code) != len(expectedOps) {
		t.Fatalf("expected code length %d, got %d", len(expectedOps), len(fn.Chunk.Code))
	}
	for i, b := range expectedOps {
		if fn.Chunk.Code[i] != b {
			t.Fatalf("byte %d expected %02x got %02x", i, b, fn.Chunk.Code[i])
		}
	}
}

func TestCompileArrayLiteral(t *testing.T) {
	src := `func make() {
  $x := [1, 2]
  return $x
}`
	mod := compileSource(t, src)
	fn := mod.Functions["make"]
	if fn == nil {
		t.Fatalf("function make not found")
	}
	// expect const 1, const 2, array(2), set_local, return
	code := fn.Chunk.Code
	if code[0] != OP_CONST {
		t.Fatalf("expected OP_CONST at 0")
	}
	if code[3] != OP_CONST {
		t.Fatalf("expected OP_CONST at 3")
	}
	if code[6] != OP_ARRAY || code[7] != 0x00 || code[8] != 0x02 {
		t.Fatalf("expected OP_ARRAY count 2")
	}
}

func TestCompileLogical(t *testing.T) {
	src := `func demo($a, $b) { return $a && $b }`
	mod := compileSource(t, src)
	fn := mod.Functions["demo"]
	if fn == nil {
		t.Fatalf("function demo not found")
	}
	if len(fn.Chunk.Code) == 0 {
		t.Fatalf("no code")
	}
}

func TestCompileBuiltins(t *testing.T) {
	src := `func demo($x) {
 	  return typeof($x)
	}`
	mod := compileSource(t, src)
	fn := mod.Functions["demo"]
	if fn == nil {
		t.Fatalf("function demo not found")
	}
	spec, _ := runtime.LookupByName("typeof")
	found := false
	for _, b := range fn.Chunk.Code {
		if b == spec.Opcode {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected OP_TYPEOF in bytecode")
	}
}

func TestCompileConditionals(t *testing.T) {
	src := `func demo($x) {
  if ($x > 1) { return $x }
  else { return 0 }
}`
	mod := compileSource(t, src)
	if mod.Functions["demo"] == nil {
		t.Fatalf("function demo not found")
	}
}

func TestCompileObjectAndIndexing(t *testing.T) {
	src := `func demo() {
  $o := { a: 1, b: 2 }
  $o.a = 3
  return $o["a"]
}`
	mod := compileSource(t, src)
	if mod.Functions["demo"] == nil {
		t.Fatalf("function demo not found")
	}
}

func TestCompileRangeLiteral(t *testing.T) {
	src := `func demo() { return [0 .. 3] }`
	mod := compileSource(t, src)
	if mod.Functions["demo"] == nil {
		t.Fatalf("function demo not found")
	}
}

func TestCompileErrorBuiltin(t *testing.T) {
	src := `func demo() { return error("boom") }`
	mod := compileSource(t, src)
	if mod.Functions["demo"] == nil {
		t.Fatalf("function demo not found")
	}
}
