package vm

import "fmt"

type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindNumber
	KindString
	KindArray
	KindObject
	KindFunction
	KindError
	KindIterator
)

type Value struct {
	Kind Kind
	Num  float64
	Str  string
	Arr  []Value
	Obj  map[string]Value
	Func *Function
	Err  string
	It   *Iterator
	B    bool
	// ReadOnly marks array/object containers as immutable from script code.
	ReadOnly bool
}

func Null() Value { return Value{Kind: KindNull} }
func Bool(b bool) Value {
	return Value{Kind: KindBool, B: b}
}
func Number(n float64) Value {
	return Value{Kind: KindNumber, Num: n}
}
func String(s string) Value {
	return Value{Kind: KindString, Str: s}
}
func Array(v []Value) Value {
	return Value{Kind: KindArray, Arr: v}
}
func Object(m map[string]Value) Value {
	return Value{Kind: KindObject, Obj: m}
}
func ErrorVal(s string) Value {
	return Value{Kind: KindError, Err: s}
}
func IteratorVal(it *Iterator) Value {
	return Value{Kind: KindIterator, It: it}
}

func Truthy(v Value) bool {
	switch v.Kind {
	case KindNull:
		return false
	case KindBool:
		return v.B
	default:
		return true
	}
}

func Equal(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindNull:
		return true
	case KindBool:
		return a.B == b.B
	case KindNumber:
		return a.Num == b.Num
	case KindString:
		return a.Str == b.Str
	case KindError:
		return a.Err == b.Err
	default:
		return &a == &b
	}
}

// Iterator supports array/object iteration.
type Iterator struct {
	arr   []Value
	obj   map[string]Value
	keys  []string
	index int
}

func NewArrayIterator(arr []Value) *Iterator {
	return &Iterator{arr: arr, index: 0}
}

func NewObjectIterator(obj map[string]Value) *Iterator {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return &Iterator{obj: obj, keys: keys, index: 0}
}

// Next returns key,value and ok.
func (it *Iterator) Next() (string, Value, bool) {
	if it.arr != nil {
		if it.index >= len(it.arr) {
			return "", Value{}, false
		}
		v := it.arr[it.index]
		k := it.index
		it.index++
		return stringIndex(k), v, true
	}
	if it.obj != nil {
		if it.index >= len(it.keys) {
			return "", Value{}, false
		}
		k := it.keys[it.index]
		v := it.obj[k]
		it.index++
		return k, v, true
	}
	return "", Value{}, false
}

func stringIndex(i int) string {
	return fmt.Sprintf("%d", i)
}
