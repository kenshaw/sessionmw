// example/simple/simple.go
package main

import (
	"fmt"
	"html"
	"net/http"

	"goji.io"
	"goji.io/pat"
	"golang.org/x/net/context"

	"github.com/knq/kv"
	"github.com/knq/sessionmw"
)

func main() {
	// create session middleware
	conf := &sessionmw.Config{
		Secret:      []byte("LymWKG0UvJFCiXLHdeYJTR1xaAcRvrf7"),
		BlockSecret: []byte("NxyECgzxiYdMhMbsBrUcAAbyBuqKDrpp"),
		Store:       kv.NewMemStore(),
	}

	// create goji mux and add sessionmw
	mux := goji.NewMux()
	mux.UseC(conf.Handler)

	// add handlers
	mux.HandleFuncC(pat.Get("/set/:name"), func(ctxt context.Context, res http.ResponseWriter, req *http.Request) {
		val := pat.Param(ctxt, "name")
		sessionmw.Set(ctxt, "name", val)
		http.Error(res, fmt.Sprintf("name saved as '%s'.", html.EscapeString(val)), http.StatusOK)
	})
	mux.HandleFuncC(pat.Get("/"), func(ctxt context.Context, res http.ResponseWriter, req *http.Request) {
		var name = "[no name]"
		val, _ := sessionmw.Get(ctxt, "name")
		if n, ok := val.(string); ok {
			name = n
		}
		http.Error(res, fmt.Sprintf("hello '%s'", html.EscapeString(name)), http.StatusOK)
	})

	// serve
	http.ListenAndServe(":3000", mux)
}
