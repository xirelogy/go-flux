package bytecode

// OpCode enumerates bytecode operations.
// Keep values in sync with docs/BYTECODE.md.
const (
	OP_CONST byte = iota
	OP_NULL
	OP_TRUE
	OP_FALSE
	OP_POP
	_ // reserved
	_ // reserved
	_ // reserved

	OP_ADD
	OP_SUB
	OP_MUL
	OP_DIV
	OP_NEG
	OP_NOT
	_ // reserved
	_ // reserved

	OP_EQ
	OP_NEQ
	OP_LT
	OP_LTE
	OP_GT
	OP_GTE
	OP_AND
	OP_OR

	OP_GET_GLOBAL
	OP_SET_GLOBAL
	OP_DEFINE_GLOBAL
	_ // reserved
	_ // reserved
	_ // reserved
	_ // reserved
	_ // reserved

	OP_GET_LOCAL
	OP_SET_LOCAL
	OP_GET_UPVALUE
	OP_SET_UPVALUE
	_ // reserved
	_ // reserved
	_ // reserved
	_ // reserved

	OP_ARRAY
	OP_OBJECT
	OP_RANGE
	OP_INDEX_GET
	OP_INDEX_SET
	OP_GET_PROP
	OP_SET_PROP
	_ // reserved

	OP_JUMP
	OP_JUMP_IF_FALSE
	OP_JUMP_IF_TRUE
	_ // reserved
	_ // reserved
	_ // reserved
	_ // reserved
	_ // reserved

	OP_CALL
	OP_RETURN
	OP_CLOSURE
	_ // reserved
	_ // reserved
	_ // reserved
	_ // reserved
	_ // reserved
)

const (
	OP_NOP   byte = 0x40
	OP_DEBUG      = 0x41

	OP_ITER_PREP byte = 0x48
	OP_ITER_NEXT      = 0x49

	// 0x80-0x9F: reserved for built-in operations.
)
