package redisstore

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/fzzy/radix/redis"
	"github.com/knq/sessionmw"
	rsrvr "github.com/operetta/go-redis-server"
)

const (
	pause = 50 * time.Millisecond
)

func TestStore(t *testing.T) {
	// create test server
	addr, listener, server, handler := createRedisServer(t)
	go server.Serve(listener)
	defer listener.Close()
	time.Sleep(pause)

	type testData map[string]interface{}

	// create redisstore
	ss, err := New("redis://" + addr.String())
	if err != nil {
		t.Fatalf("could not connect to redis server: %v", err)
	}

	// sanity check
	_, err = ss.Get("notpresent")
	if err != sessionmw.ErrSessionNotFound {
		t.Fatalf("expected sessionmw.ErrSessionNotFound, got: %v", err)
	}

	// store some data
	wg0 := sync.WaitGroup{}
	wg0.Add(10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer wg0.Done()
			err := ss.Save(
				fmt.Sprintf("id-%d", id),
				testData{
					"id": id,
				},
			)
			if err != nil {
				t.Errorf("should not encounter error, got: %v", err)
				return
			}
		}(i)
	}
	wg0.Wait()

	if len(handler.values) != 10 {
		t.Errorf("handler.values should have length 10, len: %d, %+v", len(handler.values), handler.values)
	}

	// retrieve some data
	wg1 := sync.WaitGroup{}
	wg1.Add(10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer wg1.Done()

			sess, err := ss.Get(fmt.Sprintf("id-%d", id))
			if err != nil {
				t.Errorf("should not encounter error, got: %v", err)
				return
			}

			if id, ok := sess["id"].(int); !ok {
				t.Errorf("expected: %d, got: %v", id, sess["id"])
				return
			}
		}(i)
	}
	wg1.Wait()

	// destroy some data
	wg2 := sync.WaitGroup{}
	wg2.Add(10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer wg2.Done()
			err := ss.Destroy(fmt.Sprintf("id-%d", id))
			if err != nil {
				t.Errorf("should not encounter error")
				return
			}
		}(i)
	}
	wg2.Wait()

	if len(handler.values) != 0 {
		t.Errorf("handler.values should have length 0, len: %d, %+v", len(handler.values), handler.values)
	}

	// retrieve again and make sure data is not present
	wg3 := sync.WaitGroup{}
	wg3.Add(10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer wg3.Done()
			_, err := ss.Get(fmt.Sprintf("id-%d", id))
			if err != sessionmw.ErrSessionNotFound {
				t.Errorf("should get error ErrSessionNotFound, got: %v", err)
				return
			}
		}(i)
	}
	wg3.Wait()
}

func TestNew(t *testing.T) {
	_, err := New(":,/")
	if _, ok := err.(*url.Error); !ok {
		t.Errorf("expected url.Error, got: %v", err)
	}

	_, err = New("http://google.com/")
	if err != ErrInvalidScheme {
		t.Errorf("expected ErrInvalidScheme")
	}

	_, err = New("redis://localhost:0")
	if _, ok := err.(*net.OpError); !ok {
		t.Errorf("expected net.OpError, got: %v", err)
	}

	a0, l0, s0, _ := createRedisServer(t)
	go s0.Serve(l0)
	defer l0.Close()
	time.Sleep(pause)

	rs, err := New("redis://"+a0.String(), "myprefix_")
	if err != nil {
		t.Fatalf("should not encounter error, got: %v", err)
	}
	if rs.KeyPrefix != "myprefix_" {
		t.Errorf("rs.KeyPrefix should be myprefix_")
	}
}

func TestError(t *testing.T) {
	e := &Error{"op", "cmd", errors.New("sub")}
	if "op cmd: sub" != e.Error() {
		t.Errorf("invalid error format observed, %v", e)
	}
}

func TestClientPoolErrors(t *testing.T) {
	type testData map[string]interface{}

	for i := 0; i < 3; i++ {
		// create client and connect RedisStore
		a0, l0, s0, h0 := createRedisServer(t)
		go s0.Serve(l0)
		time.Sleep(pause)

		rs0, err := New("redis://" + a0.String())
		if err != nil {
			t.Fatalf("got error: %v", err)
		}

		// sanity check
		_, err = rs0.Get("nonpresent")
		if err != sessionmw.ErrSessionNotFound {
			t.Fatalf("expected sessionmw.ErrSessionNotFound, got: %v", err)
		}

		// store something
		err = rs0.Save("something", testData{"avalue": ""})
		if err != nil {
			t.Fatalf("should not encounter error, got: %v", err)
		}

		// check that the store worked
		if len(h0.values) != 1 {
			t.Fatalf("h0.values should have length 1, len: %d, %+v", len(h0.values), h0.values)
		}
		if _, ok := h0.values[DefaultKeyPrefix+"something"]; !ok {
			t.Fatalf("something should be stored in h0.values")
		}

		// close connection
		l0.Close()
		time.Sleep(pause)

		switch i {
		case 0:
			err = rs0.Save("secondthing", testData{"avalue": ""})
		case 1:
			_, err = rs0.Get("something")
		case 2:
			err = rs0.Destroy("something")
		}

		// make sure we got an error
		if _, ok := err.(*net.OpError); !ok {
			t.Fatalf("expected net.OpError, got: %s", reflect.TypeOf(err))
		}

		// check the h0.values length is still the same
		if len(h0.values) != 1 {
			t.Fatalf("h0.values should have length 1, len: %d, %+v", len(h0.values), h0.values)
		}
		if _, ok := h0.values[DefaultKeyPrefix+"something"]; !ok {
			t.Fatalf("something should be stored in h0.values, loop: %d", i)
		}
	}
}

