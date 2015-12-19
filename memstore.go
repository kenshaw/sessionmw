package sessionmw

import "sync"

type memStore struct {
	sync.RWMutex
	data map[string]interface{}
}

// NewMemStore creates a sessionmw.Store compatiable in-memory session store.
func NewMemStore() *memStore {
	return &memStore{
		data: make(map[string]interface{}),
	}
}

// Get retrieves the session for the provided id from the in-memory session
// store.
func (ms *memStore) Get(id string) (map[string]interface{}, error) {
	ms.RLock()
	defer ms.RUnlock()

	if sess, ok := ms.data[id]; ok {
		return sess.(map[string]interface{}), nil
	}

	return nil, ErrSessionNotFound
}

// Save saves the session for the provided id from the in-memory session store.
//
// If the provided id already exists in storage, then it will be overwritten.
func (ms *memStore) Save(id string, sess map[string]interface{}) error {
	ms.Lock()
	ms.data[id] = sess
	ms.Unlock()

	return nil
}

// Destroy permanently destroys the session with the provided id in the
// in-memory session store.
func (ms *memStore) Destroy(id string) error {
	ms.Lock()
	delete(ms.data, id)
	ms.Unlock()

	return nil
}
