package flo

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/simplelru"
)

// ID represents an ID on Fluxer, which contains an embedded timestamp.
// The zero value is often used to represent the absence of an ID.
type ID uint64

var idEpoch = func() time.Time {
	time, err := time.Parse(time.DateOnly, "2015-01-01")
	if err != nil {
		panic(err)
	}

	return time
}()

func (id ID) CreatedAt() time.Time {
	return idEpoch.Add(time.Millisecond * time.Duration(id>>22))
}

func (id ID) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, `"%d"`, id), nil
}

func (id *ID) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("expected JSON string")
	}

	unquoted := data[1 : len(data)-1]
	result, err := strconv.ParseUint(string(unquoted), 10, 64)
	if err != nil {
		return err
	}

	*id = ID(result)
	return nil
}

// ColorInt represents a Fluxer RGB color value.
// ColorInt(0xRRGGBB) can be used to create a color.
type ColorInt uint32

// A Collection is a thread-safe set of Fluxer entities looked up by ID.
// New methods may be added without a major release!
// Creating your own implementation of this interface is not supported and is done at your own risk!
type Collection[T any] interface {
	// Len returns the number of items in the collection.
	Len() int
	// Keys returns a list of the IDs in the collection.
	// No particular order is guaranteed.
	Keys() []ID
	Get(key ID) (T, bool)
	Contains(key ID) bool

	Set(key ID, val T)
	// Update allows safely updating an item while preventing any concurrent read/write access.
	Update(key ID, update func(val *T)) bool
	Delete(key ID) bool

	// NOTE: the reason for not providing any iteration methods is that it seems like a bit of a footgun
	// if long running functions are called in the loop due to locking
}

// NewCollection returns a collection which will not automatically delete any items.
func NewCollection[T any]() Collection[T] {
	return &unlimitedCollection[T]{
		lookup: map[ID]*T{},
	}
}

// NewLimitedCollection returns a collection which will hold up to the provided count of items before removing older times.
func NewLimitedCollection[T any](limit int) (Collection[T], error) {
	lookup, err := simplelru.NewLRU[ID, *T](limit, nil)
	if err != nil {
		return nil, err
	}

	return &limitedCollection[T]{lookup: *lookup}, nil
}

type unlimitedCollection[T any] struct {
	lookup map[ID]*T
	mu     sync.RWMutex
}

func (c *unlimitedCollection[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.lookup)
}

func (c *unlimitedCollection[T]) Keys() []ID {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]ID, 0, len(c.lookup))
	for key := range c.lookup {
		result = append(result, key)
	}

	return result
}

func (c *unlimitedCollection[T]) Get(key ID) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.lookup[key]
	if !ok {
		var t T
		return t, false
	}

	return *val, true
}

func (c *unlimitedCollection[T]) Contains(key ID) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.lookup[key]
	return ok
}

func (c *unlimitedCollection[T]) Set(key ID, val T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lookup[key] = &val
}

func (c *unlimitedCollection[T]) Update(key ID, update func(val *T)) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if val, ok := c.lookup[key]; ok {
		update(val)
		return true
	} else {
		return false
	}
}

func (c *unlimitedCollection[T]) Delete(key ID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.lookup[key]; ok {
		delete(c.lookup, key)
		return true
	} else {
		return false
	}
}

type limitedCollection[T any] struct {
	lookup simplelru.LRU[ID, *T]
	mu     sync.RWMutex
}

func (c *limitedCollection[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lookup.Len()
}

func (c *limitedCollection[T]) Keys() []ID {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lookup.Keys()
}

func (c *limitedCollection[T]) Get(key ID) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.lookup.Get(key)
	if !ok {
		var t T
		return t, false
	}

	return *val, true
}

func (c *limitedCollection[T]) Contains(key ID) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lookup.Contains(key)
}

func (c *limitedCollection[T]) Set(key ID, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lookup.Add(key, &value)
}

func (c *limitedCollection[T]) Update(key ID, update func(value *T)) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if val, ok := c.lookup.Get(key); ok {
		update(val)
		return true
	} else {
		return false
	}
}

func (c *limitedCollection[T]) Delete(key ID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.lookup.Remove(key)
}
