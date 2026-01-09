package parser

import (
	"fmt"

	"github.com/xirelogy/go-flux/internal/ast"
	"github.com/xirelogy/go-flux/internal/lexer"
	"github.com/xirelogy/go-flux/internal/token"
)

type Parser struct {
	l         *lexer.Lexer
	curToken  token.Token
	peekToken token.Token
	errors    []string
	prevToken token.Token
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}
	// Read two tokens, so curToken and peekToken are set
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) nextToken() {
	p.prevToken = p.curToken
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseProgram() *ast.Program {
	prog := &ast.Program{}

	for p.curToken.Type != token.EOF {
		p.skipNewlines()
		if p.curToken.Type == token.EOF {
			break
		}
		stmt := p.parseStatement()
		if stmt != nil {
			prog.Statements = append(prog.Statements, stmt)
		}
		p.skipNewlines()
	}
	if len(prog.Statements) > 0 {
		prog.NodeSpan = token.Span{Start: prog.Statements[0].Span().Start, End: prog.Statements[len(prog.Statements)-1].Span().End}
	}
	return prog
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.Func:
		return p.parseFuncDecl()
	case token.Return:
		return p.parseReturn()
	case token.If:
		return p.parseIf()
	case token.While:
		return p.parseWhile()
	case token.For:
		return p.parseFor()
	case token.LBrace:
		return p.parseBlock()
	default:
		return p.parseExprStatement()
	}
}

func (p *Parser) parseBlock() ast.Statement {
	block := &ast.BlockStmt{LBrace: p.curToken.Pos}
	p.nextToken()
	p.skipNewlines()
	for p.curToken.Type != token.RBrace && p.curToken.Type != token.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.skipNewlines()
	}
	end := block.LBrace
	if p.curToken.Type == token.RBrace {
		end = p.curToken.Pos
		p.nextToken()
	} else if len(block.Statements) > 0 {
		end = block.Statements[len(block.Statements)-1].Span().End
	}
	block.BlockSpan = token.Span{Start: block.LBrace, End: end}
	return block
}

func (p *Parser) parseReturn() ast.Statement {
	ret := &ast.ReturnStmt{Return: p.curToken.Pos}
	p.nextToken()
	if !p.isEndOfStatement(p.curToken.Type) {
		ret.Value = p.parseExpression(assignPrecedence - 1)
	} else {
		// keep curToken where it is (newline/EOF/RBrace)
	}
	end := ret.Return
	if ret.Value != nil {
		end = ret.Value.Span().End
	}
	ret.StmtSpan = token.Span{Start: ret.Return, End: end}
	// advance past the expression terminator (newline or next token)
	if p.curToken.Type != token.EOF {
		p.nextToken()
	}
	return ret
}

func (p *Parser) parseIf() ast.Statement {
	stmt := &ast.IfStmt{IfPos: p.curToken.Pos}
	if !p.expectPeek(token.LParen) {
		return nil
	}
	p.nextToken()
	stmt.Condition = p.parseExpression(lowest)
	p.consumeRParen()
	p.skipNewlines()
	if p.curToken.Type != token.LBrace && p.peekToken.Type == token.LBrace {
		p.nextToken()
	}
	body := p.parseBlock()
	if blk, ok := body.(*ast.BlockStmt); ok {
		stmt.Conseq = blk
	}

	p.skipNewlines()
	for p.curToken.Type == token.ElseIf {
		clause := ElseIfClause{}
		clause.Pos = p.curToken.Pos
		if !p.expectPeek(token.LParen) {
			return stmt
		}
		p.nextToken()
		clause.Condition = p.parseExpression(lowest)
		p.consumeRParen()
		p.skipNewlines()
		if p.curToken.Type != token.LBrace && p.peekToken.Type == token.LBrace {
			p.nextToken()
		}
		body := p.parseBlock()
		if blk, ok := body.(*ast.BlockStmt); ok {
			clause.Conseq = blk
		}
		stmt.ElseIfs = append(stmt.ElseIfs, clause)
		p.skipNewlines()
	}

	if p.curToken.Type == token.Else {
		p.nextToken()
		p.skipNewlines()
		if p.curToken.Type != token.LBrace && p.peekToken.Type == token.LBrace {
			p.nextToken()
		}
		body := p.parseBlock()
		if blk, ok := body.(*ast.BlockStmt); ok {
			stmt.Alt = blk
		}
	}
	end := stmt.IfPos
	if stmt.Alt != nil {
		end = stmt.Alt.Span().End
	} else if len(stmt.ElseIfs) > 0 {
		end = stmt.ElseIfs[len(stmt.ElseIfs)-1].Span.End
	} else if stmt.Conseq != nil {
		end = stmt.Conseq.Span().End
	}
	stmt.IfSpan = token.Span{Start: stmt.IfPos, End: end}
	return stmt
}

