package sessionmw

import "sync"

// MemStore is an in-memory session store.
type MemStore struct {
	sync.RWMutex
	Data map[string]interface{}
}

// NewMemStore creates a sessionmw.Store compatiable in-memory session store.
func NewMemStore() *MemStore {
	return &MemStore{
		Data: make(map[string]interface{}),
	}
}

// Get retrieves the session for the provided id from the in-memory session
// store.
func (ms *MemStore) Get(id string) (map[string]interface{}, error) {
	ms.RLock()
	defer ms.RUnlock()

	if sess, ok := ms.Data[id]; ok {
		return sess.(map[string]interface{}), nil
	}

	return nil, ErrSessionNotFound
}

// Save saves the session for the provided id from the in-memory session store.
//
// If the provided id already exists in storage, then it will be overwritten.
func (ms *MemStore) Save(id string, sess map[string]interface{}) error {
	ms.Lock()
	ms.Data[id] = sess
	ms.Unlock()

	return nil
}

// Destroy permanently destroys the session with the provided id in the
// in-memory session store.
func (ms *MemStore) Destroy(id string) error {
	ms.Lock()
	delete(ms.Data, id)
	ms.Unlock()

	return nil
}
