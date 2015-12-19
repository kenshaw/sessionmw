package sessionmw

// Store is the common interface for session storage.
//
// A session
type Store interface {
	// Get retrieves the session for the provided id.
	Get(id string) (map[string]interface{}, error)

	// Save saves the session for the provided id.
	//
	// If the provided id already exists in storage, then it will be
	// overwritten.
	Save(id string, sess map[string]interface{}) error

	// Destroy permanently destroys the session with the provided id.
	Destroy(id string) error
}
