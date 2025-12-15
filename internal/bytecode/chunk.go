package bytecode

// Chunk is a compiled bytecode sequence with its constant pool.
type Chunk struct {
	Code   []byte
	Consts []interface{}
	Lines  []LineInfo
}

// Prototype represents a compiled function.
type Prototype struct {
	Name      string
	Source    string
	NumParams int
	Chunk     *Chunk
	Upvalues  []Upvalue
	MaxLocals int
}

// Module is the compiled form of a program: a set of function prototypes.
type Module struct {
	Functions map[string]*Prototype
}

// Upvalue describes a captured variable.
type Upvalue struct {
	IsLocal bool
	Index   uint8
}

// LineInfo maps bytecode offsets to source lines (start-inclusive).
type LineInfo struct {
	Offset int
	Line   int
}
