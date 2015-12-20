# About sessionmw [![Build Status](https://travis-ci.org/knq/sessionmw.svg)](https://travis-ci.org/knq/sessionmw) [![Coverage Status](https://coveralls.io/repos/knq/sessionmw/badge.svg?branch=master&service=github)](https://coveralls.io/github/knq/sessionmw?branch=master) #

A [Goji v2](https://goji.io/) middleware package providing session management
and storage via [context.Context](https://godoc.org/golang.org/x/net/context).

## Installation ##

Install the package via the following:

    go get -u github.com/knq/sessionmw

## Usage ##

Please see [the GoDoc API page](http://godoc.org/github.com/knq/sessionmw) for
a full API listing. Currently there is a simple in-memory store for sessions,
as well as a simple redis store. Please see the [examples](./examples)
directory.

The sessionmw package can be used similarly to the following:

```go
// example/simple/simple.go
package main

import (
    "fmt"
    "html"
    "net/http"

    "github.com/knq/sessionmw"
    "goji.io"
    "goji.io/pat"
    "golang.org/x/net/context"
)

func main() {
    // create session middleware
    sessConfig := &sessionmw.Config{
        Secret:      []byte("LymWKG0UvJFCiXLHdeYJTR1xaAcRvrf7"),
        BlockSecret: []byte("NxyECgzxiYdMhMbsBrUcAAbyBuqKDrpp"),

        Store: sessionmw.NewMemStore(),
    }

    // create goji mux and add sessionmw
    mux := goji.NewMux()
    mux.UseC(sessConfig.Handler)

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
```

## TODO ##

* Finish writing unit tests.
* Finish json store and example.
* Finish [groupcache](https://github.com/golang/groupcache) store and example.
* Finish simple database store and example.