func TestRedisServerErrors(t *testing.T) {
	type testData map[string]interface{}

	for i := 0; i < 3; i++ {
		// create client and connect RedisStore
		a0, l0, s0, h0 := createRedisServer(t)
		go s0.Serve(l0)
		time.Sleep(pause)

		rs0, err := New("redis://" + a0.String())
		if err != nil {
			t.Fatalf("got error: %v", err)
		}

		// sanity check
		_, err = rs0.Get("nonpresent")
		if err != sessionmw.ErrSessionNotFound {
			t.Fatalf("expected sessionmw.ErrSessionNotFound, got: %v", err)
		}

		// store something
		err = rs0.Save("something", testData{"avalue": ""})
		if err != nil {
			t.Fatalf("should not encounter error, got: %v", err)
		}

		// check that the store worked
		if len(h0.values) != 1 {
			t.Fatalf("h0.values should have length 1, len: %d, %+v", len(h0.values), h0.values)
		}
		if _, ok := h0.values[DefaultKeyPrefix+"something"]; !ok {
			t.Fatalf("something should be stored in h0.values")
		}

		// force errors
		h0.forceErr = true

		switch i {
		case 0:
			err = rs0.Save("secondthing", testData{"avalue": ""})
		case 1:
			_, err = rs0.Get("something")
		case 2:
			err = rs0.Destroy("something")
		}

		// make sure we got an error
		if _, ok := err.(*Error); !ok {
			t.Fatalf("expected Error, got: %s", reflect.TypeOf(err))
		}

		err = err.(*Error).Err

		// verify the error was forced
		if _, ok := err.(*redis.CmdError); !ok {
			t.Fatalf("expected redis.CmdError, got: %s", reflect.TypeOf(err))
		}

		err = err.(*redis.CmdError).Err
		if "ERROR forced" != err.Error() {
			t.Fatalf("expected forced error, got: %v", err)
		}

		// check the h0.values length is still the same
		if len(h0.values) != 1 {
			t.Fatalf("h0.values should have length 1, len: %d, %+v", len(h0.values), h0.values)
		}
		if _, ok := h0.values[DefaultKeyPrefix+"something"]; !ok {
			t.Fatalf("something should be stored in h0.values, loop: %d", i)
		}
	}
}

type redisHandler struct {
	rw       sync.RWMutex
	t        *testing.T
	values   map[string][]byte
	forceErr bool
}

func (r *redisHandler) GET(key string) ([]byte, error) {
	//r.t.Logf("HANDLER GET %s", key)
	if r.forceErr {
		return nil, errors.New("forced")
	}

	r.rw.RLock()
	v := r.values[key]
	r.rw.RUnlock()
	return v, nil
}

func (r *redisHandler) SET(key string, value []byte) error {
	//r.t.Logf("HANDLER SET %s", key)
	if r.forceErr {
		return errors.New("forced")
	}

	r.rw.Lock()
	r.values[key] = value
	r.rw.Unlock()
	return nil
}

func (r *redisHandler) DEL(key string, keys ...string) (int, error) {
	//r.t.Logf("HANDLER DEL %s", key)
	if r.forceErr {
		return 0, errors.New("forced")
	}

	r.rw.Lock()
	delete(r.values, key)
	r.rw.Unlock()
	return 1, nil
}

func createListener(t *testing.T) (*net.TCPAddr, net.Listener) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("problem creating socket for temporary redis server, %v", err)
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatalf("could not open listener %s: %v", addr, err)
	}

	return addr, listener
}

func createRedisServer(t *testing.T) (*net.TCPAddr, net.Listener, *rsrvr.Server, *redisHandler) {
	addr, listener := createListener(t)
	addr.Port = listener.Addr().(*net.TCPAddr).Port
	conf := (&rsrvr.Config{}).
		Host("127.0.0.1").
		Proto("tcp").
		Port(addr.Port)

	values := make(map[string][]byte)

	handler := &redisHandler{
		t:      t,
		values: values,
	}

	server, err := rsrvr.NewServer(conf.Handler(handler))

	if err != nil {
		t.Fatalf("encountered error creating redis server: %v", err)
	}

	return addr, listener, server, handler
}
