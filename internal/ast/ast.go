package ast

import "github.com/xirelogy/go-flux/internal/token"

// Node represents any AST node.
type Node interface {
	Pos() token.Position
	Span() token.Span
}

// Statement is an executable node.
type Statement interface {
	Node
	stmtNode()
}

// Expression produces a value.
type Expression interface {
	Node
	exprNode()
}

// Program is the root node.
type Program struct {
	Statements []Statement
	NodeSpan   token.Span
}

func (p *Program) Pos() token.Position {
	if len(p.Statements) == 0 {
		return token.Position{}
	}
	return p.Statements[0].Pos()
}
func (p *Program) Span() token.Span { return p.NodeSpan }

// Statements

type BlockStmt struct {
	LBrace     token.Position
	Statements []Statement
	BlockSpan  token.Span
}

func (b *BlockStmt) Pos() token.Position { return b.LBrace }
func (b *BlockStmt) Span() token.Span    { return b.BlockSpan }
func (b *BlockStmt) stmtNode()           {}

type ExprStmt struct {
	Expression Expression
	Start      token.Position
	StmtSpan   token.Span
}

func (e *ExprStmt) Pos() token.Position { return e.Start }
func (e *ExprStmt) Span() token.Span    { return e.StmtSpan }
func (e *ExprStmt) stmtNode()           {}

type ReturnStmt struct {
	Return   token.Position
	Value    Expression
	StmtSpan token.Span
}

func (r *ReturnStmt) Pos() token.Position { return r.Return }
func (r *ReturnStmt) Span() token.Span    { return r.StmtSpan }
func (r *ReturnStmt) stmtNode()           {}

type IfStmt struct {
	IfPos     token.Position
	Condition Expression
	Conseq    *BlockStmt
	ElseIfs   []ElseIfClause
	Alt       *BlockStmt
	IfSpan    token.Span
}

func (i *IfStmt) Pos() token.Position { return i.IfPos }
func (i *IfStmt) Span() token.Span    { return i.IfSpan }
func (i *IfStmt) stmtNode()           {}

type ElseIfClause struct {
	Condition Expression
	Conseq    *BlockStmt
	Pos       token.Position
	Span      token.Span
}

type WhileStmt struct {
	WhilePos  token.Position
	Condition Expression
	Body      *BlockStmt
	NodeSpan  token.Span
}

func (w *WhileStmt) Pos() token.Position { return w.WhilePos }
func (w *WhileStmt) Span() token.Span    { return w.NodeSpan }
func (w *WhileStmt) stmtNode()           {}

type ForStmt struct {
	ForPos   token.Position
	Binding  ForBinding
	Iterable Expression
	Body     *BlockStmt
	NodeSpan token.Span
}

func (f *ForStmt) Pos() token.Position { return f.ForPos }
func (f *ForStmt) Span() token.Span    { return f.NodeSpan }
func (f *ForStmt) stmtNode()           {}

type ForBinding struct {
	Pos       token.Position
	Key       string // empty if only value
	ValueName string
}

type FuncDecl struct {
	FuncPos  token.Position
	Name     string
	NamePos  token.Position
	Params   []Param
	Body     *BlockStmt
	NodeSpan token.Span
}

func (f *FuncDecl) Pos() token.Position { return f.FuncPos }
func (f *FuncDecl) Span() token.Span    { return f.NodeSpan }
func (f *FuncDecl) stmtNode()           {}

// Expressions

type Identifier struct {
	Name string
	PosT token.Position
	Sp   token.Span
}

func (i *Identifier) Pos() token.Position { return i.PosT }
func (i *Identifier) Span() token.Span    { return i.Sp }
func (i *Identifier) exprNode()           {}

type Variable struct {
	Name string
	PosT token.Position
	Sp   token.Span
}

func (v *Variable) Pos() token.Position { return v.PosT }
func (v *Variable) Span() token.Span    { return v.Sp }
func (v *Variable) exprNode()           {}

type NumberLiteral struct {
	Value string
	PosT  token.Position
	Sp    token.Span
}

func (n *NumberLiteral) Pos() token.Position { return n.PosT }
func (n *NumberLiteral) Span() token.Span    { return n.Sp }
func (n *NumberLiteral) exprNode()           {}

type StringLiteral struct {
	Value string
	PosT  token.Position
	Sp    token.Span
}

