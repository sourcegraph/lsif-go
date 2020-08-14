package indexer

import "sync"

type MutexMap struct {
	mutex sync.RWMutex
	m     map[string]*sync.RWMutex
}

func newMutexMap() *MutexMap {
	return &MutexMap{
		m: map[string]*sync.RWMutex{},
	}
}

func (m *MutexMap) Lock(v string)    { m.mutexFor(v).Lock() }
func (m *MutexMap) Unlock(v string)  { m.mutexFor(v).Unlock() }
func (m *MutexMap) RLock(v string)   { m.mutexFor(v).RLock() }
func (m *MutexMap) RUnlock(v string) { m.mutexFor(v).RUnlock() }

func (m *MutexMap) mutexFor(v string) *sync.RWMutex {
	m.mutex.RLock()
	mutex, ok := m.m[v]
	m.mutex.RUnlock()
	if ok {
		return mutex
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if mutex, ok := m.m[v]; ok {
		return mutex
	}

	var newMutex sync.RWMutex
	m.m[v] = &newMutex
	return &newMutex
}
