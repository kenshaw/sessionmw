package sessionmw

import (
	"fmt"
	"sync"
	"testing"
)

func TestMemStore(t *testing.T) {
	type testData map[string]interface{}

	// create memstore
	ss := NewMemStore()

	// sanity check
	_, err := ss.Get("notpresent")
	if err != ErrSessionNotFound {
		t.Fatalf("expected error, got: %v", err)
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

	if len(ss.data) != 10 {
		t.Errorf("ss.data should have length 10, len: %d, %+v", len(ss.data), ss.data)
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

	if len(ss.data) != 0 {
		t.Errorf("ss.data should have length 0, len: %d, %+v", len(ss.data), ss.data)
	}

	// retrieve again and make sure data is not present
	wg3 := sync.WaitGroup{}
	wg3.Add(10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer wg3.Done()
			_, err := ss.Get(fmt.Sprintf("id-%d", id))
			if err != ErrSessionNotFound {
				t.Errorf("should get error ErrSessionNotFound, got: %v", err)
				return
			}
		}(i)
	}
	wg3.Wait()
}
