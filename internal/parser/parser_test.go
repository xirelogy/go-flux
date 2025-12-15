package parser

import (
	"testing"

	"github.com/xirelogy/go-flux/internal/ast"
	"github.com/xirelogy/go-flux/internal/lexer"
)

func TestParseReturnAndExpr(t *testing.T) {
	input := `return 5
$a := 10 + 2`

	p := New(lexer.New(input))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	if len(prog.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Statements))
	}
	_, ok := prog.Statements[0].(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("expected ReturnStmt, got %T", prog.Statements[0])
	}
	if _, ok := prog.Statements[1].(*ast.ExprStmt); !ok {
		t.Fatalf("expected ExprStmt, got %T", prog.Statements[1])
	}
}

func TestParseForIn(t *testing.T) {
	input := `for ([$k, $v] in $obj) {
  $sum = $sum + $v
}`
	p := New(lexer.New(input))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	forStmt, ok := prog.Statements[0].(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected ForStmt, got %T", prog.Statements[0])
	}
	if forStmt.Binding.Key != "k" || forStmt.Binding.ValueName != "v" {
		t.Fatalf("binding mismatch: %v", forStmt.Binding)
	}
	if forStmt.Iterable == nil || forStmt.Body == nil {
		t.Fatalf("missing iterable or body")
	}
}

func TestParseRangeLiteral(t *testing.T) {
	input := `[$start .. $end]`
	p := New(lexer.New(input))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	stmt, ok := prog.Statements[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected ExprStmt, got %T", prog.Statements[0])
	}
	_, ok = stmt.Expression.(*ast.RangeLiteral)
	if !ok {
		t.Fatalf("expected RangeLiteral, got %T", stmt.Expression)
	}
}

func TestParseFunctionLiteral(t *testing.T) {
	input := `func add($a, $b) {
  return $a + $b
}`
	p := New(lexer.New(input))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	fn, ok := prog.Statements[0].(*ast.FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl, got %T", prog.Statements[0])
	}
	if fn.Name != "add" || len(fn.Params) != 2 {
		t.Fatalf("unexpected func signature: %s %d params", fn.Name, len(fn.Params))
	}
	if fn.Body == nil || len(fn.Body.Statements) != 1 {
		t.Fatalf("unexpected body")
	}
}
