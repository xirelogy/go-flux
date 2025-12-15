package token

// Type identifies the category of a token.
type Type string

// Token carries the lexical item along with its source position.
type Token struct {
	Type    Type
	Literal string
	Pos     Position
}

// Position describes a byte offset and 1-based line/column.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Span represents an inclusive start and end position for a node.
type Span struct {
	Start Position
	End   Position
}

const (
	Illegal Type = "ILLEGAL"
	EOF     Type = "EOF"
	Newline Type = "NEWLINE"

	// identifiers and literals
	Ident    Type = "IDENT"
	Variable Type = "VAR"
	Number   Type = "NUMBER"
	String   Type = "STRING"

	// keywords
	If      Type = "IF"
	ElseIf  Type = "ELSEIF"
	Else    Type = "ELSE"
	While   Type = "WHILE"
	For     Type = "FOR"
	In      Type = "IN"
	Func    Type = "FUNC"
	Return  Type = "RETURN"
	True    Type = "TRUE"
	False   Type = "FALSE"
	Null    Type = "NULL"
	Yield   Type = "YIELD"
	Iterate Type = "ITERATE"
	Using   Type = "USING"

	// operators
	Assign       Type = "ASSIGN"       // =
	Define       Type = "DEFINE"       // :=
	Plus         Type = "PLUS"         // +
	Minus        Type = "MINUS"        // -
	Star         Type = "STAR"         // *
	Slash        Type = "SLASH"        // /
	Bang         Type = "BANG"         // !
	Equal        Type = "EQUAL"        // ==
	NotEqual     Type = "NOTEQUAL"     // !=
	Less         Type = "LESS"         // <
	LessEqual    Type = "LESSEQUAL"    // <=
	Greater      Type = "GREATER"      // >
	GreaterEqual Type = "GREATEREQUAL" // >=
	AndAnd       Type = "ANDAND"       // &&
	OrOr         Type = "OROR"         // ||
	Range        Type = "RANGE"        // ..

	// delimiters
	Comma    Type = "COMMA"
	Colon    Type = "COLON"
	Dot      Type = "DOT"
	LParen   Type = "LPAREN"
	RParen   Type = "RPAREN"
	LBrace   Type = "LBRACE"
	RBrace   Type = "RBRACE"
	LBracket Type = "LBRACKET"
	RBracket Type = "RBRACKET"
)

var keywords = map[string]Type{
	"if":      If,
	"elseif":  ElseIf,
	"else":    Else,
	"while":   While,
	"for":     For,
	"in":      In,
	"func":    Func,
	"return":  Return,
	"true":    True,
	"false":   False,
	"null":    Null,
	"yield":   Yield,
	"iterate": Iterate,
	"using":   Using,
}

// LookupIdent returns the keyword token type or Ident.
func LookupIdent(ident string) Type {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return Ident
}