type ElseIfClause = ast.ElseIfClause

func (p *Parser) parseWhile() ast.Statement {
	stmt := &ast.WhileStmt{WhilePos: p.curToken.Pos}
	if !p.expectPeek(token.LParen) {
		return nil
	}
	p.nextToken()
	stmt.Condition = p.parseExpression(lowest)
	p.consumeRParen()
	p.skipNewlines()
	if p.curToken.Type != token.LBrace && p.peekToken.Type == token.LBrace {
		p.nextToken()
	}
	body := p.parseBlock()
	if blk, ok := body.(*ast.BlockStmt); ok {
		stmt.Body = blk
	}
	end := stmt.WhilePos
	if stmt.Body != nil {
		end = stmt.Body.Span().End
	}
	stmt.NodeSpan = token.Span{Start: stmt.WhilePos, End: end}
	return stmt
}

func (p *Parser) parseFor() ast.Statement {
	stmt := &ast.ForStmt{ForPos: p.curToken.Pos}
	if !p.expectPeek(token.LParen) {
		return nil
	}
	p.nextToken() // move to '('
	p.nextToken() // move to first token inside parens
	stmt.Binding = p.parseForBinding()
	switch {
	case p.curToken.Type == token.In:
		// already on 'in'
	case p.peekToken.Type == token.In:
		p.nextToken() // advance to 'in'
	default:
		p.errorf(p.curToken.Pos, "expected 'in' in for binding")
		return stmt
	}
	p.nextToken() // move past 'in' to iterable expression
	stmt.Iterable = p.parseExpression(lowest)
	p.consumeRParen()
	p.skipNewlines()
	if p.curToken.Type != token.LBrace && p.peekToken.Type == token.LBrace {
		p.nextToken()
	}
	body := p.parseBlock()
	if blk, ok := body.(*ast.BlockStmt); ok {
		stmt.Body = blk
	}
	end := stmt.ForPos
	if stmt.Body != nil {
		end = stmt.Body.Span().End
	}
	stmt.NodeSpan = token.Span{Start: stmt.ForPos, End: end}
	return stmt
}

func (p *Parser) parseForBinding() ast.ForBinding {
	switch p.curToken.Type {
	case token.Variable:
		name := p.curToken.Literal
		pos := p.curToken.Pos
		p.nextToken()
		return ast.ForBinding{Pos: pos, ValueName: name}
	case token.LBracket:
		lpos := p.curToken.Pos
		if !p.expectPeek(token.Variable) {
			return ast.ForBinding{Pos: lpos}
		}
		p.nextToken()
		key := p.curToken.Literal
		if !p.expectPeek(token.Comma) {
			return ast.ForBinding{Pos: lpos}
		}
		p.nextToken()
		if !p.expectPeek(token.Variable) {
			return ast.ForBinding{Pos: lpos}
		}
		p.nextToken()
		val := p.curToken.Literal
		if !p.expectPeek(token.RBracket) {
			return ast.ForBinding{Pos: lpos}
		}
		p.nextToken()
		return ast.ForBinding{Pos: lpos, Key: key, ValueName: val}
	default:
		p.errorf(p.curToken.Pos, "invalid for binding")
		return ast.ForBinding{Pos: p.curToken.Pos}
	}
}

