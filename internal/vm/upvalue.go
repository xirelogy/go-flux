package vm

// upvalue represents a captured variable slot.
// It points to a live local until the owning frame is closed,
// after which the value is stored in closed.
type upvalue struct {
	location *Value
	closed   Value
}

func newUpvalue(slot *Value) *upvalue {
	return &upvalue{location: slot}
}

func (uv *upvalue) get() Value {
	if uv == nil {
		return Null()
	}
	if uv.location != nil {
		return *uv.location
	}
	return uv.closed
}

func (uv *upvalue) set(v Value) {
	if uv == nil {
		return
	}
	if uv.location != nil {
		*uv.location = v
		return
	}
	uv.closed = v
}

func (uv *upvalue) close() {
	if uv.location != nil {
		uv.closed = *uv.location
		uv.location = nil
	}
}
