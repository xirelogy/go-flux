package compiler

import (
	"fmt"
	"strconv"

	"github.com/xirelogy/go-flux/internal/ast"
	"github.com/xirelogy/go-flux/internal/token"
)

// Compile parses a program AST into a Module of function prototypes.
func Compile(prog *ast.Program, source string) (*Module, error) {
	c := &compiler{
		module: &Module{Functions: make(map[string]*Prototype)},
		source: source,
	}

	for _, stmt := range prog.Statements {
		switch fn := stmt.(type) {
		case *ast.FuncDecl:
			proto, err := c.compileFunction(fn)
			if err != nil {
				return nil, err
			}
			c.module.Functions[fn.Name] = proto
		default:
			return nil, fmt.Errorf("top-level statements other than func are not supported")
		}
	}

	return c.module, nil
}

type compiler struct {
	module *Module
	source string
	errors []error
}

type funcCompiler struct {
	chunk  *Chunk
	scope  *scope
	line   int
	temp   int
	source string
}

func (c *compiler) compileFunction(fn *ast.FuncDecl) (*Prototype, error) {
	fc := newFuncCompiler(c.source)

	// parameters as locals
	for i, p := range fn.Params {
		if i >= 255 {
			return nil, fmt.Errorf("too many parameters")
		}
		fc.scope.addLocal(p.Name)
	}

	if err := fc.compileBlock(fn.Body); err != nil {
		return nil, err
	}

	// ensure function returns null if no explicit return
	if len(fn.Body.Statements) == 0 || fc.lastOp() != OP_RETURN {
		fc.emitByte(OP_NULL)
		fc.emitByte(OP_RETURN)
	}

	return &Prototype{
		Name:      fn.Name,
		Source:    c.source,
		NumParams: len(fn.Params),
		Chunk:     fc.chunk,
		Upvalues:  fc.scope.upvalues,
		MaxLocals: int(fc.scope.nextLoc),
	}, nil
}

func newFuncCompiler(source string) *funcCompiler {
	return &funcCompiler{
		chunk:  &Chunk{},
		scope:  newScope(nil),
		source: source,
	}
}

func newFuncCompilerWithScope(parent *scope, source string) *funcCompiler {
	return &funcCompiler{
		chunk:  &Chunk{},
		scope:  newScope(parent),
		source: source,
	}
}

func (fc *funcCompiler) ensureLocal(name string) uint8 {
	if slot, ok := fc.scope.resolveLocal(name); ok {
		return slot
	}
	return fc.scope.addLocal(name)
}

func (fc *funcCompiler) newTemp() uint8 {
	name := fmt.Sprintf("!t%d", fc.temp)
	fc.temp++
	return fc.scope.addLocal(name)
}

func (fc *funcCompiler) lastOp() byte {
	if len(fc.chunk.Code) == 0 {
		return 0
	}
	return fc.chunk.Code[len(fc.chunk.Code)-1]
}

func (fc *funcCompiler) compileBlock(block *ast.BlockStmt) error {
	for _, stmt := range block.Statements {
		fc.setLine(stmt.Pos().Line)
		switch s := stmt.(type) {
		case *ast.ExprStmt:
			if err := fc.compileExpr(s.Expression); err != nil {
				return err
			}
			if _, ok := s.Expression.(*ast.AssignExpr); !ok {
				fc.emitByte(OP_POP)
			}
		case *ast.ReturnStmt:
			if s.Value != nil {
				if err := fc.compileExpr(s.Value); err != nil {
					return err
				}
			} else {
				fc.emitByte(OP_NULL)
			}
			fc.emitByte(OP_RETURN)
		case *ast.IfStmt:
			if err := fc.compileIf(s); err != nil {
				return err
			}
		case *ast.WhileStmt:
			if err := fc.compileWhile(s); err != nil {
				return err
			}
		case *ast.ForStmt:
			if err := fc.compileForIn(s); err != nil {
				return err
			}
		case *ast.FuncDecl:
			if err := fc.compileNestedFuncDecl(s); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported statement type %T", stmt)
		}
	}
	return nil
}

