/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"maps"
	"sync"
)

// SyncMap is a map protected by a mutex.
type SyncMap[K comparable, V any] struct {
	m   map[K]V
	mux sync.Mutex
}

// Load returns the value stored in the map for a key, or nil if no value is present. The ok result indicates whether value was found in the map.
func (sm *SyncMap[K, V]) Load(key K) (value V, ok bool) {
	sm.mux.Lock()
	if sm.m != nil {
		value, ok = sm.m[key]
	}
	sm.mux.Unlock()
	return value, ok
}

// Store sets the value for a key.
func (sm *SyncMap[K, V]) Store(key K, value V) {
	sm.mux.Lock()
	if sm.m == nil {
		sm.m = make(map[K]V, 128)
	}
	sm.m[key] = value
	sm.mux.Unlock()
}

// Delete deletes the value for a key.
func (sm *SyncMap[K, V]) Delete(key K) (value V, deleted bool) {
	sm.mux.Lock()
	if sm.m != nil {
		value, deleted = sm.m[key]
		delete(sm.m, key)
	}
	sm.mux.Unlock()
	return value, deleted
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value. The loaded result is true if the value was loaded, false if stored.
func (sm *SyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	return sm.LoadOrStoreFunc(key, func() V { return value })
}

// LoadOrStoreFunc returns the existing value for the key if present.
// Otherwise, it stores and returns the given value. The loaded result is true if the value was loaded, false if stored.
func (sm *SyncMap[K, V]) LoadOrStoreFunc(key K, value func() V) (actual V, loaded bool) {
	sm.mux.Lock()
	if sm.m == nil {
		sm.m = make(map[K]V, 128)
	}
	actual, ok := sm.m[key]
	if ok {
		sm.mux.Unlock()
		return actual, true
	}
	newValue := value()
	sm.m[key] = newValue
	sm.mux.Unlock()
	return newValue, false
}

// Keys returns all keys.
func (sm *SyncMap[K, V]) Keys() (keys []K) {
	sm.mux.Lock()
	for k := range sm.m {
		keys = append(keys, k)
	}
	sm.mux.Unlock()
	return keys
}

// Values returns all values.
func (sm *SyncMap[K, V]) Values() (values []V) {
	sm.mux.Lock()
	for _, v := range sm.m {
		values = append(values, v)
	}
	sm.mux.Unlock()
	return values
}

// Snapshot returns a shallow copy of the internal map.
func (sm *SyncMap[K, V]) Snapshot() (copy map[K]V) {
	copy = make(map[K]V)
	sm.mux.Lock()
	maps.Copy(copy, sm.m)
	sm.mux.Unlock()
	return copy
}

// DoUnderLock obtains a lock and passes the internal map to the callback.
func (sm *SyncMap[K, V]) DoUnderLock(callback func(m map[K]V)) {
	sm.mux.Lock()
	if sm.m == nil {
		sm.m = make(map[K]V, 128)
	}
	callback(sm.m)
	sm.mux.Unlock()
}

// Len is the number of elements in the map.
func (sm *SyncMap[K, V]) Len() (n int) {
	sm.mux.Lock()
	n = len(sm.m)
	sm.mux.Unlock()
	return n
}