func (p *Parser) parseFuncDecl() ast.Statement {
	decl := &ast.FuncDecl{FuncPos: p.curToken.Pos}
	if !p.expectPeek(token.Ident) {
		return nil
	}
	p.nextToken()
	decl.Name = p.curToken.Literal
	decl.NamePos = p.curToken.Pos
	if !p.expectPeek(token.LParen) {
		return nil
	}
	p.nextToken() // move to '('
	p.nextToken() // move to first param or ')'
	decl.Params = p.parseParamList()
	p.nextToken() // move past ')'
	p.skipNewlines()
	if p.curToken.Type != token.LBrace && p.peekToken.Type == token.LBrace {
		p.nextToken()
	}
	body := p.parseBlock()
	if blk, ok := body.(*ast.BlockStmt); ok {
		decl.Body = blk
	}
	end := decl.FuncPos
	if decl.Body != nil {
		end = decl.Body.Span().End
	}
	decl.NodeSpan = token.Span{Start: decl.FuncPos, End: end}
	return decl
}

func (p *Parser) parseExprStatement() ast.Statement {
	stmt := &ast.ExprStmt{Start: p.curToken.Pos}
	stmt.Expression = p.parseExpression(lowest)
	if stmt.Expression != nil {
		stmt.StmtSpan = token.Span{Start: stmt.Start, End: stmt.Expression.Span().End}
	}
	// move past the end of the expression to allow outer loop to progress
	if p.curToken.Type != token.EOF {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	var left ast.Expression

	switch p.curToken.Type {
	case token.Ident:
		left = &ast.Identifier{Name: p.curToken.Literal, PosT: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}}
	case token.Variable:
		left = &ast.Variable{Name: p.curToken.Literal, PosT: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}}
	case token.Number:
		left = &ast.NumberLiteral{Value: p.curToken.Literal, PosT: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}}
	case token.String:
		left = &ast.StringLiteral{Value: p.curToken.Literal, PosT: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}}
	case token.True:
		left = &ast.BoolLiteral{Value: true, PosT: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}}
	case token.False:
		left = &ast.BoolLiteral{Value: false, PosT: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}}
	case token.Null:
		left = &ast.NullLiteral{PosT: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}}
	case token.Func:
		left = p.parseFuncExpr()
	case token.LParen:
		p.nextToken()
		left = p.parseExpression(lowest)
		if !p.expectPeek(token.RParen) {
			return nil
		}
		p.nextToken()
	case token.LBracket:
		left = p.parseArrayOrRange()
	case token.LBrace:
		left = p.parseObjectLiteral()
	case token.Bang, token.Minus, token.Plus:
		left = p.parsePrefixExpression()
	default:
		p.errorf(p.curToken.Pos, "unexpected token %s", p.curToken.Type)
		return nil
	}

	if left == nil {
		return nil
	}

	for !p.isEndOfExpression(p.peekToken.Type) && precedence < p.peekPrecedence() {
		op := p.peekToken.Type
		p.nextToken()
		switch op {
		case token.Assign, token.Define:
			left = p.parseAssignExpression(left)
		case token.Plus, token.Minus, token.Star, token.Slash,
			token.Equal, token.NotEqual,
			token.Less, token.LessEqual, token.Greater, token.GreaterEqual,
			token.AndAnd, token.OrOr:
			left = p.parseInfixExpression(left)
		case token.LParen:
			left = p.parseCallExpression(left)
		case token.Dot:
			left = p.parseMemberExpression(left)
		case token.LBracket:
			left = p.parseIndexExpression(left)
		default:
			return left
		}
		if left == nil {
			return nil
		}
	}

	return left
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expr := &ast.UnaryExpr{
		Operator: p.curToken.Type,
		PosT:     p.curToken.Pos,
	}
	p.nextToken()
	expr.Right = p.parseExpression(prefixPrecedence)
	if expr.Right == nil {
		return nil
	}
	expr.Sp = token.Span{Start: expr.PosT, End: expr.Right.Span().End}
	return expr
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expr := &ast.BinaryExpr{
		Left:     left,
		Operator: p.curToken.Type,
		PosT:     p.curToken.Pos,
	}
	precedence := p.curPrecedence()
	p.nextToken()
	expr.Right = p.parseExpression(precedence)
	if expr.Right == nil {
		return nil
	}
	expr.Sp = token.Span{Start: left.Span().Start, End: expr.Right.Span().End}
	return expr
}

