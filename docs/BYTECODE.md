# go-flux Bytecode (draft)

This document outlines the initial bytecode format for the go-flux VM. Goals: compact, deterministic, easy to interpret, and friendly to embedding with clear limits (stack depth, instruction count).

## Representation
- **Chunk**: compiled output of a source unit; contains `code []byte`, `consts []Value`, `lines []LineInfo`, and `upvalues` metadata per function.
- **Instructions**: one-byte opcode followed by zero or more operands (little-endian). Most operands are 1 or 2 bytes for compactness.
- **Constants**: pool of literals and function prototypes. Strings are UTF-8; numbers are 64-bit float; booleans/null use tagged constants.
- **Line info**: sparse mapping from bytecode offset to source line for diagnostics.

## Value model (runtime)
- Dynamic types: null, boolean, number (float64), string, array, object, function/closure, error.
- Arrays/objects are heap-managed; values on the VM stack are tagged references.

## Stack frame layout
- **Data stack**: operand stack.
- **Call frame**: holds return IP, base pointer, function prototype reference, upvalue bindings.
- **Upvalues**: closures capture slots by index; open upvalues track stack slots; closed upvalues heap-allocate copies.

## Opcodes (draft)
```
00 OP_CONST <u16 idx>        ; push consts[idx]
01 OP_NULL                   ; push null
02 OP_TRUE                   ; push true
03 OP_FALSE                  ; push false
04 OP_POP                    ; pop and discard

08 OP_ADD                    ; binary +
09 OP_SUB                    ; binary -
0A OP_MUL                    ; binary *
0B OP_DIV                    ; binary /
0C OP_NEG                    ; unary -
0D OP_NOT                    ; unary !

10 OP_EQ                     ; ==
11 OP_NEQ                    ; !=
12 OP_LT                     ; <
13 OP_LTE                    ; <=
14 OP_GT                     ; >
15 OP_GTE                    ; >=
16 OP_AND                    ; short-circuit &&
17 OP_OR                     ; short-circuit ||

18 OP_GET_GLOBAL <u16 idx>   ; push global by name const idx
19 OP_SET_GLOBAL <u16 idx>   ; assign global (expects value on stack)
1A OP_DEFINE_GLOBAL <u16 idx>; define global (value on stack)

20 OP_GET_LOCAL  <u8 slot>   ; push local
21 OP_SET_LOCAL  <u8 slot>   ; assign local
22 OP_GET_UPVALUE <u8 idx>   ; push upvalue
23 OP_SET_UPVALUE <u8 idx>   ; assign upvalue

28 OP_ARRAY <u16 count>      ; build array from top N values
29 OP_OBJECT <u16 count>     ; build object from top 2N values (key/value)
2A OP_RANGE                  ; build array range from top 2 numbers (start,end)
2B OP_INDEX_GET              ; pop index, pop target, push value (errors if missing)
2C OP_INDEX_SET              ; pop value, index, target; assign (errors if missing)
2D OP_GET_PROP <u16 name>    ; pop target; push property value (errors if missing)
2E OP_SET_PROP <u16 name>    ; pop value, target; assign property (errors if missing)

30 OP_JUMP <u16 offset>      ; absolute jump
31 OP_JUMP_IF_FALSE <u16>    ; pop cond; if falsey, jump
32 OP_JUMP_IF_TRUE <u16>     ; pop cond; if truthy, jump

38 OP_CALL <u8 argc>         ; pop args, callee; push result
39 OP_RETURN                 ; return (value on stack or null if absent)
3A OP_CLOSURE <u16 proto> <u8 upcount> <up-desc...>
                              ; push closure from const proto; up-desc pairs: (isLocal? u8, index u8)

40 OP_NOP                    ; official no-op
41 OP_DEBUG                  ; reserved for future debug hook/no-op

48 OP_ITER_PREP              ; pop iterable, push iterator (errors if not iterable)
49 OP_ITER_NEXT <u16 jump>   ; iterator on stack; if has next -> push key?value and continue, else jump to offset
```

Notes:
- Built-ins occupy `0x80`–`0x9F` and are registered via `internal/builtins` (plug-in style).
- **Short-circuit**: `OP_AND`/`OP_OR` expect jump patching by compiler (emit conditional jumps around RHS).
- **Range literal**: compiler expands to `OP_RANGE`.
- **Global names**: referenced via constant string indices for compaction.
- **Call**: host functions and script functions share the call path; type-checked at runtime.

## Errors and limits
- Runtime errors include: type errors on operators, missing properties/indices (unless using safe builtins), out-of-bounds range operands, invalid call targets.
- VM enforces: max stack depth, max call depth, instruction limit (for timeouts), and heap guard hooks.

## Future adjustments
- Add specialized opcodes if profiling shows hotspots (e.g., concatenation, array push/pop).
- Optional const folding for literals and simple expressions in compiler.
- Debug hooks: optional `OP_DEBUG` no-op slots or offset callbacks from interpreter.
