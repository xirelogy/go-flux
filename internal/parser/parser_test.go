package parser

import (
	"testing"

	"github.com/xirelogy/go-flux/internal/ast"
	"github.com/xirelogy/go-flux/internal/lexer"
	"github.com/xirelogy/go-flux/internal/token"
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

func TestParseIfCallCondition(t *testing.T) {
	input := `if (_callFunction(1, 2) > 2) { return 1 }`
	p := New(lexer.New(input))
	prog := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	stmt, ok := prog.Statements[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected IfStmt, got %T", prog.Statements[0])
	}
	if stmt.Condition == nil {
		t.Fatalf("expected condition")
	}
	cond, ok := stmt.Condition.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr condition, got %T", stmt.Condition)
	}
	if cond.Operator != token.Greater {
		t.Fatalf("expected '>' operator, got %v", cond.Operator)
	}
	call, ok := cond.Left.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected call on left, got %T", cond.Left)
	}
	ident, ok := call.Callee.(*ast.Identifier)
	if !ok || ident.Name != "_callFunction" {
		t.Fatalf("expected callee _callFunction, got %T (%v)", call.Callee, ident)
	}
	if len(call.Arguments) != 2 {
		t.Fatalf("expected 2 args, got %d", len(call.Arguments))
	}
	if _, ok := cond.Right.(*ast.NumberLiteral); !ok {
		t.Fatalf("expected number literal on right, got %T", cond.Right)
	}
}

func TestParseInvalidOperator(t *testing.T) {
	input := `func bad($c) { $c->clear() }`
	p := New(lexer.New(input))
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors")
	}
}

func TestParseCallMissingRParen(t *testing.T) {
	input := `func bad($c) { inc(1, 2 }`
	p := New(lexer.New(input))
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors")
	}
}

func TestParseCallTrailingComma(t *testing.T) {
	input := `func bad($c) { inc(1,) }`
	p := New(lexer.New(input))
	_ = p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatalf("expected parser errors")
	}
}
