// Package sessionmw provides a Goji v2 context aware session middleware.
package sessionmw

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/knq/baseconv"

	"goji.io"

	"golang.org/x/net/context"
)

// context store constants
type contextKey int

// the various keys stored in context.Context
const (
	sessionContextKey    contextKey = 0
	sessionIDContextKey  contextKey = 1
	storeContextKey      contextKey = 2
	cookieNameContextKey contextKey = 3
)

const (
	// DefaultCookieName is the default cookie name.
	DefaultCookieName = "SESSID"
)

// IDFn is the ID generation func type.
type IDFn func() string

// session is the session storage.
type session struct {
	sync.RWMutex
	data map[string]interface{}
}

// ID retrieves the id for this session from the context.
func ID(ctxt context.Context) string {
	sessID := ctxt.Value(sessionIDContextKey).(string)
	return sessID
}

// Set stores a session value into the context.
//
// Session values will be saved to the underlying store after Handler has
// finished.
func Set(ctxt context.Context, key string, val interface{}) {
	sess := ctxt.Value(sessionContextKey).(session)
	sess.Lock()
	sess.data[key] = val
	sess.Unlock()
}

// Get retrieves a previously stored session value from the context.
func Get(ctxt context.Context, key string) (interface{}, bool) {
	sess := ctxt.Value(sessionContextKey).(session)
	sess.RLock()
	val, ok := sess.data[key]
	sess.RUnlock()
	return val, ok
}

// Delete deletes a stored session value from the context.
func Delete(ctxt context.Context, key string) {
	sess := ctxt.Value(sessionContextKey).(session)
	sess.Lock()
	delete(sess.data, key)
	sess.Unlock()
}

// GetStore retrieves the session store from the context.
func GetStore(ctxt context.Context) Store {
	st := ctxt.Value(storeContextKey).(Store)
	return st
}

// CookieName retrieves the cookie name from the context.
func CookieName(ctxt context.Context) string {
	return ctxt.Value(cookieNameContextKey).(string)
}

// Destroy destroys a session in the underlying session store.
//
// Note that any existing values will continue to remain in the context after
// destruction. The context should be closed (or destroyed).
//
// If the optional http.ResponseWriter is provided, then an expired cookie will
// be added to the response headers.
func Destroy(ctxt context.Context, res ...http.ResponseWriter) error {
	sessID := ID(ctxt)
	st := GetStore(ctxt)

	if len(res) > 0 {
		http.SetCookie(res[0], &http.Cookie{
			Name:    CookieName(ctxt),
			Expires: time.Now(),
			Value:   "-",
			MaxAge:  -1,
		})
	}

	return st.Destroy(sessID)
}

// Config contains the configuration parameters for the session middleware.
type Config struct {
	// Secret is
	Secret      []byte
	BlockSecret []byte

	// Store is the underlying session store.
	Store Store

	// IDFn is the id generation func.
	IDFn IDFn

	// Name is the cookie name.
	Name string

	// Path is the cookie path.
	Path string

	// Domain is the cookie domain.
	Domain string

	// Expires is the cookie expiration time.
	Expires time.Time

	// MaxAge is the cookie max age.
	MaxAge time.Duration

	// Secure is the cookie secure flag.
	Secure bool

	// HttpOnly is the cookie http only flag.
	HttpOnly bool
}

// Handler provides the goji.Handler for the session middleware.
func (c Config) Handler(h goji.Handler) goji.Handler {
	if c.Secret == "" {
		panic(errors.New("sessionmw config Secret cannot be empty string"))
	}

	if c.BlockSecret == "" {
		panic(errors.New("sessionmw config BlockSecret cannot be empty string"))
	}

	if c.Store == nil {
		panic(errors.New("sessionmw config Store was not provided"))
	}

	// create securecookie
	sc := securecookie.New([]byte(c.Secret), []byte(c.BlockSecret))
	sc.MaxAge(int(c.MaxAge))

	idFn := defaultIDGen
	if c.IDFn != nil {
		idFn = c.IDFn
	}

	name := c.Name
	if name == "" {
		name = DefaultCookieName
	}

	// load or create session
	return &sessMiddleware{
		h:  h,
		sc: sc,

		st:   c.Store,
		idFn: idFn,

		name:     name,
		path:     c.Path,
		domain:   c.Domain,
		expires:  c.Expires,
		maxAge:   c.MaxAge,
		secure:   c.Secure,
		httpOnly: c.HttpOnly,
	}
}