func (fc *funcCompiler) compileIf(stmt *ast.IfStmt) error {
	if err := fc.compileExpr(stmt.Condition); err != nil {
		return err
	}
	// Jump if false to else/next
	jumpIfFalsePos := fc.emitJump(OP_JUMP_IF_FALSE)
	fc.emitByte(OP_POP) // pop condition before executing conseq

	if err := fc.compileBlock(stmt.Conseq); err != nil {
		return err
	}
	jumpOverElse := fc.emitJump(OP_JUMP)
	fc.patchJump(jumpIfFalsePos)
	fc.emitByte(OP_POP) // pop condition when skipping conseq

	// elseifs / else
	for _, clause := range stmt.ElseIfs {
		if err := fc.compileExpr(clause.Condition); err != nil {
			return err
		}
		jFalse := fc.emitJump(OP_JUMP_IF_FALSE)
		fc.emitByte(OP_POP)
		if err := fc.compileBlock(clause.Conseq); err != nil {
			return err
		}
		jOver := fc.emitJump(OP_JUMP)
		fc.patchJump(jFalse)
		fc.emitByte(OP_POP)
		fc.patchJump(jOver)
	}

	if stmt.Alt != nil {
		if err := fc.compileBlock(stmt.Alt); err != nil {
			return err
		}
	}
	fc.patchJump(jumpOverElse)
	return nil
}

func (fc *funcCompiler) compileWhile(stmt *ast.WhileStmt) error {
	loopStart := len(fc.chunk.Code)
	if err := fc.compileExpr(stmt.Condition); err != nil {
		return err
	}
	// jump out if false
	exitJump := fc.emitJump(OP_JUMP_IF_FALSE)
	fc.emitByte(OP_POP)
	if err := fc.compileBlock(stmt.Body); err != nil {
		return err
	}
	fc.emitLoop(loopStart)
	fc.patchJump(exitJump)
	fc.emitByte(OP_POP)
	return nil
}

func (fc *funcCompiler) compileForIn(stmt *ast.ForStmt) error {
	// iterator preparation
	if err := fc.compileExpr(stmt.Iterable); err != nil {
		return err
	}
	fc.emitByte(OP_ITER_PREP) // leaves iterator on stack

	loopStart := len(fc.chunk.Code)
	iterNextPos := fc.emitJump(OP_ITER_NEXT) // jump target patched to exit; opcode consumes iterator?

	// When OP_ITER_NEXT succeeds, it should push key/value or value. We assign to bindings.
	if stmt.Binding.Key != "" {
		keySlot := fc.ensureLocal(stmt.Binding.Key)
		valSlot := fc.ensureLocal(stmt.Binding.ValueName)
		// stack: ... key value
		fc.emitBytes(OP_SET_LOCAL, valSlot)
		fc.emitBytes(OP_SET_LOCAL, keySlot)
	} else {
		valSlot := fc.ensureLocal(stmt.Binding.ValueName)
		fc.emitBytes(OP_SET_LOCAL, valSlot)
		fc.emitByte(OP_POP) // discard key
	}

	if err := fc.compileBlock(stmt.Body); err != nil {
		return err
	}
	fc.emitLoop(loopStart)
	fc.patchJump(iterNextPos)
	fc.emitByte(OP_POP) // pop iterator
	return nil
}