func (p *Parser) parseAssignExpression(left ast.Expression) ast.Expression {
	expr := &ast.AssignExpr{
		Left:     left,
		Operator: p.curToken.Type,
		PosT:     p.curToken.Pos,
	}
	p.nextToken()
	expr.Value = p.parseExpression(assignPrecedence - 1)
	if expr.Value != nil {
		expr.Sp = token.Span{Start: left.Span().Start, End: expr.Value.Span().End}
	}
	return expr
}

func (p *Parser) parseCallExpression(callee ast.Expression) ast.Expression {
	expr := &ast.CallExpr{
		Callee: callee,
		PosT:   p.curToken.Pos,
	}
	p.nextToken()
	expr.Arguments = p.parseExpressionList(token.RParen)
	end := expr.PosT
	if len(expr.Arguments) > 0 {
		end = expr.Arguments[len(expr.Arguments)-1].Span().End
	} else if p.curToken.Type == token.RParen {
		end = p.curToken.Pos
	} else {
		end = p.prevToken.Pos
	}
	expr.Sp = token.Span{Start: callee.Span().Start, End: end}
	return expr
}

func (p *Parser) parseMemberExpression(left ast.Expression) ast.Expression {
	pos := p.curToken.Pos
	if !p.expectPeek(token.Ident) {
		return nil
	}
	p.nextToken()
	prop := p.curToken.Literal
	return &ast.MemberExpr{
		Left:     left,
		Property: prop,
		PosT:     pos,
		Sp:       token.Span{Start: left.Span().Start, End: p.curToken.Pos},
	}
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	pos := p.curToken.Pos
	p.nextToken()
	index := p.parseExpression(lowest)
	if !p.expectPeek(token.RBracket) {
		return nil
	}
	p.nextToken()
	return &ast.IndexExpr{
		Left:  left,
		Index: index,
		PosT:  pos,
		Sp:    token.Span{Start: left.Span().Start, End: pos},
	}
}

func (p *Parser) parseArrayOrRange() ast.Expression {
	startPos := p.curToken.Pos
	p.nextToken()
	// Empty array
	if p.curToken.Type == token.RBracket {
		p.nextToken()
		return &ast.ArrayLiteral{PosT: startPos}
	}

	first := p.parseExpression(lowest)
	// range literal detection relies on peek
	if p.peekToken.Type == token.Range {
		p.nextToken() // move to Range
		p.nextToken() // move to end expression start
		end := p.parseExpression(lowest)
		if p.peekToken.Type != token.RBracket {
			p.errorf(p.curToken.Pos, "expected ']' to close range")
			return &ast.RangeLiteral{Start: first, End: end, PosT: startPos}
		}
		p.nextToken() // move to ']'
		spanEnd := p.curToken.Pos
		return &ast.RangeLiteral{Start: first, End: end, PosT: startPos, Sp: token.Span{Start: startPos, End: spanEnd}}
	}

	elements := []ast.Expression{first}
	for p.peekToken.Type == token.Comma {
		p.nextToken() // move to comma
		p.nextToken() // move to next element
		if p.curToken.Type == token.RBracket {
			break
		}
		elem := p.parseExpression(lowest)
		elements = append(elements, elem)
	}
	if p.curToken.Type == token.RBracket {
		spanEnd := p.curToken.Pos
		p.nextToken()
		return &ast.ArrayLiteral{Elements: elements, PosT: startPos, Sp: token.Span{Start: startPos, End: spanEnd}}
	}
	if p.peekToken.Type != token.RBracket {
		p.errorf(p.curToken.Pos, "expected ']' to close array")
		return &ast.ArrayLiteral{Elements: elements, PosT: startPos}
	}
	p.nextToken() // move to ']'
	spanEnd := p.curToken.Pos
	return &ast.ArrayLiteral{Elements: elements, PosT: startPos, Sp: token.Span{Start: startPos, End: spanEnd}}
}

