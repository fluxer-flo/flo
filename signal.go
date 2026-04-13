package flo

import (
	"slices"
	"sync"
)

// A Signal can have listeners attached which are called when it is emitted. Adding a listener is done in a thread-safe manner.
type Signal[T any] struct {
	mu        sync.RWMutex
	id        uint64
	callbacks []signalCallback[T]
}

type signalCallback[T any] struct {
	id   uint64
	f    func(T)
	once bool
}

// On adds a listener to the the signal which will be spawned on a new goroutine.
// It can be removed by calling [SignalListener.Remove] on the result.
// You can also use OnSync to listen on whatever goroutine emitted it.
func (s *Signal[T]) On(f func(T)) SignalListener[T] {
	return s.OnSync(func(t T) {
		go f(t)
	})
}

// Once adds a one-time listener to the the signal which will be spawned on a new goroutine.
// It can be removed by calling [SignalListener.Remove] on the result.
// You can also use OnceSync to listen on whatever goroutine emitted it.
func (s *Signal[T]) Once(f func(T)) SignalListener[T] {
	return s.OnceSync(func(t T) {
		go f(t)
	})
}

// OnSync adds a listener to the the signal which will be fired on the same gorountine as it is emitted.
// It can be removed by calling [SignalListener.Remove] on the result.
func (s *Signal[T]) OnSync(f func(T)) SignalListener[T] {
	return s.on(f, false)
}

// OnceSync adds a one-time listener to the the signal which will be fired on the same gorountine as it is emitted.
// It can be removed by calling [SignalListener.Remove] on the result.
func (s *Signal[T]) OnceSync(f func(T)) SignalListener[T] {
	return s.on(f, true)
}

func (s *Signal[T]) Chan() (<-chan T, SignalListener[T]) {
	ch := make(chan T)
	return ch, s.On(func(t T) {
		go func() {
			ch <- t
		}()
	})
}

func (s *Signal[T]) OnceChan() (chan T, SignalListener[T]) {
	result := make(chan T)
	return result, s.Once(func(t T) {
		go func() {
			<-result
		}()
	})
}

// SignalListener is a handle for an added listener which can be used to remove it.
type SignalListener[T any] struct {
	signal  *Signal[T]
	id      uint64
	removed bool
}

func (s *Signal[T]) on(f func(T), once bool) SignalListener[T] {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.callbacks = append(s.callbacks, signalCallback[T]{
		id:   s.id,
		f:    f,
		once: once,
	})

	result := SignalListener[T]{
		signal: s,
		id:     s.id,
	}

	s.id++
	return result
}

// Remove removes the listener from the signal if it has not already been removed - returning true if so.
func (l *SignalListener[T]) Remove() bool {
	l.signal.mu.Lock()
	defer l.signal.mu.Unlock()

	if l.removed {
		return false
	}

	l.removed = true

	idx := len(l.signal.callbacks) - 1
	for ; idx >= 0; idx-- {
		if l.signal.callbacks[idx].id == l.id {
			break
		}
	}

	if idx < 0 {
		return false
	}

	l.signal.callbacks = append(l.signal.callbacks[:idx], l.signal.callbacks[idx+1:]...)
	return true
}

// ClearListeners removes all listeners from the signal.
func (s *Signal[T]) ClearListeners() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.callbacks = nil
}

func (s *Signal[T]) emit(val T) {
	s.mu.Lock()
	callbacks := make([]signalCallback[T], 0, len(s.callbacks))
	callbacks = append(callbacks, s.callbacks...)

	s.callbacks = slices.DeleteFunc(s.callbacks, func(callback signalCallback[T]) bool {
		return callback.once
	})
	s.mu.Unlock()

	for _, callback := range callbacks {
		callback.f(val)
	}
}