func (fc *funcCompiler) compileExpr(expr ast.Expression) error {
	fc.setLine(expr.Pos().Line)
	switch e := expr.(type) {
	case *ast.NumberLiteral:
		num, err := strconv.ParseFloat(e.Value, 64)
		if err != nil {
			return fmt.Errorf("invalid number %q", e.Value)
		}
		fc.emitConst(num)
	case *ast.StringLiteral:
		fc.emitConst(e.Value)
	case *ast.BoolLiteral:
		if e.Value {
			fc.emitByte(OP_TRUE)
		} else {
			fc.emitByte(OP_FALSE)
		}
	case *ast.NullLiteral:
		fc.emitByte(OP_NULL)
	case *ast.ArrayLiteral:
		for _, el := range e.Elements {
			if err := fc.compileExpr(el); err != nil {
				return err
			}
		}
		fc.emitBytes(OP_ARRAY, byte(len(e.Elements)>>8), byte(len(e.Elements)))
	case *ast.RangeLiteral:
		if err := fc.compileExpr(e.Start); err != nil {
			return err
		}
		if err := fc.compileExpr(e.End); err != nil {
			return err
		}
		fc.emitByte(OP_RANGE)
	case *ast.ObjectLiteral:
		for _, f := range e.Fields {
			key := objectKeyToString(f.Key)
			fc.emitConst(key)
			if err := fc.compileExpr(f.Value); err != nil {
				return err
			}
		}
		count := len(e.Fields)
		fc.emitBytes(OP_OBJECT, byte(count>>8), byte(count))
	case *ast.Identifier:
		fc.emitGlobalGet(e.Name)
	case *ast.Variable:
		if slot, ok := fc.scope.resolveLocal(e.Name); ok {
			fc.emitBytes(OP_GET_LOCAL, slot)
		} else if up, ok := fc.scope.resolveUpvalue(e.Name); ok {
			fc.emitBytes(OP_GET_UPVALUE, up.Index)
		} else {
			fc.emitGlobalGet(e.Name)
		}
	case *ast.UnaryExpr:
		if err := fc.compileExpr(e.Right); err != nil {
			return err
		}
		switch e.Operator {
		case token.Minus:
			fc.emitByte(OP_NEG)
		case token.Bang:
			fc.emitByte(OP_NOT)
		case token.Plus:
			// unary plus is a no-op
		default:
			return fmt.Errorf("unsupported unary op %s", e.Operator)
		}
	case *ast.BinaryExpr:
		if e.Operator == token.AndAnd || e.Operator == token.OrOr {
			return fc.compileLogical(e)
		}
		if err := fc.compileExpr(e.Left); err != nil {
			return err
		}
		if err := fc.compileExpr(e.Right); err != nil {
			return err
		}
		switch e.Operator {
		case token.Plus:
			fc.emitByte(OP_ADD)
		case token.Minus:
			fc.emitByte(OP_SUB)
		case token.Star:
			fc.emitByte(OP_MUL)
		case token.Slash:
			fc.emitByte(OP_DIV)
		case token.Equal:
			fc.emitByte(OP_EQ)
		case token.NotEqual:
			fc.emitByte(OP_NEQ)
		case token.Less:
			fc.emitByte(OP_LT)
		case token.LessEqual:
			fc.emitByte(OP_LTE)
		case token.Greater:
			fc.emitByte(OP_GT)
		case token.GreaterEqual:
			fc.emitByte(OP_GTE)
		default:
			return fmt.Errorf("unsupported binary op %s", e.Operator)
		}
	case *ast.AssignExpr:
		return fc.compileAssign(e)
	case *ast.CallExpr:
		if name, ok := builtinName(e.Callee); ok {
			for _, arg := range e.Arguments {
				if err := fc.compileExpr(arg); err != nil {
					return err
				}
			}
			if err := fc.emitBuiltin(name, len(e.Arguments)); err != nil {
				return err
			}
		} else {
			if err := fc.compileExpr(e.Callee); err != nil {
				return err
			}
			for _, arg := range e.Arguments {
				if err := fc.compileExpr(arg); err != nil {
					return err
				}
			}
			fc.emitBytes(OP_CALL, byte(len(e.Arguments)))
		}
	case *ast.MemberExpr:
		if err := fc.compileExpr(e.Left); err != nil {
			return err
		}
		idx := fc.addConst(e.Property)
		fc.emitBytes(OP_GET_PROP, byte(idx>>8), byte(idx))
	case *ast.IndexExpr:
		if err := fc.compileExpr(e.Left); err != nil {
			return err
		}
		if err := fc.compileExpr(e.Index); err != nil {
			return err
		}
		fc.emitByte(OP_INDEX_GET)
	case *ast.FuncExpr:
		return fc.compileFuncExpr(e)
	default:
		return fmt.Errorf("unsupported expression type %T", expr)
	}
	return nil
}