func (p *Parser) parseObjectLiteral() ast.Expression {
	obj := &ast.ObjectLiteral{PosT: p.curToken.Pos}
	p.nextToken()
	if p.curToken.Type == token.RBrace {
		p.nextToken()
		obj.Sp = token.Span{Start: obj.PosT, End: p.prevToken.Pos}
		return obj
	}
	p.skipNewlines()
	for {
		p.skipNewlines()
		if p.curToken.Type == token.RBrace {
			break
		}
		field := ast.ObjectField{}
		field.Key = p.parseObjectKey()
		p.skipPeekNewlines()
		if !p.expectPeek(token.Colon) {
			return obj
		}
		p.nextToken() // move to ':'
		p.nextToken() // move to value start
		p.skipNewlines()
		field.Value = p.parseExpression(lowest)
		p.skipNewlines()
		p.skipPeekNewlines()
		obj.Fields = append(obj.Fields, field)
		if p.peekToken.Type == token.RBrace {
			p.nextToken() // move to '}'
			break
		}
		if p.peekToken.Type != token.Comma {
			p.errorf(p.curToken.Pos, "expected ',' or '}' in object literal")
			break
		}
		p.nextToken() // move to ','
		p.nextToken() // move to next key or '}'
		if p.curToken.Type == token.RBrace {
			break
		}
	}
	obj.Sp = token.Span{Start: obj.PosT, End: p.prevToken.Pos}
	return obj
}

func (p *Parser) parseObjectKey() ast.ObjectKey {
	switch p.curToken.Type {
	case token.Ident:
		key := ast.ObjectKey{Ident: p.curToken.Literal, PosT: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}}
		return key
	case token.String:
		val := p.curToken.Literal
		return ast.ObjectKey{Str: &val, PosT: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}}
	case token.Number:
		val := p.curToken.Literal
		return ast.ObjectKey{Num: &val, PosT: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}}
	default:
		p.errorf(p.curToken.Pos, "invalid object key")
		return ast.ObjectKey{PosT: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}}
	}
}

func (p *Parser) parseExpressionList(end token.Type) []ast.Expression {
	list := []ast.Expression{}
	if p.curToken.Type == end {
		return list
	}
	for {
		exp := p.parseExpression(lowest)
		if exp == nil {
			return list
		}
		list = append(list, exp)
		if p.peekToken.Type == token.Comma {
			p.nextToken() // move to comma
			p.nextToken() // move to next expression start
			if p.curToken.Type == end {
				p.errorf(p.curToken.Pos, "expected expression")
				return list
			}
			continue
		}
		if p.peekToken.Type == end {
			p.nextToken() // move to end
		}
		if p.curToken.Type != end {
			p.errorf(p.peekToken.Pos, "expected ',' or %s", end)
		}
		break
	}
	return list
}

func (p *Parser) parseParamList() []ast.Param {
	params := []ast.Param{}
	if p.curToken.Type == token.RParen {
		return params
	}
	if p.curToken.Type != token.Variable {
		p.errorf(p.curToken.Pos, "expected parameter")
		return params
	}
	params = append(params, ast.Param{Name: p.curToken.Literal, Pos: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}})
	for p.peekToken.Type == token.Comma {
		p.nextToken()
		p.nextToken()
		if p.curToken.Type != token.Variable {
			p.errorf(p.curToken.Pos, "expected parameter")
			return params
		}
		params = append(params, ast.Param{Name: p.curToken.Literal, Pos: p.curToken.Pos, Sp: token.Span{Start: p.curToken.Pos, End: p.curToken.Pos}})
	}
	return params
}

