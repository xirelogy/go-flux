package lexer

import (
	"strings"

	"github.com/xirelogy/go-flux/internal/token"
)

// Lexer converts source text into a stream of tokens.
type Lexer struct {
	input        string
	pos          int  // current position in bytes
	readPos      int  // next read position
	ch           byte // current char
	line         int
	column       int
	parenDepth   int
	bracketDepth int
	lastToken    token.Type
}

// New creates a lexer for the provided source text.
func New(input string) *Lexer {
	l := &Lexer{
		input:     input,
		line:      1,
		column:    0,
		lastToken: token.Newline, // treat start as newline boundary
	}
	l.readChar()
	return l
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() token.Token {
	for {
		l.skipWhitespace()

		if l.ch == '\n' {
			if tok, ok := l.consumeNewline(); ok {
				return tok
			}
			continue
		}

		if l.ch == 0 {
			return l.makeToken(token.EOF, "")
		}

		if l.ch == '/' {
			if l.peekChar() == '/' {
				l.skipLineComment()
				continue
			}
			if l.peekChar() == '*' {
				l.skipBlockComment()
				continue
			}
		}

		switch l.ch {
		case '=':
			if l.peekChar() == '=' {
				ch := l.ch
				l.readChar()
				tok := l.makeToken(token.Equal, string(ch)+string(l.ch))
				l.readChar()
				return l.finishToken(tok)
			}
			tok := l.makeToken(token.Assign, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case ':':
			if l.peekChar() == '=' {
				ch := l.ch
				l.readChar()
				tok := l.makeToken(token.Define, string(ch)+string(l.ch))
				l.readChar()
				return l.finishToken(tok)
			}
			tok := l.makeToken(token.Colon, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '+':
			tok := l.makeToken(token.Plus, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '-':
			tok := l.makeToken(token.Minus, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '*':
			tok := l.makeToken(token.Star, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '/':
			tok := l.makeToken(token.Slash, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '!':
			if l.peekChar() == '=' {
				ch := l.ch
				l.readChar()
				tok := l.makeToken(token.NotEqual, string(ch)+string(l.ch))
				l.readChar()
				return l.finishToken(tok)
			}
			tok := l.makeToken(token.Bang, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '<':
			if l.peekChar() == '=' {
				ch := l.ch
				l.readChar()
				tok := l.makeToken(token.LessEqual, string(ch)+string(l.ch))
				l.readChar()
				return l.finishToken(tok)
			}
			tok := l.makeToken(token.Less, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '>':
			if l.peekChar() == '=' {
				ch := l.ch
				l.readChar()
				tok := l.makeToken(token.GreaterEqual, string(ch)+string(l.ch))
				l.readChar()
				return l.finishToken(tok)
			}
			tok := l.makeToken(token.Greater, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '&':
			if l.peekChar() == '&' {
				ch := l.ch
				l.readChar()
				tok := l.makeToken(token.AndAnd, string(ch)+string(l.ch))
				l.readChar()
				return l.finishToken(tok)
			}
			tok := l.makeToken(token.Illegal, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '|':
			if l.peekChar() == '|' {
				ch := l.ch
				l.readChar()
				tok := l.makeToken(token.OrOr, string(ch)+string(l.ch))
				l.readChar()
				return l.finishToken(tok)
			}
			tok := l.makeToken(token.Illegal, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '.':
			if l.peekChar() == '.' {
				ch := l.ch
				l.readChar()
				tok := l.makeToken(token.Range, string(ch)+string(l.ch))
				l.readChar()
				return l.finishToken(tok)
			}
			tok := l.makeToken(token.Dot, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case ',':
			tok := l.makeToken(token.Comma, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '(':
			tok := l.makeToken(token.LParen, string(l.ch))
			l.readChar()
			l.parenDepth++
			return l.finishToken(tok)
		case ')':
			tok := l.makeToken(token.RParen, string(l.ch))
			l.readChar()
			if l.parenDepth > 0 {
				l.parenDepth--
			}
			return l.finishToken(tok)
		case '[':
			tok := l.makeToken(token.LBracket, string(l.ch))
			l.readChar()
			l.bracketDepth++
			return l.finishToken(tok)
		case ']':
			tok := l.makeToken(token.RBracket, string(l.ch))
			l.readChar()
			if l.bracketDepth > 0 {
				l.bracketDepth--
			}
			return l.finishToken(tok)
		case '{':
			tok := l.makeToken(token.LBrace, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '}':
			tok := l.makeToken(token.RBrace, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		case '"':
			return l.readString()
		case '$':
			return l.readVariable()
		default:
			if isLetter(l.ch) {
				return l.readIdentifier()
			}
			if isDigit(l.ch) {
				return l.readNumber()
			}

			tok := l.makeToken(token.Illegal, string(l.ch))
			l.readChar()
			return l.finishToken(tok)
		}
	}
}

func (l *Lexer) makeToken(t token.Type, lit string) token.Token {
	return token.Token{
		Type:    t,
		Literal: lit,
		Pos: token.Position{
			Offset: l.pos,
			Line:   l.line,
			Column: l.column,
		},
	}
}

func (l *Lexer) finishToken(tok token.Token) token.Token {
	l.lastToken = tok.Type
	return tok
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) consumeNewline() (token.Token, bool) {
	pos := l.makeToken(token.Newline, "")
	l.readChar()

	if l.parenDepth == 0 && l.bracketDepth == 0 && newlineEligible(l.lastToken) {
		l.lastToken = token.Newline
		return pos, true
	}
	return token.Token{}, false
}

func (l *Lexer) skipLineComment() {
	for l.ch != 0 && l.ch != '\n' {
		l.readChar()
	}
}

func (l *Lexer) skipBlockComment() {
	l.readChar() // consume '/'
	l.readChar() // consume '*'
	for {
		if l.ch == 0 {
			return
		}
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar() // '*'
			l.readChar() // '/'
			return
		}
		l.readChar()
	}
}

func (l *Lexer) readIdentifier() token.Token {
	start := l.makeToken(token.Ident, "")
	var sb strings.Builder
	for isLetter(l.ch) || isDigit(l.ch) {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	lit := sb.String()
	tokType := token.LookupIdent(lit)
	start.Type = tokType
	start.Literal = lit
	return l.finishToken(start)
}

func (l *Lexer) readVariable() token.Token {
	start := l.makeToken(token.Variable, "")
	l.readChar() // consume '$'

	if !isLetter(l.ch) {
		illegal := l.makeToken(token.Illegal, "$")
		l.lastToken = token.Illegal
		return illegal
	}

	var sb strings.Builder
	for isLetter(l.ch) || isDigit(l.ch) {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	start.Literal = sb.String()
	return l.finishToken(start)
}

func (l *Lexer) readNumber() token.Token {
	start := l.makeToken(token.Number, "")
	var sb strings.Builder
	for isDigit(l.ch) {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	if l.ch == '.' && isDigit(l.peekChar()) {
		sb.WriteByte(l.ch)
		l.readChar()
		for isDigit(l.ch) {
			sb.WriteByte(l.ch)
			l.readChar()
		}
	}
	start.Literal = sb.String()
	return l.finishToken(start)
}

func (l *Lexer) readString() token.Token {
	start := l.makeToken(token.String, "")
	var sb strings.Builder

	for {
		l.readChar()
		if l.ch == 0 {
			illegal := l.makeToken(token.Illegal, "unterminated string")
			l.lastToken = token.Illegal
			return illegal
		}
		if l.ch == '"' {
			l.readChar()
			break
		}
		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case '"', '\\':
				sb.WriteByte(l.ch)
			case 'n':
				sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			case 't':
				sb.WriteByte('\t')
			case 'b':
				sb.WriteByte('\b')
			case 'f':
				sb.WriteByte('\f')
			default:
				sb.WriteByte(l.ch)
			}
			continue
		}
		sb.WriteByte(l.ch)
	}

	start.Literal = sb.String()
	return l.finishToken(start)
}

func newlineEligible(t token.Type) bool {
	switch t {
	case token.Ident, token.Variable, token.Number, token.String,
		token.True, token.False, token.Null,
		token.RParen, token.RBracket, token.RBrace,
		token.Return:
		return true
	default:
		return false
	}
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.pos = l.readPos
		l.ch = 0
		return
	}

	l.ch = l.input[l.readPos]
	l.pos = l.readPos
	l.readPos++

	if l.ch == '\n' {
		l.line++
		l.column = 0
	} else {
		l.column++
	}
}
