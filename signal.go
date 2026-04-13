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
// You can also use OnSync to listen on whatever goroutine emitted it.
func (s *Signal[T]) On(f func(T)) ListenerRemoveFunc {
	return s.OnSync(func(t T) {
		go f(t)
	})
}

// Once adds a one-time listener to the the signal which will be spawned on a new goroutine.
// You can also use OnceSync to listen on whatever goroutine emitted it.
func (s *Signal[T]) Once(f func(T)) ListenerRemoveFunc {
	return s.OnceSync(func(t T) {
		go f(t)
	})
}

// OnSync adds a listener to the the signal which will be fired on the same gorountine as it is emitted.
func (s *Signal[T]) OnSync(f func(T)) ListenerRemoveFunc {
	return s.on(f, false)
}

// OnceSync adds a one-time listener to the the signal which will be fired on the same gorountine as it is emitted.
func (s *Signal[T]) OnceSync(f func(T)) ListenerRemoveFunc {
	return s.on(f, true)
}

func (s *Signal[T]) Chan() (<-chan T, ListenerRemoveFunc) {
	ch := make(chan T)
	return ch, s.On(func(t T) {
		go func() {
			ch <- t
		}()
	})
}

func (s *Signal[T]) OnceChan() (chan T, ListenerRemoveFunc) {
	result := make(chan T)
	return result, s.Once(func(t T) {
		go func() {
			<-result
		}()
	})
}

// ListenerRemoveFunc can be used to remove a listener by calling it.
type ListenerRemoveFunc func()

func (s *Signal[T]) on(f func(T), once bool) ListenerRemoveFunc {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.callbacks = append(s.callbacks, signalCallback[T]{
		id:   s.id,
		f:    f,
		once: once,
	})

	id := s.id
	s.id++
	return func() {
		s.remove(id)
	}
}

func (s *Signal[T]) remove(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := len(s.callbacks) - 1
	for ; idx >= 0; idx-- {
		if s.callbacks[idx].id == id {
			break
		}
	}

	if idx < 0 {
		return
	}

	s.callbacks = append(s.callbacks[:idx], s.callbacks[idx+1:]...)
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
