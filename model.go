package flo

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

// ID represents an ID on Fluxer, which contains an embedded timestamp.
type ID uint64

var idEpoch = func() time.Time {
	time, err := time.Parse(time.DateOnly, "2015-01-01")
	if err != nil {
		panic(err)
	}

	return time
}()

// NewID creates a new dummy ID with the timestamp provided.
// Two IDs created with the same timestamp will be identical.
func NewID(timestamp time.Time) ID {
	return ID((timestamp.Sub(idEpoch).Milliseconds()) << 22)
}

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
// 0xRRGGBB can be used to create a color.
type ColorInt uint32

// String returns a hex color code string for the color.
func (c ColorInt) String() string {
	return fmt.Sprintf("#%06X", uint32(c))
}

// A Collection is a possibly-limited thread-safe set of Fluxer entities looked up by ID.
// The zero value does not allow anything to be inserted.
// Assigning a new collection to an existing one is not a thread-safe operation.
type Collection[T any] struct {
	limit   int
	lookup  map[ID]*collectionEntry[T]
	lru     *lruNode // least recently used
	lruTail *lruNode // most recently used
	mu      sync.RWMutex
}

// NewCollection creates a new collection with a limited amount of items.
// If the limit is reached, the least recently used item will be removed upon adding a new item.
// Passing a limit under 0 will result in a panic.
// Use [NewCollectionUnlimited] if you don't want to number of items to be limited.
func NewCollection[T any](limit int) Collection[T] {
	if limit < 0 {
		panic("limit may not be negative")
	}

	return Collection[T]{limit: limit}
}

// NewCollectionUnlimited creates a new collection which can hold an unlimited amount of items.
func NewCollectionUnlimited[T any]() Collection[T] {
	return Collection[T]{limit: -1}
}

type collectionEntry[T any] struct {
	val T
	lru *lruNode
}

type lruNode struct {
	id   ID
	prev *lruNode
	next *lruNode
}

// removeFromLRU removes an LRU node from the list, assming it is not nil.
func (c *Collection[T]) removeFromLRU(node *lruNode) {
	if node.prev != nil {
		node.prev.next = node.next
	}

	if node.next != nil {
		node.next.prev = node.prev
	}

	if c.lru == node {
		c.lru = node.next
	}

	if c.lruTail == node {
		c.lruTail = node.prev
	}
}

// moveToLRUTail moves an LRU node to the end of the list, assuming it is not nil.
func (c *Collection[T]) moveToLRUTail(node *lruNode) {
	if node.next == nil {
		// already at tail
		return
	}

	c.removeFromLRU(node)

	node.prev = c.lruTail
	node.next = nil

	c.lruTail = node
}

// addToLRUTailAndEvict creates a new LRU node at the end of the list. This should not be called if limit <= 0.
func (c *Collection[T]) addToLRUTailAndEvict(id ID) *lruNode {
	if len(c.lookup) > c.limit {
		panic("collection lookup size went over limit! something is very wrong!")
	} else if len(c.lookup) == c.limit {
		delete(c.lookup, c.lru.id)
		c.lru = c.lru.next
	}

	newTail := &lruNode{id: id, prev: c.lruTail}

	if c.lru == nil {
		c.lru = newTail
	}

	if c.lruTail != nil {
		c.lruTail.next = newTail
	}

	c.lruTail = newTail
	return newTail
}

// Limit returns the item limit if the collection is limited. Otherwise it returns -1, false.
func (c *Collection[T]) Limit() (int, bool) {
	if c.limit >= 0 {
		return c.limit, true
	} else {
		return -1, false
	}
}

// Len returns the number of items in the collection.
func (c *Collection[T]) Len() int {
	if c == nil || c.limit == 0 {
		return 0
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.lookup)
}

// IDs returns the item IDs in the collection. The order is effectively random.
func (c *Collection[T]) IDs() []ID {
	if c == nil || c.limit == 0 {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.lookup) == 0 {
		return nil
	}

	result := make([]ID, 0, len(c.lookup))
	for id := range c.lookup {
		result = append(result, id)
	}

	return result
}

// Get returns a value in the collection by ID.
// It marks the item as most recently used if a limit is set.
func (c *Collection[T]) Get(id ID) (T, bool) {
	if c.limit == 0 {
		var t T
		return t, false
	}

	if c.limit < 0 {
		c.mu.RLock()
		defer c.mu.RUnlock()

		if c.lookup == nil {
			var t T
			return t, false
		}

		entry, ok := c.lookup[id]
		if !ok {
			var t T
			return t, false
		}

		return entry.val, ok
	} else {
		c.mu.Lock()
		defer c.mu.Unlock()

		if c.lookup == nil {
			var t T
			return t, false
		}

		if entry, ok := c.lookup[id]; ok {
			if entry.lru != nil {
				c.moveToLRUTail(entry.lru)
			}

			return entry.val, true
		} else {
			var t T
			return t, false
		}
	}
}

// Contains returns true if an item with the specified ID is included in the collection.
// It does not update the item's recency.
func (c *Collection[T]) Contains(id ID) bool {
	if c.limit == 0 {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.lookup == nil {
		return false
	}

	_, ok := c.lookup[id]
	return ok
}

// Set adds or updates the item with the specified ID in the collection.
// It marks the item as most recently used if a limit is set.
func (c *Collection[T]) Set(id ID, val T) {
	if c.limit == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lookup == nil {
		c.lookup = map[ID]*collectionEntry[T]{}
	}

	if entry, ok := c.lookup[id]; ok {
		entry.val = val
		if entry.lru != nil {
			c.moveToLRUTail(entry.lru)
		}
		return
	}

	entry := &collectionEntry[T]{val: val}
	if c.limit > 0 {
		entry.lru = c.addToLRUTailAndEvict(id)
	}

	c.lookup[id] = entry
}

// Update allows safely updating the item with the specified ID from the collection through a pointer if it is present.
// It marks the item as most recently used if a limit is set.
// Copying the value is fine, but the pointer should not be copied outside the closure.
func (c *Collection[T]) Update(id ID, update func(val *T)) bool {
	if c.limit == 0 {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lookup == nil {
		return false
	}

	entry, ok := c.lookup[id]
	if !ok {
		return false
	}

	if entry.lru != nil {
		c.moveToLRUTail(entry.lru)
	}

	update(&entry.val)
	return true
}

// Upsert behaves like Update, but in the case where it returns false i.e. an item was not updated, it is added instead.
func (c *Collection[T]) Upsert(id ID, val T, update func(val *T)) bool {
	if c.limit == 0 {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lookup == nil {
		c.lookup = map[ID]*collectionEntry[T]{}
	}

	entry, ok := c.lookup[id]
	if !ok {
		entry := &collectionEntry[T]{val: val}
		if c.limit > 0 {
			entry.lru = c.addToLRUTailAndEvict(id)
		}

		c.lookup[id] = entry
		return false
	}

	if entry.lru != nil {
		c.moveToLRUTail(entry.lru)
	}

	update(&entry.val)
	return true
}

// Delete removes the item with the specified ID from the collection.
func (c *Collection[T]) Delete(id ID) (*T, bool) {
	if c.limit == 0 {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lookup == nil {
		return nil, false
	}

	entry, ok := c.lookup[id]
	if !ok {
		return nil, false
	}

	if entry.lru != nil {
		c.removeFromLRU(entry.lru)
	}

	delete(c.lookup, id)
	entry.lru = nil // avoid leaking
	return &entry.val, true
}

// Clear removes all items from the collection.
func (c *Collection[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lookup = nil
	c.lru = nil
	c.lruTail = nil
}
