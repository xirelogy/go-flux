package bytecode

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Disassembler formats bytecode as a readable assembly-style dump.
type Disassembler struct {
	w       io.Writer
	visited map[*Prototype]bool
	printed bool
}

// NewDisassembler constructs a disassembler that writes to w.
func NewDisassembler(w io.Writer) *Disassembler {
	return &Disassembler{
		w:       w,
		visited: make(map[*Prototype]bool),
	}
}

// DisassemblePrototype emits a readable dump for a prototype and any nested prototypes.
func (d *Disassembler) DisassemblePrototype(label string, proto *Prototype) error {
	if proto == nil || proto.Chunk == nil {
		return fmt.Errorf("nil prototype")
	}
	if d.visited[proto] {
		return nil
	}
	d.visited[proto] = true
	d.startSection()
	name := label
	if name == "" {
		name = proto.Name
	}
	if name == "" {
		name = "<anon>"
	}
	source := proto.Source
	if source == "" {
		source = "<unknown>"
	}
	fmt.Fprintf(d.w, "func %s (params=%d, locals=%d, upvalues=%d) source=%s\n",
		name, proto.NumParams, proto.MaxLocals, len(proto.Upvalues), source)
	if err := d.disassembleChunk(proto.Chunk); err != nil {
		return err
	}
	for idx, c := range proto.Chunk.Consts {
		child, ok := c.(*Prototype)
		if !ok {
			continue
		}
		childName := child.Name
		if childName == "" {
			childName = fmt.Sprintf("<closure@const:%d>", idx)
		}
		if err := d.DisassemblePrototype(childName, child); err != nil {
			return err
		}
	}
	return nil
}

// PrintNative emits a header for a native (host) function.
func (d *Disassembler) PrintNative(name string) {
	d.startSection()
	if name == "" {
		name = "<native>"
	}
	fmt.Fprintf(d.w, "func %s [native]\n", name)
}

// PrintMissing emits a header when a function has no prototype.
func (d *Disassembler) PrintMissing(name string) {
	d.startSection()
	if name == "" {
		name = "<unknown>"
	}
	fmt.Fprintf(d.w, "func %s [missing prototype]\n", name)
}

func (d *Disassembler) startSection() {
	if d.printed {
		fmt.Fprintln(d.w)
	}
	d.printed = true
}

func (d *Disassembler) disassembleChunk(chunk *Chunk) error {
	if chunk == nil {
		return fmt.Errorf("nil chunk")
	}
	code := chunk.Code
	for ip := 0; ip < len(code); {
		offset := ip
		op := code[ip]
		ip++
		line := lineForOffset(chunk.Lines, offset)
		lineStr := "-"
		if line > 0 {
			lineStr = strconv.Itoa(line)
		}
		opName, comment := opName(op)
		operands, err := d.decodeOperands(op, chunk, &ip)
		if err != nil {
			return err
		}
		detail := strings.TrimSpace(operands)
		if comment != "" {
			if detail != "" {
				detail += " "
			}
			detail += "; " + comment
		}
		fmt.Fprintf(d.w, "%04d %4s %-16s", offset, lineStr, opName)
		if detail != "" {
			fmt.Fprintf(d.w, " %s", detail)
		}
		fmt.Fprintln(d.w)
	}
	return nil
}

func (d *Disassembler) decodeOperands(op byte, chunk *Chunk, ip *int) (string, error) {
	code := chunk.Code
	switch op {
	case OP_CONST:
		idx, err := readU16(code, ip)
		if err != nil {
			return "", err
		}
		if int(idx) >= len(chunk.Consts) {
			return "", fmt.Errorf("const index out of range: %d", idx)
		}
		return fmt.Sprintf("%d ; const[%d]=%s", idx, idx, formatConst(chunk.Consts[idx])), nil
	case OP_GET_GLOBAL, OP_SET_GLOBAL, OP_DEFINE_GLOBAL:
		idx, err := readU16(code, ip)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d ; name=%s", idx, formatConstRef(chunk, idx)), nil
	case OP_GET_PROP, OP_SET_PROP:
		idx, err := readU16(code, ip)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d ; prop=%s", idx, formatConstRef(chunk, idx)), nil
	case OP_GET_LOCAL, OP_SET_LOCAL, OP_GET_UPVALUE, OP_SET_UPVALUE, OP_CALL:
		slot, err := readU8(code, ip)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d", slot), nil
	case OP_ARRAY, OP_OBJECT:
		count, err := readU16(code, ip)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d", count), nil
	case OP_JUMP, OP_JUMP_IF_FALSE, OP_JUMP_IF_TRUE, OP_ITER_NEXT:
		off, err := readU16(code, ip)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d", off), nil
	case OP_CLOSURE:
		idx, err := readU16(code, ip)
		if err != nil {
			return "", err
		}
		upcount, err := readU8(code, ip)
		if err != nil {
			return "", err
		}
		upvals := make([]string, 0, upcount)
		for i := 0; i < int(upcount); i++ {
			isLocal, err := readU8(code, ip)
			if err != nil {
				return "", err
			}
			slot, err := readU8(code, ip)
			if err != nil {
				return "", err
			}
			if isLocal == 1 {
				upvals = append(upvals, fmt.Sprintf("local %d", slot))
			} else {
				upvals = append(upvals, fmt.Sprintf("upvalue %d", slot))
			}
		}
		operand := fmt.Sprintf("%d %d", idx, upcount)
		if len(upvals) > 0 {
			operand = operand + " [" + strings.Join(upvals, ", ") + "]"
		}
		return operand, nil
	default:
		return "", nil
	}
}

