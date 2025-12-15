package compiler

// scope tracks locals and upvalues for nested functions.
type scope struct {
	enclosing *scope
	locals    map[string]uint8
	upvalues  []Upvalue
	nextLoc   uint8
}

func newScope(enclosing *scope) *scope {
	return &scope{
		enclosing: enclosing,
		locals:    make(map[string]uint8),
		upvalues:  []Upvalue{},
		nextLoc:   0,
	}
}

// addLocal reserves a slot for a local variable.
func (s *scope) addLocal(name string) uint8 {
	slot := s.nextLoc
	s.locals[name] = slot
	s.nextLoc++
	return slot
}

// resolveLocal returns slot and true if found in current scope.
func (s *scope) resolveLocal(name string) (uint8, bool) {
	slot, ok := s.locals[name]
	return slot, ok
}

// resolveUpvalue walks enclosing scopes to find a name, capturing it if needed.
func (s *scope) resolveUpvalue(name string) (Upvalue, bool) {
	if s.enclosing == nil {
		return Upvalue{}, false
	}
	if slot, ok := s.enclosing.resolveLocal(name); ok {
		up := Upvalue{IsLocal: true, Index: slot}
		s.upvalues = append(s.upvalues, up)
		return Upvalue{IsLocal: false, Index: uint8(len(s.upvalues) - 1)}, true
	}
	if up, ok := s.enclosing.resolveUpvalue(name); ok {
		s.upvalues = append(s.upvalues, up)
		return Upvalue{IsLocal: false, Index: uint8(len(s.upvalues) - 1)}, true
	}
	return Upvalue{}, false
}
