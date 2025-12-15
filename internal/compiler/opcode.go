package compiler

import "github.com/xirelogy/go-flux/internal/bytecode"

const (
	OP_CONST         = bytecode.OP_CONST
	OP_NULL          = bytecode.OP_NULL
	OP_TRUE          = bytecode.OP_TRUE
	OP_FALSE         = bytecode.OP_FALSE
	OP_POP           = bytecode.OP_POP
	OP_ADD           = bytecode.OP_ADD
	OP_SUB           = bytecode.OP_SUB
	OP_MUL           = bytecode.OP_MUL
	OP_DIV           = bytecode.OP_DIV
	OP_NEG           = bytecode.OP_NEG
	OP_NOT           = bytecode.OP_NOT
	OP_EQ            = bytecode.OP_EQ
	OP_NEQ           = bytecode.OP_NEQ
	OP_LT            = bytecode.OP_LT
	OP_LTE           = bytecode.OP_LTE
	OP_GT            = bytecode.OP_GT
	OP_GTE           = bytecode.OP_GTE
	OP_AND           = bytecode.OP_AND
	OP_OR            = bytecode.OP_OR
	OP_GET_GLOBAL    = bytecode.OP_GET_GLOBAL
	OP_SET_GLOBAL    = bytecode.OP_SET_GLOBAL
	OP_DEFINE_GLOBAL = bytecode.OP_DEFINE_GLOBAL
	OP_GET_LOCAL     = bytecode.OP_GET_LOCAL
	OP_SET_LOCAL     = bytecode.OP_SET_LOCAL
	OP_GET_UPVALUE   = bytecode.OP_GET_UPVALUE
	OP_SET_UPVALUE   = bytecode.OP_SET_UPVALUE
	OP_ARRAY         = bytecode.OP_ARRAY
	OP_OBJECT        = bytecode.OP_OBJECT
	OP_RANGE         = bytecode.OP_RANGE
	OP_INDEX_GET     = bytecode.OP_INDEX_GET
	OP_INDEX_SET     = bytecode.OP_INDEX_SET
	OP_GET_PROP      = bytecode.OP_GET_PROP
	OP_SET_PROP      = bytecode.OP_SET_PROP
	OP_JUMP          = bytecode.OP_JUMP
	OP_JUMP_IF_FALSE = bytecode.OP_JUMP_IF_FALSE
	OP_JUMP_IF_TRUE  = bytecode.OP_JUMP_IF_TRUE
	OP_CALL          = bytecode.OP_CALL
	OP_RETURN        = bytecode.OP_RETURN
	OP_CLOSURE       = bytecode.OP_CLOSURE
	OP_ITER_PREP     = bytecode.OP_ITER_PREP
	OP_ITER_NEXT     = bytecode.OP_ITER_NEXT
	OP_NOP           = bytecode.OP_NOP
	OP_DEBUG         = bytecode.OP_DEBUG
	// 0x80-0x9F reserved for built-ins. See internal/builtins for assignments.
)