func (fc *funcCompiler) compileLogical(e *ast.BinaryExpr) error {
	switch e.Operator {
	case token.AndAnd:
		if err := fc.compileExpr(e.Left); err != nil {
			return err
		}
		endJump := fc.emitJump(OP_JUMP_IF_FALSE)
		fc.emitByte(OP_POP)
		if err := fc.compileExpr(e.Right); err != nil {
			return err
		}
		fc.patchJump(endJump)
		return nil
	case token.OrOr:
		if err := fc.compileExpr(e.Left); err != nil {
			return err
		}
		endJump := fc.emitJump(OP_JUMP_IF_TRUE)
		fc.emitByte(OP_POP)
		if err := fc.compileExpr(e.Right); err != nil {
			return err
		}
		fc.patchJump(endJump)
		return nil
	default:
		return fmt.Errorf("unsupported logical op %s", e.Operator)
	}
}

func (fc *funcCompiler) compileAssign(e *ast.AssignExpr) error {
	switch lhs := e.Left.(type) {
	case *ast.Variable:
		if e.Operator == token.Define {
			if _, exists := fc.scope.locals[lhs.Name]; !exists {
				fc.scope.addLocal(lhs.Name)
			}
		}
		if err := fc.compileExpr(e.Value); err != nil {
			return err
		}
		if slot, ok := fc.scope.resolveLocal(lhs.Name); ok {
			fc.emitBytes(OP_SET_LOCAL, slot)
		} else if up, ok := fc.scope.resolveUpvalue(lhs.Name); ok {
			fc.emitBytes(OP_SET_UPVALUE, up.Index)
		} else {
			fc.emitGlobalSet(lhs.Name, e.Operator == token.Define)
		}
	case *ast.MemberExpr:
		if err := fc.compileExpr(lhs.Left); err != nil {
			return err
		}
		idx := fc.addConst(lhs.Property)
		if err := fc.compileExpr(e.Value); err != nil {
			return err
		}
		fc.emitBytes(OP_SET_PROP, byte(idx>>8), byte(idx))
	case *ast.IndexExpr:
		if err := fc.compileExpr(lhs.Left); err != nil {
			return err
		}
		if err := fc.compileExpr(lhs.Index); err != nil {
			return err
		}
		if err := fc.compileExpr(e.Value); err != nil {
			return err
		}
		fc.emitByte(OP_INDEX_SET)
	default:
		return fmt.Errorf("invalid assignment target %T", e.Left)
	}
	return nil
}

func (fc *funcCompiler) compileFuncExpr(fn *ast.FuncExpr) error {
	idx, upvalues, err := fc.compilePrototype("", fn.Params, fn.Body)
	if err != nil {
		return err
	}
	fc.emitBytes(OP_CLOSURE, byte(idx>>8), byte(idx), byte(len(upvalues)))
	for _, uv := range upvalues {
		isLocal := byte(0)
		if uv.IsLocal {
			isLocal = 1
		}
		fc.emitBytes(isLocal, uv.Index)
	}
	return nil
}

func (fc *funcCompiler) compileNestedFuncDecl(fn *ast.FuncDecl) error {
	idx, upvalues, err := fc.compilePrototype(fn.Name, fn.Params, fn.Body)
	if err != nil {
		return err
	}
	fc.emitBytes(OP_CLOSURE, byte(idx>>8), byte(idx), byte(len(upvalues)))
	for _, uv := range upvalues {
		isLocal := byte(0)
		if uv.IsLocal {
			isLocal = 1
		}
		fc.emitBytes(isLocal, uv.Index)
	}
	slot := fc.ensureLocal(fn.Name)
	fc.emitBytes(OP_SET_LOCAL, slot)
	return nil
}

