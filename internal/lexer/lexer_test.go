package lexer

import (
	"testing"

	"github.com/xirelogy/go-flux/internal/token"
)

func TestLexerBasicTokens(t *testing.T) {
	input := `
func add($a, $b) {
  $c := $a + $b
  if ($c >= 10 && $a != $b) {
    return $c
  }
}
`

	tests := []token.Token{
		{Type: token.Func, Literal: "func"},
		{Type: token.Ident, Literal: "add"},
		{Type: token.LParen, Literal: "("},
		{Type: token.Variable, Literal: "a"},
		{Type: token.Comma, Literal: ","},
		{Type: token.Variable, Literal: "b"},
		{Type: token.RParen, Literal: ")"},
		{Type: token.LBrace, Literal: "{"},
		{Type: token.Variable, Literal: "c"},
		{Type: token.Define, Literal: ":="},
		{Type: token.Variable, Literal: "a"},
		{Type: token.Plus, Literal: "+"},
		{Type: token.Variable, Literal: "b"},
		{Type: token.Newline},
		{Type: token.If, Literal: "if"},
		{Type: token.LParen, Literal: "("},
		{Type: token.Variable, Literal: "c"},
		{Type: token.GreaterEqual, Literal: ">="},
		{Type: token.Number, Literal: "10"},
		{Type: token.AndAnd, Literal: "&&"},
		{Type: token.Variable, Literal: "a"},
		{Type: token.NotEqual, Literal: "!="},
		{Type: token.Variable, Literal: "b"},
		{Type: token.RParen, Literal: ")"},
		{Type: token.LBrace, Literal: "{"},
		{Type: token.Return, Literal: "return"},
		{Type: token.Variable, Literal: "c"},
		{Type: token.Newline},
		{Type: token.RBrace, Literal: "}"},
		{Type: token.Newline},
		{Type: token.RBrace, Literal: "}"},
		{Type: token.Newline},
		{Type: token.EOF},
	}

	l := New(input)
	for i, expected := range tests {
		tok := l.NextToken()
		if tok.Type != expected.Type || tok.Literal != expected.Literal {
			t.Fatalf("token %d: expected %v %q, got %v %q", i, expected.Type, expected.Literal, tok.Type, tok.Literal)
		}
	}
}

func TestLexerRangeAndIndexing(t *testing.T) {
	input := `[0 .. 3]
$arr[0] = indexRead($obj, "missing", "fallback")`

	expectedTypes := []token.Type{
		token.LBracket, token.Number, token.Range, token.Number, token.RBracket, token.Newline,
		token.Variable, token.LBracket, token.Number, token.RBracket, token.Assign,
		token.Ident, token.LParen, token.Variable, token.Comma, token.String, token.Comma, token.String, token.RParen,
		token.EOF,
	}

	l := New(input)
	for i, typ := range expectedTypes {
		tok := l.NextToken()
		if tok.Type != typ {
			t.Fatalf("token %d: expected %v, got %v (%q)", i, typ, tok.Type, tok.Literal)
		}
	}
}

func TestLexerNewlineSuppression(t *testing.T) {
	input := `$a := (
  1 +
  2)
$b := [1,
 2]
valueExist($b, 2)
`

	expected := []token.Type{
		token.Variable, token.Define, token.LParen, token.Number, token.Plus, token.Number, token.RParen, token.Newline,
		token.Variable, token.Define, token.LBracket, token.Number, token.Comma, token.Number, token.RBracket, token.Newline,
		token.Ident, token.LParen, token.Variable, token.Comma, token.Number, token.RParen, token.Newline,
		token.EOF,
	}

	l := New(input)
	for i, typ := range expected {
		tok := l.NextToken()
		if tok.Type != typ {
			t.Fatalf("token %d: expected %v, got %v (%q)", i, typ, tok.Type, tok.Literal)
		}
	}
}

func TestLexerComments(t *testing.T) {
	input := `// line comment
$a := 1
/* block
comment */
$b := 2`

	expected := []token.Type{
		token.Variable, token.Define, token.Number, token.Newline,
		token.Variable, token.Define, token.Number, token.EOF,
	}

	l := New(input)
	for i, typ := range expected {
		tok := l.NextToken()
		if tok.Type != typ {
			t.Fatalf("token %d: expected %v, got %v (%q)", i, typ, tok.Type, tok.Literal)
		}
	}
}
