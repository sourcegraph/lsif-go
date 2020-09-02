package main

import "sync"

type cachedString struct {
	f     func() string
	value string
	once  sync.Once
}

func newCachedString(f func() string) *cachedString {
	return &cachedString{f: f}
}

func (cs *cachedString) Value() string {
	cs.once.Do(func() { cs.value = cs.f() })
	return cs.value
}