func opName(op byte) (string, string) {
	if info, ok := LookupBuiltinInfo(op); ok {
		return "OP_BUILTIN_" + info.Name, fmt.Sprintf("arity=%d", info.Arity)
	}
	if op >= 0x80 {
		return fmt.Sprintf("OP_BUILTIN_0x%02X", op), ""
	}
	switch op {
	case OP_CONST:
		return "OP_CONST", ""
	case OP_NULL:
		return "OP_NULL", ""
	case OP_TRUE:
		return "OP_TRUE", ""
	case OP_FALSE:
		return "OP_FALSE", ""
	case OP_POP:
		return "OP_POP", ""
	case OP_ADD:
		return "OP_ADD", ""
	case OP_SUB:
		return "OP_SUB", ""
	case OP_MUL:
		return "OP_MUL", ""
	case OP_DIV:
		return "OP_DIV", ""
	case OP_NEG:
		return "OP_NEG", ""
	case OP_NOT:
		return "OP_NOT", ""
	case OP_EQ:
		return "OP_EQ", ""
	case OP_NEQ:
		return "OP_NEQ", ""
	case OP_LT:
		return "OP_LT", ""
	case OP_LTE:
		return "OP_LTE", ""
	case OP_GT:
		return "OP_GT", ""
	case OP_GTE:
		return "OP_GTE", ""
	case OP_AND:
		return "OP_AND", ""
	case OP_OR:
		return "OP_OR", ""
	case OP_GET_GLOBAL:
		return "OP_GET_GLOBAL", ""
	case OP_SET_GLOBAL:
		return "OP_SET_GLOBAL", ""
	case OP_DEFINE_GLOBAL:
		return "OP_DEFINE_GLOBAL", ""
	case OP_GET_LOCAL:
		return "OP_GET_LOCAL", ""
	case OP_SET_LOCAL:
		return "OP_SET_LOCAL", ""
	case OP_GET_UPVALUE:
		return "OP_GET_UPVALUE", ""
	case OP_SET_UPVALUE:
		return "OP_SET_UPVALUE", ""
	case OP_ARRAY:
		return "OP_ARRAY", ""
	case OP_OBJECT:
		return "OP_OBJECT", ""
	case OP_RANGE:
		return "OP_RANGE", ""
	case OP_INDEX_GET:
		return "OP_INDEX_GET", ""
	case OP_INDEX_SET:
		return "OP_INDEX_SET", ""
	case OP_GET_PROP:
		return "OP_GET_PROP", ""
	case OP_SET_PROP:
		return "OP_SET_PROP", ""
	case OP_JUMP:
		return "OP_JUMP", ""
	case OP_JUMP_IF_FALSE:
		return "OP_JUMP_IF_FALSE", ""
	case OP_JUMP_IF_TRUE:
		return "OP_JUMP_IF_TRUE", ""
	case OP_CALL:
		return "OP_CALL", ""
	case OP_RETURN:
		return "OP_RETURN", ""
	case OP_CLOSURE:
		return "OP_CLOSURE", ""
	case OP_NOP:
		return "OP_NOP", ""
	case OP_DEBUG:
		return "OP_DEBUG", ""
	case OP_ITER_PREP:
		return "OP_ITER_PREP", ""
	case OP_ITER_NEXT:
		return "OP_ITER_NEXT", ""
	default:
		return fmt.Sprintf("OP_0x%02X", op), ""
	}
}

func lineForOffset(lines []LineInfo, offset int) int {
	line := 0
	for _, info := range lines {
		if info.Offset > offset {
			break
		}
		line = info.Line
	}
	return line
}

func readU8(code []byte, ip *int) (byte, error) {
	if *ip >= len(code) {
		return 0, fmt.Errorf("unexpected end of bytecode")
	}
	val := code[*ip]
	*ip = *ip + 1
	return val, nil
}

func readU16(code []byte, ip *int) (uint16, error) {
	if *ip+1 >= len(code) {
		return 0, fmt.Errorf("unexpected end of bytecode")
	}
	hi := code[*ip]
	lo := code[*ip+1]
	*ip += 2
	return uint16(hi)<<8 | uint16(lo), nil
}

func formatConstRef(chunk *Chunk, idx uint16) string {
	if chunk == nil || int(idx) >= len(chunk.Consts) {
		return "<invalid>"
	}
	return formatConst(chunk.Consts[idx])
}

func formatConst(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case string:
		return strconv.Quote(val)
	case *Prototype:
		name := val.Name
		if name == "" {
			name = "<anon>"
		}
		return "proto " + name
	default:
		return "<unknown>"
	}
}
