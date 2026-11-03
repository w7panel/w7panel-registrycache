package helper

import (
	"hash/fnv"
	"sync"
)

// StripedMutex serializes operations for a key with a fixed memory footprint.
// Keys can share a stripe. Do not copy after first use or recursively lock keys.
// The zero value is ready to use.
type StripedMutex struct{ locks [256]sync.Mutex }

// Lock acquires the key's stripe and returns the corresponding unlock function.
func (m *StripedMutex) Lock(key string) func() {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	lock := &m.locks[hash.Sum32()%uint32(len(m.locks))]
	lock.Lock()
	return lock.Unlock
}
