package vm

import (
	"fmt"
	"strings"

	"github.com/xirelogy/go-flux/internal/bytecode"
)

// TraceInfo describes a single instruction dispatch for debugging/tracing.
type TraceInfo struct {
	Op       byte
	Function string
	Source   string
	Line     int
	IP       int
}

// TraceHook observes instruction dispatch for debugging/profiling.
type TraceHook func(TraceInfo)

// FrameInfo captures the call frame at the time of an error or trace event.
type FrameInfo struct {
	Function string
	Source   string
	Line     int
	IP       int
}

// RuntimeError carries source/stack information for VM failures.
type RuntimeError struct {
	Message string
	Frame   FrameInfo
	Stack   []FrameInfo
	Cause   error
}

func (e *RuntimeError) Error() string {
	locParts := []string{}
	if e.Frame.Source != "" {
		if e.Frame.Line > 0 {
			locParts = append(locParts, fmt.Sprintf("%s:%d", e.Frame.Source, e.Frame.Line))
		} else {
			locParts = append(locParts, e.Frame.Source)
		}
	} else if e.Frame.Line > 0 {
		locParts = append(locParts, fmt.Sprintf("line %d", e.Frame.Line))
	}
	if e.Frame.Function != "" {
		locParts = append(locParts, fmt.Sprintf("in %s", e.Frame.Function))
	}
	loc := strings.Join(locParts, " ")
	if loc != "" {
		return fmt.Sprintf("%s: %s", loc, e.Message)
	}
	return e.Message
}

// Unwrap exposes the original error, if any.
func (e *RuntimeError) Unwrap() error {
	return e.Cause
}

func (vm *VM) errorf(fr *frame, format string, args ...interface{}) (Value, error) {
	msg := fmt.Sprintf(format, args...)
	err := vm.newRuntimeError(fr, vm.offsetForFrame(fr), msg, nil)
	return ErrorVal(err.Error()), err
}

func (vm *VM) wrapError(fr *frame, val Value, err error) (Value, error) {
	if err == nil {
		return val, nil
	}
	if _, ok := err.(*RuntimeError); !ok {
		err = vm.newRuntimeError(fr, vm.offsetForFrame(fr), err.Error(), err)
	}
	if val.Kind != KindError {
		val = ErrorVal(err.Error())
	}
	return val, err
}

func (vm *VM) newRuntimeError(fr *frame, offset int, msg string, cause error) *RuntimeError {
	frameInfo := vm.frameInfo(fr, offset)
	stack := vm.stackTrace(fr, offset)
	return &RuntimeError{
		Message: msg,
		Frame:   frameInfo,
		Stack:   stack,
		Cause:   cause,
	}
}

func (vm *VM) trace(fr *frame, op byte) {
	if vm.traceHook == nil {
		return
	}
	info := vm.frameInfo(fr, vm.offsetForFrame(fr))
	vm.traceHook(TraceInfo{
		Op:       op,
		Function: info.Function,
		Source:   info.Source,
		Line:     info.Line,
		IP:       info.IP,
	})
}

func (vm *VM) stackTrace(current *frame, offset int) []FrameInfo {
	if len(vm.frames) == 0 {
		return nil
	}
	trace := make([]FrameInfo, 0, len(vm.frames))
	for i := len(vm.frames) - 1; i >= 0; i-- {
		fr := &vm.frames[i]
		off := fr.lastOp
		if fr == current && offset >= 0 {
			off = offset
		}
		trace = append(trace, vm.frameInfo(fr, off))
	}
	return trace
}

func (vm *VM) frameInfo(fr *frame, offset int) FrameInfo {
	if fr == nil || fr.fn == nil {
		return FrameInfo{}
	}
	name := fr.fn.Name
	src := fr.fn.Source
	if src == "" && fr.fn.Proto != nil {
		src = fr.fn.Proto.Source
	}
	line := 0
	if fr.fn.Proto != nil && fr.fn.Proto.Chunk != nil {
		line = lineForOffset(fr.fn.Proto.Chunk, offset)
	}
	return FrameInfo{
		Function: name,
		Source:   src,
		Line:     line,
		IP:       offset,
	}
}

func (vm *VM) offsetForFrame(fr *frame) int {
	if fr == nil {
		return -1
	}
	if fr.lastOp >= 0 {
		return fr.lastOp
	}
	return fr.ip
}

func lineForOffset(chunk *bytecode.Chunk, offset int) int {
	if chunk == nil || offset < 0 {
		return 0
	}
	line := 0
	for _, info := range chunk.Lines {
		if offset < info.Offset {
			break
		}
		line = info.Line
	}
	return line
}
