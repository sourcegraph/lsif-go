package indexer

import (
	"sync"
)

const NumLockStripes = 512

type StripedMutex struct {
	mutex sync.RWMutex
	keys  map[string]uint64
	locks []*sync.RWMutex
}

func newStripedMutex() *StripedMutex {
	locks := make([]*sync.RWMutex, NumLockStripes)
	for i := range locks {
		locks[i] = &sync.RWMutex{}
	}

	return &StripedMutex{
		keys:  map[string]uint64{},
		locks: locks,
	}
}

func (m *StripedMutex) LockKey(v string)    { m.mutexForKey(v).Lock() }
func (m *StripedMutex) UnlockKey(v string)  { m.mutexForKey(v).Unlock() }
func (m *StripedMutex) RLockKey(v string)   { m.mutexForKey(v).RLock() }
func (m *StripedMutex) RUnlockKey(v string) { m.mutexForKey(v).RUnlock() }

func (m *StripedMutex) mutexForIndex(v uint64) *sync.RWMutex {
	return m.locks[int(v)%len(m.locks)]
}

func (m *StripedMutex) mutexForKey(v string) *sync.RWMutex {
	return m.mutexForIndex(m.indexFor(v))
}

func (m *StripedMutex) indexFor(v string) uint64 {
	m.mutex.RLock()
	key, ok := m.keys[v]
	m.mutex.RUnlock()
	if ok {
		return key
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if key, ok := m.keys[v]; ok {
		return key
	}

	key = uint64(len(m.keys))
	m.keys[v] = key
	return key
}
