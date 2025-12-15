# go-flux Language Spec (draft)

This document captures the initial grammar and semantics for the go-flux scripting language. The goal is a small, embeddable language that is easy to parse to an AST and execute on a VM.

## Lexical structure
- **Whitespace** (other than newlines) is insignificant outside of literals; tabs are recommended but not required.
- **Case sensitivity**
  - The language is case-sensitive: keywords, builtins, and identifiers must match exact case.
- **Comments**
  - Line comments: `// ...` to end of line.
  - Block comments: `/* ... */`, non-nesting.
- **Statement separation**
  - Statements end at a newline or closing `}`/EOF. There are no semicolons in the language.
  - Newlines inside `(...)`, `[...]`, `{...}` do not end a statement; trailing operators/delimiters (`,`, `..`, `.`, `(`, `[`) continue onto the next line.
- **Identifiers**
  - Global function names: `[A-Za-z_][A-Za-z0-9_]*` (no `$` prefix).
  - Variable names: `$` followed by identifier chars (`$foo`, `$x1`).
  - Property names in object literals may be identifiers, string literals, or numeric literals.
- **Literals**
  - `null`, `true`, `false`
  - Numbers: decimal integers or floats (`123`, `5.88`, `0.5`, `42.0`). Sign may be applied via unary `+` or `-`.
  - Strings: double-quoted `"..."` with standard escape sequences `\" \\ \n \r \t \b \f`.
  - Arrays: `[ expr_list_opt ]` with optional trailing comma.
  - Objects: `{ object_fields_opt }` with optional trailing comma; keys are identifier | string literal | numeric literal.

## Types
Dynamic types: `null`, `boolean`, `number`, `string`, `array`, `object`, `function`, `error`.

## Expressions
- Primary: literals, variables, parenthesized expressions.
- Arrays: `[ expr (, expr)* ,? ]`
- Objects: `{ object_field (, object_field)* ,? }` where `object_field` is `key : expr` and `key` is identifier | string | number.
- Property access: `expr . identifier`
- Indexing: `expr [ expression ]` for array/object element access.
- Function expression (anonymous): `func ( params_opt ) block`
- Function call: `expr ( args_opt )`
- Assignment: `lvalue assign_op expr` where `assign_op` is `=` or `:=`.
  - `:=` introduces/initializes; `=` mutates existing variable or property.
  - `lvalue` is variable, property access, or indexed access (`$x`, `$obj.prop`, `$arr[$i]`).
- Arithmetic: `+ - * /`
- Comparison: `== != < > <= >=`
- Grouping: `(` `)`
- Builtins: `error("description")`, `typeof(expr)`, `indexExist(target, index)`, `indexRead(target, index, default)`, `valueExist(array, value)`
- Return value defaults to `null` if no `return` executed.
- Range literal: `[ start .. end ]` produces an array of numbers from `start` to `end` inclusive. Both `start` and `end` expressions must evaluate to integers (otherwise a runtime error). Step is `+1` if `start <= end`, otherwise `-1`.

### Operator precedence (high to low)
1) Calls, property/index access: `expr(...)`, `expr.identifier`, `expr[expr]`
2) Unary: `+ - !` (prefix)
3) Multiplicative: `* /`
4) Additive: `+ -`
5) Comparison: `< > <= >= == !=`
6) Logical: `&&` then `||` (left-associative)
7) Assignment: `=` `:=` (right-associative)

### Grammar (EBNF-style)
```
program         := statement*

statement       := block
                 | if_stmt
                 | while_stmt
                 | for_stmt
                 | return_stmt
                 | expr_stmt
                 | func_decl

block           := "{" statement* "}"

if_stmt         := "if" "(" expression ")" block ("elseif" "(" expression ")" block)* ("else" block)?
while_stmt      := "while" "(" expression ")" block
for_stmt        := "for" "(" for_binding "in" expression ")" block
for_binding     := variable | "[" variable "," variable "]"
return_stmt     := "return" expression?
expr_stmt       := expression

func_decl       := "func" identifier "(" param_list? ")" block

expression      := assignment
assignment      := logical_or (assign_op assignment)?
assign_op       := "=" | ":="

logical_or      := logical_and ( "||" logical_and )*
logical_and     := equality ( "&&" equality )*
equality        := comparison (("==" | "!=") comparison)*
comparison      := addition (("<" | ">" | "<=" | ">=") addition)*
addition        := multiplication (("+" | "-") multiplication)*
multiplication  := unary (("*" | "/") unary)*
unary           := (("+" | "-" | "!") unary) | postfix
postfix         := primary (call | prop_access | index)*
call            := "(" arg_list? ")"
prop_access     := "." identifier
index           := "[" expression "]"

primary         := literal
                 | variable
                 | func_expr
                 | "(" expression ")"

literal         := "null" | "true" | "false" | number | string | array_lit | range_lit | object_lit
array_lit       := "[" (expression ("," expression)* ","?)? "]"                // elements
range_lit       := "[" expression ".." expression "]"                         // inclusive numeric range → array
object_lit      := "{" (object_field ("," object_field)* ","?)? "}"
object_field    := object_key ":" expression
object_key      := identifier | string | number

func_expr       := "func" "(" param_list? ")" block
param_list      := variable ("," variable)*
arg_list        := expression ("," expression)*

variable        := "$" identifier
identifier      := /[A-Za-z_][A-Za-z0-9_]*/
number          := /[0-9]+(\\.[0-9]+)?/
string          := /"(\\"|\\\\|\\n|\\r|\\t|\\b|\\f|[^"])*"/
```