func (s *StringLiteral) Pos() token.Position { return s.PosT }
func (s *StringLiteral) Span() token.Span    { return s.Sp }
func (s *StringLiteral) exprNode()           {}

type BoolLiteral struct {
	Value bool
	PosT  token.Position
	Sp    token.Span
}

func (b *BoolLiteral) Pos() token.Position { return b.PosT }
func (b *BoolLiteral) Span() token.Span    { return b.Sp }
func (b *BoolLiteral) exprNode()           {}

type NullLiteral struct {
	PosT token.Position
	Sp   token.Span
}

func (n *NullLiteral) Pos() token.Position { return n.PosT }
func (n *NullLiteral) Span() token.Span    { return n.Sp }
func (n *NullLiteral) exprNode()           {}

type ArrayLiteral struct {
	Elements []Expression
	PosT     token.Position
	Sp       token.Span
}

func (a *ArrayLiteral) Pos() token.Position { return a.PosT }
func (a *ArrayLiteral) Span() token.Span    { return a.Sp }
func (a *ArrayLiteral) exprNode()           {}

type RangeLiteral struct {
	Start Expression
	End   Expression
	PosT  token.Position
	Sp    token.Span
}

func (r *RangeLiteral) Pos() token.Position { return r.PosT }
func (r *RangeLiteral) Span() token.Span    { return r.Sp }
func (r *RangeLiteral) exprNode()           {}

type ObjectLiteral struct {
	Fields []ObjectField
	PosT   token.Position
	Sp     token.Span
}

func (o *ObjectLiteral) Pos() token.Position { return o.PosT }
func (o *ObjectLiteral) Span() token.Span    { return o.Sp }
func (o *ObjectLiteral) exprNode()           {}

type ObjectField struct {
	Key   ObjectKey
	Value Expression
}

type ObjectKey struct {
	Ident string
	Str   *string
	Num   *string
	PosT  token.Position
	Sp    token.Span
}

type IndexExpr struct {
	Left  Expression
	Index Expression
	PosT  token.Position
	Sp    token.Span
}

func (i *IndexExpr) Pos() token.Position { return i.PosT }
func (i *IndexExpr) Span() token.Span    { return i.Sp }
func (i *IndexExpr) exprNode()           {}

type MemberExpr struct {
	Left     Expression
	Property string
	PosT     token.Position
	Sp       token.Span
}

func (m *MemberExpr) Pos() token.Position { return m.PosT }
func (m *MemberExpr) Span() token.Span    { return m.Sp }
func (m *MemberExpr) exprNode()           {}

type CallExpr struct {
	Callee    Expression
	Arguments []Expression
	PosT      token.Position
	Sp        token.Span
}

func (c *CallExpr) Pos() token.Position { return c.PosT }
func (c *CallExpr) Span() token.Span    { return c.Sp }
func (c *CallExpr) exprNode()           {}

type AssignExpr struct {
	Left     Expression
	Value    Expression
	Operator token.Type
	PosT     token.Position
	Sp       token.Span
}

func (a *AssignExpr) Pos() token.Position { return a.PosT }
func (a *AssignExpr) Span() token.Span    { return a.Sp }
func (a *AssignExpr) exprNode()           {}

type BinaryExpr struct {
	Left     Expression
	Operator token.Type
	Right    Expression
	PosT     token.Position
	Sp       token.Span
}

func (b *BinaryExpr) Pos() token.Position { return b.PosT }
func (b *BinaryExpr) Span() token.Span    { return b.Sp }
func (b *BinaryExpr) exprNode()           {}

type UnaryExpr struct {
	Operator token.Type
	Right    Expression
	PosT     token.Position
	Sp       token.Span
}

func (u *UnaryExpr) Pos() token.Position { return u.PosT }
func (u *UnaryExpr) Span() token.Span    { return u.Sp }
func (u *UnaryExpr) exprNode()           {}

type FuncExpr struct {
	FuncPos token.Position
	Params  []Param
	Body    *BlockStmt
	Sp      token.Span
}

func (f *FuncExpr) Pos() token.Position { return f.FuncPos }
func (f *FuncExpr) Span() token.Span    { return f.Sp }
func (f *FuncExpr) exprNode()           {}

type Param struct {
	Name string
	Pos  token.Position
	Sp   token.Span
}