func (fc *funcCompiler) compilePrototype(name string, params []ast.Param, body *ast.BlockStmt) (uint16, []Upvalue, error) {
	child := newFuncCompilerWithScope(fc.scope, fc.source)
	for i, p := range params {
		if i >= 255 {
			return 0, nil, fmt.Errorf("too many parameters")
		}
		child.scope.addLocal(p.Name)
	}
	if err := child.compileBlock(body); err != nil {
		return 0, nil, err
	}
	if len(body.Statements) == 0 || child.lastOp() != OP_RETURN {
		child.emitByte(OP_NULL)
		child.emitByte(OP_RETURN)
	}
	proto := &Prototype{
		Name:      name,
		Source:    fc.source,
		NumParams: len(params),
		Chunk:     child.chunk,
		Upvalues:  child.scope.upvalues,
		MaxLocals: int(child.scope.nextLoc),
	}
	idx := fc.addConst(proto)
	return idx, proto.Upvalues, nil
}

func (fc *funcCompiler) emitConst(v interface{}) {
	idx := fc.addConst(v)
	fc.emitBytes(OP_CONST, byte(idx>>8), byte(idx))
}

func (fc *funcCompiler) addConst(v interface{}) uint16 {
	fc.chunk.Consts = append(fc.chunk.Consts, v)
	return uint16(len(fc.chunk.Consts) - 1)
}

func (fc *funcCompiler) lastConstIndexBytes() []byte {
	idx := uint16(len(fc.chunk.Consts) - 1)
	return []byte{byte(idx >> 8), byte(idx)}
}

func (fc *funcCompiler) emitGlobalGet(name string) {
	idx := fc.addConst(name)
	fc.emitBytes(OP_GET_GLOBAL, byte(idx>>8), byte(idx))
}

func (fc *funcCompiler) emitGlobalSet(name string, define bool) {
	idx := fc.addConst(name)
	if define {
		fc.emitBytes(OP_DEFINE_GLOBAL, byte(idx>>8), byte(idx))
	} else {
		fc.emitBytes(OP_SET_GLOBAL, byte(idx>>8), byte(idx))
	}
}

func (fc *funcCompiler) emitByte(b byte) {
	fc.recordLine()
	fc.chunk.Code = append(fc.chunk.Code, b)
}

func (fc *funcCompiler) emitBytes(b ...byte) {
	fc.recordLine()
	fc.chunk.Code = append(fc.chunk.Code, b...)
}

func (fc *funcCompiler) emitJump(op byte) int {
	fc.emitByte(op)
	// placeholder for u16
	fc.emitByte(0xff)
	fc.emitByte(0xff)
	return len(fc.chunk.Code) - 2
}

func (fc *funcCompiler) patchJump(pos int) {
	offset := len(fc.chunk.Code)
	fc.chunk.Code[pos] = byte(offset >> 8)
	fc.chunk.Code[pos+1] = byte(offset)
}

func (fc *funcCompiler) emitLoop(start int) {
	fc.emitByte(OP_JUMP)
	offset := start
	fc.emitByte(byte(offset >> 8))
	fc.emitByte(byte(offset))
}

func (fc *funcCompiler) setLine(line int) {
	if line > 0 {
		fc.line = line
	}
}

func (fc *funcCompiler) recordLine() {
	if fc.line == 0 {
		return
	}
	off := len(fc.chunk.Code)
	if len(fc.chunk.Lines) == 0 || fc.chunk.Lines[len(fc.chunk.Lines)-1].Offset != off {
		fc.chunk.Lines = append(fc.chunk.Lines, LineInfo{Offset: off, Line: fc.line})
	}
}

func objectKeyToString(k ast.ObjectKey) string {
	if k.Ident != "" {
		return k.Ident
	}
	if k.Str != nil {
		return *k.Str
	}
	if k.Num != nil {
		return *k.Num
	}
	return ""
}
