package helper

import "container/heap"

// IndexedMinHeap orders values by priority and keeps one entry per key.
// Updating or removing an entry takes O(log n). Callers provide synchronization.
type IndexedMinHeap[K comparable, V any] struct {
	items indexedHeapItems[K, V]
	byKey map[K]*indexedHeapEntry[K, V]
}

type indexedHeapEntry[K comparable, V any] struct {
	key   K
	value V
	index int
}

type indexedHeapItems[K comparable, V any] struct {
	entries []*indexedHeapEntry[K, V]
	less    func(V, V) bool
}

func NewIndexedMinHeap[K comparable, V any](less func(V, V) bool) *IndexedMinHeap[K, V] {
	if less == nil {
		panic("helper: min heap comparator must not be nil")
	}
	return &IndexedMinHeap[K, V]{items: indexedHeapItems[K, V]{less: less}, byKey: map[K]*indexedHeapEntry[K, V]{}}
}

func (h *IndexedMinHeap[K, V]) Len() int { return len(h.items.entries) }

func (h *IndexedMinHeap[K, V]) Peek() (V, bool) {
	if h.Len() == 0 {
		var zero V
		return zero, false
	}
	return h.items.entries[0].value, true
}

func (h *IndexedMinHeap[K, V]) Upsert(key K, value V) {
	if entry := h.byKey[key]; entry != nil {
		entry.value = value
		heap.Fix(&h.items, entry.index)
		return
	}
	entry := &indexedHeapEntry[K, V]{key: key, value: value}
	heap.Push(&h.items, entry)
	h.byKey[key] = entry
}

func (h *IndexedMinHeap[K, V]) Remove(key K) bool {
	entry := h.byKey[key]
	if entry == nil {
		return false
	}
	heap.Remove(&h.items, entry.index)
	delete(h.byKey, key)
	return true
}

func (h *IndexedMinHeap[K, V]) Pop() (V, bool) {
	if h.Len() == 0 {
		var zero V
		return zero, false
	}
	entry := heap.Pop(&h.items).(*indexedHeapEntry[K, V])
	delete(h.byKey, entry.key)
	return entry.value, true
}

func (h indexedHeapItems[K, V]) Len() int { return len(h.entries) }
func (h indexedHeapItems[K, V]) Less(i, j int) bool {
	return h.less(h.entries[i].value, h.entries[j].value)
}
func (h indexedHeapItems[K, V]) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
	h.entries[i].index = i
	h.entries[j].index = j
}
func (h *indexedHeapItems[K, V]) Push(value any) {
	entry := value.(*indexedHeapEntry[K, V])
	entry.index = len(h.entries)
	h.entries = append(h.entries, entry)
}
func (h *indexedHeapItems[K, V]) Pop() any {
	last := len(h.entries) - 1
	entry := h.entries[last]
	h.entries[last] = nil
	h.entries = h.entries[:last]
	entry.index = -1
	return entry
}
