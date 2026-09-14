package tulip

import (
	"fmt"
	"runtime"
	"unsafe"
)

// State is a live streaming-state handle: an opaque pointer to a boxed Rust
// indicator state, kept valid across batch calls and released exactly once
// by Close (which invokes the indicator's own `<name>_state_free`, supplied
// by the indicator package as freeFn).
//
// A finalizer closes leaked States as a backstop; prefer `defer`.
type State struct {
	handle uintptr
	id     uint32 // CIndicatorId (fnv1a32 of the indicator name)
	freeFn func(uintptr)
	closed bool
}

// NewState wraps a state handle as a user-closeable State (finalized).
func NewState(id uint32, handle uintptr, freeFn func(uintptr)) *State {
	s := &State{handle: handle, id: id, freeFn: freeFn}
	runtime.SetFinalizer(s, (*State).Close)
	return s
}

// newStateNoFinalizer is for handles owned by another wrapper (the lanes of
// a SimdResult): no finalizer — the owner's Close releases them.
func newStateNoFinalizer(id uint32, handle uintptr, freeFn func(uintptr)) *State {
	return &State{handle: handle, id: id, freeFn: freeFn}
}

// Handle returns the raw handle as a void* cgo argument for
// indicator-specific calls (batch continuations). Guard with Closed().
func (s *State) Handle() unsafe.Pointer { return stateAsArg(s.handle) }

// Closed reports whether Close has run.
func (s *State) Closed() bool { return s.closed }

// Close releases the boxed state. Idempotent.
func (s *State) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	runtime.SetFinalizer(s, nil)
	s.freeFn(s.handle)
	return nil
}

// Serialize snapshots the current state into an opaque TRFS blob. The blob
// is self-describing (its header embeds the indicator name) and
// copy-on-return: the native buffer is released inside this call, so the
// []byte is ordinary GC'd data — safe to store in a DB or file, and
// directly consumable by any other tulip_rs_ffi binding.
func (s *State) Serialize(format Format) ([]byte, error) {
	if s.closed {
		return nil, ErrClosed
	}
	return stateSerialize(s.id, uint32(format), s.handle)
}

// Clone returns an independent deep copy of the state (Rust `Clone` on the
// core side — no serde round-trip). Useful for snapshotting a stream or
// feeding concurrent batch lanes from one point.
func (s *State) Clone() (*State, error) {
	if s.closed {
		return nil, ErrClosed
	}
	h, err := stateClone(s.id, s.handle)
	if err != nil {
		return nil, err
	}
	return NewState(s.id, h, s.freeFn), nil
}

// DeserializeState rebuilds a state handle from a blob produced by
// Serialize. Header magic, schema version, format and the embedded
// indicator name are all validated on the Rust side; anything malformed
// returns an error, never a garbage handle.
//
// id/freeFn come from the indicator package restoring its own concrete
// state type; a blob written by a different indicator is rejected because
// its embedded name won't match what the registry derives from it.
func DeserializeState(id uint32, blob []byte, freeFn func(uintptr)) (*State, error) {
	if len(blob) == 0 {
		return nil, fmt.Errorf("tulip: empty blob")
	}
	h, err := stateDeserialize(blob)
	if err != nil {
		return nil, err
	}
	return NewState(id, h, freeFn), nil
}
