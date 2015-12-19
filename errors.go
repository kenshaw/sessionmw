package sessionmw

import "errors"

// ErrSessionNotFound is the error returned by sessionmw.Store providers when a
// session cannot be found.
var ErrSessionNotFound = errors.New("session not found")
