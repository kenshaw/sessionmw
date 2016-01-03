package sessionmw

// Store is the common interface for session storage.
//
// Please see github.com/knq/kv.Store for a compatible store.
type Store interface {
	// Write retrieves the session for the provided id.
	Write(key string, obj interface{}) error

	// Read saves the session for the provided id.
	//
	// If the provided id already exists in storage, then it will be
	// overwritten.
	Read(key string) (interface{}, error)

	// Destroy permanently destroys the session with the provided id.
	Erase(key string) error
}