// sessMiddleware provides the actual session middleware.
type sessMiddleware struct {
	h  goji.Handler
	sc *securecookie.SecureCookie

	st   Store
	idFn IDFn

	name     string
	path     string
	domain   string
	expires  time.Time
	maxAge   time.Duration
	secure   bool
	httpOnly bool
}

// sessionID returns the session id from the http.Request if present.
func (s *sessMiddleware) sessionID(req *http.Request) (string, bool) {
	// grab cookie from request
	c, err := req.Cookie(s.name)
	if err != nil {
		return s.idFn(), false
	}

	// decode value
	v := make(map[string]string)
	err = s.sc.Decode(s.name, c.Value, &v)
	if err != nil {
		return s.idFn(), false
	}

	// retrieve id
	sessID, ok := v["id"]
	if !ok {
		return s.idFn(), false
	}

	return sessID, true
}

func (s *sessMiddleware) encodeCookie(id string) (string, error) {
	v := map[string]string{
		"id": id,
	}
	return s.sc.Encode(s.name, v)
}

// getSession retrieves the session from the http request, returning the
// session id and the session storage.
func (s *sessMiddleware) getSession(ctxt context.Context, res http.ResponseWriter, req *http.Request) (string, session, bool) {
	// grab id
	sessID, ok := s.sessionID(req)

	// if there was a problem retrieving the session id
	if !ok {
		return sessID, session{
			data: make(map[string]interface{}),
		}, true
	}

	// retrieve session from storage
	d, err := s.st.Get(sessID)
	if err != nil {
		return sessID, session{
			data: make(map[string]interface{}),
		}, true
	}

	// FIXME: do logic here for determining when to refresh
	var refresh = false
	return sessID, session{data: d}, refresh
}

// ServeHTTPC handles the actual session middleware logic.
func (s *sessMiddleware) ServeHTTPC(ctxt context.Context, res http.ResponseWriter, req *http.Request) {
	// retrieve session
	sessID, sess, refresh := s.getSession(ctxt, res, req)
	//log.Printf(">> session id: %s, refresh: %t", sessID, refresh)

	// refresh
	if refresh {
		// encode the cookie
		v, err := s.encodeCookie(sessID)
		if err != nil {
			http.Error(res, "internal server error", http.StatusInternalServerError)
			return
		}

		// set the cookie
		http.SetCookie(res, &http.Cookie{
			Name:     s.name,
			Path:     s.path,
			Domain:   s.domain,
			Expires:  s.expires,
			MaxAge:   int(s.maxAge),
			Secure:   s.secure,
			HttpOnly: s.httpOnly,
			Value:    v,
		})
	}

	// add context values
	ctxt = context.WithValue(ctxt, sessionIDContextKey, sessID)
	ctxt = context.WithValue(ctxt, storeContextKey, s.st)
	ctxt = context.WithValue(ctxt, sessionContextKey, sess)
	ctxt = context.WithValue(ctxt, cookieNameContextKey, s.name)

	// serve
	s.h.ServeHTTPC(ctxt, res, req)

	// save session
	s.st.Save(sessID, sess.data)
}

// defaultIDGen is the default session id generation func.
func defaultIDGen() string {
	n := uint64(time.Now().UnixNano())&0xffffffffffffffc0 | uint64(rand.Intn(1024))
	s, _ := baseconv.Encode62(fmt.Sprintf("%d", n))
	return s
}
