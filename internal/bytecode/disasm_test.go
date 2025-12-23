package bytecode

import (
	"bytes"
	"strings"
	"testing"
)

func TestDisassembleBuiltinName(t *testing.T) {
	const opcode byte = 0x83
	if _, ok := LookupBuiltinInfo(opcode); !ok {
		RegisterBuiltinInfo("indexRead", opcode, 3)
	}
	proto := &Prototype{
		Name: "test",
		Chunk: &Chunk{
			Code:  []byte{opcode},
			Lines: []LineInfo{{Offset: 0, Line: 1}},
		},
	}
	var buf bytes.Buffer
	dis := NewDisassembler(&buf)
	if err := dis.DisassemblePrototype("test", proto); err != nil {
		t.Fatalf("disassemble: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "OP_BUILTIN_indexRead") {
		t.Fatalf("expected builtin name, got:\n%s", out)
	}
	if !strings.Contains(out, "arity=3") {
		t.Fatalf("expected arity, got:\n%s", out)
	}
}
