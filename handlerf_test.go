// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"testing"
)

// Handler names end with three Y/N flags:
// 1. does the handler accept a *Context arg
// 2. does the handler return a string as its first ret val
// 3. does the handler return an error as its last ret val

func handleNNN() {
	return
}

func handleNNY() error {
	return nil
}

func handleNYN() string {
	return "NYN"
}

func handleNYY() (string, error) {
	return "NYY", nil
}

func handleYNN(ctx *Context) {
	fmt.Fprint(ctx, "YNN")
}

func handleYNY(ctx *Context) error {
	fmt.Fprint(ctx, "YNY")
	return nil
}

func handleYYN(ctx *Context) string {
	fmt.Fprint(ctx, "YY")
	return "N"
}

func handleYYY(ctx *Context) (string, error) {
	fmt.Fprint(ctx, "YY")
	return "Y", nil
}

var handlerfTests = []Test{
	{
		method:         "GET",
		path:           "/NNN",
		expectedStatus: 200,
		expectedBody:   "",
	},
	{
		method:         "GET",
		path:           "/NNY",
		expectedStatus: 200,
		expectedBody:   "",
	},
	{
		method:         "GET",
		path:           "/NYN",
		expectedStatus: 200,
		expectedBody:   "NYN",
	},
	{
		method:         "GET",
		path:           "/NYY",
		expectedStatus: 200,
		expectedBody:   "NYY",
	},
	{
		method:         "GET",
		path:           "/YNN",
		expectedStatus: 200,
		expectedBody:   "YNN",
	},
	{
		method:         "GET",
		path:           "/YNY",
		expectedStatus: 200,
		expectedBody:   "YNY",
	},
	{
		method:         "GET",
		path:           "/YYN",
		expectedStatus: 200,
		expectedBody:   "YYN",
	},
	{
		method:         "GET",
		path:           "/YYY",
		expectedStatus: 200,
		expectedBody:   "YYY",
	},
}

func handlerfTestServer() *Server {
	s := NewServer()
	s.SetLogger(nopLogger)
	s.Get("/NNN", handleNNN)
	s.Get("/NNY", handleNNY)
	s.Get("/NYN", handleNYN)
	s.Get("/NYY", handleNYY)
	s.Get("/YNN", handleYNN)
	s.Get("/YNY", handleYNY)
	s.Get("/YYN", handleYYN)
	s.Get("/YYY", handleYYY)
	return s
}

func TestHandlerSig(t *testing.T) {
	s := handlerfTestServer()
	for _, test := range handlerfTests {
		testFull(t, s, test)
	}
}
