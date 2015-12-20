package redisstore

import (
	"bytes"
	"encoding/gob"
	"errors"
	"net/url"

	"github.com/fzzy/radix/extra/pool"
	"github.com/fzzy/radix/redis"
	"github.com/knq/sessionmw"
)

const (
	// DefaultKeyPrefix is the default key prefix'd to ids in redis.
	DefaultKeyPrefix = "SESS_"
)

// RedisStore provides a sessionmw.Store compatiable redis-backed session
// store.
type RedisStore struct {
	// KeyPrefix will be used as the prefix used with the supplied ids during
	// Get/Save/Destroy.
	KeyPrefix string

	// Pool is the redis connection pool.
	Pool *pool.Pool
}

// ErrInvalidScheme is the error thrown when the scheme in the supplied URL to
// New is invalid (ie, URL doesn't start with redis://).
var ErrInvalidScheme = errors.New("invalid scheme")

// Error reports an error and the operation and redis command that caused it.
type Error struct {
	// Op is the operation.
	Op string

	// Cmd is the redis command that was being called during the operation.
	Cmd string

	// Err is the underlying error, if any.
	Err error
}

// Error returns an informative string for the error.
func (e *Error) Error() string {
	return e.Op + " " + e.Cmd + ": " + e.Err.Error()
}

// New creates a RedisStore, and creates a redis connection pool for the
// supplied redis url. Written/read data in the store use the DefaultKeyPrefix
// or the provided key prefix for key retrieval/storage.
func New(redisURL string, keyPrefix ...string) (*RedisStore, error) {
	// determine key prefix
	keyPrefixVal := ""
	if len(keyPrefix) > 0 {
		keyPrefixVal = keyPrefix[0]
	} else {
		keyPrefixVal = DefaultKeyPrefix
	}

	// parse url
	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, err
	}

	// ensure that its a redis:// url
	if u.Scheme != "redis" {
		return nil, ErrInvalidScheme
	}

	// create pool
	p, err := pool.NewPool("tcp", u.Host, 1)
	if err != nil {
		return nil, err
	}

	return &RedisStore{
		KeyPrefix: keyPrefixVal,
		Pool:      p,
	}, nil
}

// Get retrieves the session for the provided id from the redis session store.
func (rs *RedisStore) Get(id string) (map[string]interface{}, error) {
	// get client
	client, err := rs.Pool.Get()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// execute get
	res := client.Cmd("GET", rs.KeyPrefix+id)
	if res.Err != nil {
		return nil, &Error{"write", "GET", res.Err}
	}

	// if id is not present
	if res.Type == redis.NilReply {
		return nil, sessionmw.ErrSessionNotFound
	}

	// read response
	buf, err := res.Bytes()
	if err != nil {
		return nil, &Error{"read", "GET", err}
	}

	// wrap buf in an io.Reader
	r := bytes.NewReader(buf)

	// attempt decode
	var sess map[string]interface{}
	err = gob.NewDecoder(r).Decode(&sess)
	if err != nil {
		return nil, &Error{"decode", "GET", err}
	}

	return sess, nil
}

// Save saves the session for the provided id from the redis session store.
//
// If the provided id already exists in redis, then it will be overwritten.
func (rs *RedisStore) Save(id string, sess map[string]interface{}) error {
	// get client
	client, err := rs.Pool.Get()
	if err != nil {
		return err
	}
	defer client.Close()

	// encode data
	var buf bytes.Buffer
	err = gob.NewEncoder(&buf).Encode(sess)
	if err != nil {
		return &Error{"encode", "SET", err}
	}

	// execute set
	err = client.Cmd("SET", rs.KeyPrefix+id, buf.Bytes()).Err
	if err != nil {
		err = &Error{"write", "SET", err}
	}
	return err
}

// Destroy permanently destroys the session with the provided id from the redis
// session store.
func (rs *RedisStore) Destroy(id string) error {
	// get client
	client, err := rs.Pool.Get()
	if err != nil {
		return err
	}
	defer client.Close()

	// execute del
	err = client.Cmd("DEL", rs.KeyPrefix+id).Err
	if err != nil {
		err = &Error{"write", "DEL", err}
	}
	return err
}
