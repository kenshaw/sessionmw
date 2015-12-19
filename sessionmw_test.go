package sessionmw

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"goji.io/pat"
	"golang.org/x/net/context"

	"goji.io"

	"github.com/knq/baseconv"
)

const cookieName = "sessionmw_test"

var rexp = regexp.MustCompile(`(?i)` + cookieName + `=[^;\s]*`)

func TestDefaultIDGen(t *testing.T) {
	var err error
	var j uint64
	last := uint64(0)
	for i := 0; i < 100; i++ {
		d := defaultIDGen()
		s, _ := baseconv.Decode62(d)

		if j, err = strconv.ParseUint(s, 10, 64); err != nil {
			t.Fatalf("error encountered: %v", err)
		}
		if j <= last {
			t.Fatalf("ids should increment")
		}
		last = j
	}
}

func newMux() (*memStore, *goji.Mux) {
	ms := NewMemStore()

	// create session middleware
	conf := &Config{
		Secret:      "LymWKG0UvJFCiXLHdeYJTR1xaAcRvrf7",
		BlockSecret: "NxyECgzxiYdMhMbsBrUcAAbyBuqKDrpp",

		Store: ms,
		Name:  cookieName,
	}

	// create goji mux and add sessionmw
	mux := goji.NewMux()
	mux.UseC(conf.Handler)
	mux.HandleFuncC(pat.Get("/set/:name"), func(ctxt context.Context, res http.ResponseWriter, req *http.Request) {
		val := pat.Param(ctxt, "name")
		Set(ctxt, "name", val)
		http.Error(res, fmt.Sprintf("saved %s", html.EscapeString(val)), http.StatusOK)
	})
	mux.HandleFuncC(pat.Get("/del"), func(ctxt context.Context, res http.ResponseWriter, req *http.Request) {
		Delete(ctxt, "name")
		http.Error(res, "deleted", http.StatusOK)
	})
	mux.HandleFuncC(pat.Get("/id"), func(ctxt context.Context, res http.ResponseWriter, req *http.Request) {
		http.Error(res, ID(ctxt), http.StatusOK)
	})
	mux.HandleFuncC(pat.Get("/destroy"), func(ctxt context.Context, res http.ResponseWriter, req *http.Request) {
		Destroy(ctxt, res)
		http.Error(res, "destroyed", http.StatusOK)
	})
	mux.HandleFuncC(pat.Get("/"), func(ctxt context.Context, res http.ResponseWriter, req *http.Request) {
		var name = "[no name]"
		val, _ := Get(ctxt, "name")
		if n, ok := val.(string); ok {
			name = n
		}
		http.Error(res, html.EscapeString(name), http.StatusOK)
	})

	return ms, mux
}

func TestHandler(t *testing.T) {
	ms, mux := newMux()

	r0, _ := get(mux, "/", nil, t)
	check(200, r0, t)
	cookie := getCookie(r0, t)
	if "[no name]" != strings.TrimSpace(r0.Body.String()) {
		t.Fatalf("expected [no name], got: '%s'", r0.Body.String())
	}
	if len(ms.data) != 1 {
		t.Fatalf("ms.data should be length 1")
	}

	r1, _ := get(mux, "/id", cookie, t)
	check(200, r1, t)
	sessID := strings.TrimSpace(r1.Body.String())
	log.Printf(">>> %+v", ms.data[sessID])
	sess, ok := ms.data[sessID].(map[string]interface{})
	if !ok {
		t.Fatalf("ms.data should contain %s of type map[string]interface{}", sessID)
	}

	r2, _ := get(mux, "/set/foo", cookie, t)
	check(200, r2, t)
	if "saved foo" != strings.TrimSpace(r2.Body.String()) {
		t.Fatalf("expected saved foo")
	}
	if len(ms.data) != 1 {
		t.Fatalf("ms.data should be length 1")
	}
	if n, ok := sess["name"]; !ok || "foo" != n {
		t.Fatalf("sess[name] should be foo")
	}

	r3, _ := get(mux, "/del", cookie, t)
	check(200, r3, t)
	if "deleted" != strings.TrimSpace(r3.Body.String()) {
		t.Fatalf("expected deleted")
	}
	if len(ms.data) != 1 {
		t.Fatalf("ms.data should be length 1")
	}
	if _, ok := sess["name"]; ok {
		t.Fatalf("sess[name] should not be defined")
	}

	r4, _ := get(mux, "/", cookie, t)
	check(200, r4, t)
	if "[no name]" != strings.TrimSpace(r4.Body.String()) {
		t.Fatalf("expected [no name], got: '%s'", r4.Body.String())
	}
	if len(ms.data) != 1 {
		t.Fatalf("ms.data should be length 1")
	}

	r5, _ := get(mux, "/destroy", cookie, t)
	check(200, r5, t)
	if "destroyed" != strings.TrimSpace(r5.Body.String()) {
		t.Fatalf("expected destroyed, got: '%s'", r5.Body.String())
	}
	newCookie := getCookie(r5, t)
	if newCookie.Value != "-" {
		t.Fatalf("new cookie value should be -")
	}
}

func get(mux *goji.Mux, path string, cookie *http.Cookie, t *testing.T) (*httptest.ResponseRecorder, string) {
	rr := httptest.NewRecorder()
	q, _ := http.NewRequest("GET", path, nil)
	if cookie != nil {
		q.AddCookie(cookie)
	}
	mux.ServeHTTP(rr, q)

	l := ""

	switch {
	case rr.Code >= 300 && rr.Code < 400:
		if len(rr.HeaderMap["Location"]) != 1 {
			t.Errorf("code %d redirect had 0 or more than 1 location header (count: %d)", rr.Code, len(rr.HeaderMap["Location"]))
		} else {
			l = rr.HeaderMap["Location"][0]
			if len(l) < 1 {
				t.Error("redirect location should not be empty string")
			}
		}
	}

	return rr, l
}

func getCookie(rr *httptest.ResponseRecorder, t *testing.T) *http.Cookie {
	cookieStr := rexp.FindString(rr.HeaderMap["Set-Cookie"][0])
	cookie := &http.Cookie{
		Name:  cookieName,
		Value: cookieStr[strings.Index(cookieStr, "=")+1:],
	}

	// sanity check
	if len(cookie.Value) < 1 {
		t.Errorf("cookie should not be empty")
	}

	return cookie
}

func check(code int, rr *httptest.ResponseRecorder, t *testing.T) {
	if code != rr.Code {
		t.Logf("GOT: %d -- %s", rr.Code, rr.Body.String())
		t.Errorf("expected %d, got: %d", code, rr.Code)
	}
}