Notes:
- `program` is a sequence of statements; top-level functions are declared with `func name(...) { ... }`.
- Statements are separated by newlines or closing `}`/EOF; newlines inside `()`, `[]`, `{}` do not terminate a statement.
- `for` loops iterate `for ( binding in expr )` where `binding` is `$v` or `[$k, $v]` as defined above.
- Trailing commas are allowed in array and object literals.
- `:=` is intended for variable introduction; `=` for reassignment or property writes.
- All functions are first-class values. If no `return` executes, the function yields `null`.
- Range literals use `[..]`; when `..` appears between two expressions inside brackets, it parses as a range rather than an array literal.

## Statements and control flow
- **Blocks**: `{ ... }` group multiple statements.
- **If / Elseif / Else**: standard conditional branching with parenthesized conditions.
- **While**: pre-condition loop.
- **For** (iterable): `for ( $v in expr ) { ... }` loops over an iterable; `$v` binds to each element value.
  - Key/value form: `for ( [$k, $v] in expr ) { ... }` binds key/index to `$k` and value to `$v`.
  - Iteration order: arrays iterate from index `0` upward; objects iterate insertion order of properties.
- **Return**: `return expr` or bare `return` (implies `null`); terminated by newline or block end.
- **Expression statement**: any expression used as a statement; terminated by newline or block end.
- **Boolean logic**: `&&`, `||` are short-circuiting; unary `!` negates truthiness.

Iterable sources: arrays and objects are iterable by default. Numeric ranges use the built-in range literal `[start .. end]`, yielding an array and inheriting the iteration rules of arrays.

## Functions
- **Declarations**: `func add($a, $b) { return $a + $b }` define global functions (invocable from host).
- **Expressions**: `func ($x) { return $x * 2 }` produce first-class function values.
- **Methods on objects**: assign functions as properties, directly or later via dot access.
  - Inline: `$obj = { minus: func ($a, $b) { return $a - $b }, }`
  - After creation: `$obj.minus = func ($a, $b) { return $a - $b }`

## Builtins

### error
`error(description)`  
Constructs an `error` value with the provided description (string). Returning or propagating this value signals failure semantics to the VM/host.

### typeof
`typeof(value)`  
Returns a string naming the dynamic type of `value`: `"null"`, `"boolean"`, `"number"`, `"string"`, `"array"`, `"object"`, `"function"`, or `"error"`.

### indexExist
`indexExist(target, index)`  
Returns `true` if `index` exists on `target` (array in-bounds, object key present); otherwise `false`. Raises a runtime error if `target` is neither array nor object.

### indexRead
`indexRead(target, index, defaultValue)`  
Returns `target[index]` for arrays/objects. If the index/key is missing or out-of-bounds, returns `defaultValue` without raising an error. Raises a runtime error if `target` is neither array nor object.

### valueExist
`valueExist(array, value)`  
Returns `true` if `value` is present in `array` using standard equality rules; otherwise `false`. Raises a runtime error if `array` is not an array.

Functions default to returning `null` when no explicit return value is provided.

## Property access and mutation
- Read: `$obj.prop`
- Read via index: `$arr[$i]` or `$obj[$key]`
- Write: `$obj.prop = expr`
- Write via index: `$arr[$i] = expr` or `$obj[$key] = expr`
- Nested property chains are allowed (`$a.b.c`).
- Indexing with `[]` on arrays/objects throws a runtime error when the index/key is missing or out-of-bounds; use `indexExist`/`indexRead` for safe checks/access.

## Program shape
- Typical scripts consist of global function declarations. The host embeds the VM and invokes entrypoint functions by name.

## Example
Business rule: deny login outside 07:00–18:00 for users in Sales or Administration.
```flux
func onCheckAuth($context, $userProfile) {
  // helper to see if user is in a restricted department
  $isRestricted := valueExist($userProfile.departments, "Sales") ||
                   valueExist($userProfile.departments, "Administration")

  if ($isRestricted && !$context.timeOfDayWithin("07:00", "18:00")) {
    return false
  }

  return true
}
```

## Non-features
- Import/module system is intentionally absent; scripts are single compilation units loaded by the host. Any composition is handled by the embedding environment.