func (p *Parser) consumeRParen() {
	if p.curToken.Type == token.RParen {
		p.nextToken()
		return
	}
	if p.peekToken.Type == token.RParen {
		p.nextToken() // move to ')'
		p.nextToken() // move past ')'
		return
	}
	p.errorf(p.curToken.Pos, "expected ')'")
}

func (p *Parser) parseFuncExpr() ast.Expression {
	fn := &ast.FuncExpr{FuncPos: p.curToken.Pos}
	if !p.expectPeek(token.LParen) {
		return nil
	}
	p.nextToken() // move to '('
	p.nextToken() // move to first param or ')'
	fn.Params = p.parseParamList()
	p.nextToken() // move past ')'
	p.skipNewlines()
	if p.curToken.Type != token.LBrace && p.peekToken.Type == token.LBrace {
		p.nextToken()
	}
	body := p.parseBlock()
	if blk, ok := body.(*ast.BlockStmt); ok {
		fn.Body = blk
	}
	end := fn.FuncPos
	if fn.Body != nil {
		end = fn.Body.Span().End
	}
	fn.Sp = token.Span{Start: fn.FuncPos, End: end}
	return fn
}

func (p *Parser) expectPeek(t token.Type) bool {
	if p.peekToken.Type == t {
		return true
	}
	p.errorf(p.peekToken.Pos, "expected next token to be %s, got %s", t, p.peekToken.Type)
	return false
}

func (p *Parser) peekPrecedence() int {
	if prec, ok := precedences[p.peekToken.Type]; ok {
		return prec
	}
	return lowest
}

func (p *Parser) curPrecedence() int {
	if prec, ok := precedences[p.curToken.Type]; ok {
		return prec
	}
	return lowest
}

func (p *Parser) skipNewlines() {
	for p.curToken.Type == token.Newline {
		p.nextToken()
	}
}

func (p *Parser) skipPeekNewlines() {
	for p.peekToken.Type == token.Newline {
		p.nextToken()
	}
}

func (p *Parser) isEndOfExpression(t token.Type) bool {
	switch t {
	case token.Newline, token.RBrace, token.EOF, token.Comma, token.RParen, token.RBracket:
		return true
	default:
		return false
	}
}

func (p *Parser) isEndOfStatement(t token.Type) bool {
	switch t {
	case token.Newline, token.RBrace, token.EOF:
		return true
	default:
		return false
	}
}

func (p *Parser) errorf(pos token.Position, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	p.errors = append(p.errors, fmt.Sprintf("%d:%d: %s", pos.Line, pos.Column, msg))
}

const (
	lowest = iota + 1
	assignPrecedence
	orPrecedence
	andPrecedence
	equalPrecedence
	lessGreaterPrecedence
	sumPrecedence
	productPrecedence
	prefixPrecedence
	callPrecedence
)

var precedences = map[token.Type]int{
	token.Assign:       assignPrecedence,
	token.Define:       assignPrecedence,
	token.OrOr:         orPrecedence,
	token.AndAnd:       andPrecedence,
	token.Equal:        equalPrecedence,
	token.NotEqual:     equalPrecedence,
	token.Less:         lessGreaterPrecedence,
	token.LessEqual:    lessGreaterPrecedence,
	token.Greater:      lessGreaterPrecedence,
	token.GreaterEqual: lessGreaterPrecedence,
	token.Plus:         sumPrecedence,
	token.Minus:        sumPrecedence,
	token.Star:         productPrecedence,
	token.Slash:        productPrecedence,
	token.LParen:       callPrecedence,
	token.LBracket:     callPrecedence,
	token.Dot:          callPrecedence,
}
