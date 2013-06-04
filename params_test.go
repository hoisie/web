// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"testing"
)

func handleI(ctx *Context) {
	fmt.Fprint(ctx, ctx.Params.GetInt("i"))
}

func handleS(ctx *Context) string {
	return ctx.Params.GetString("s")
}

func paramsTestServer() *Server {
	s := NewServer()
	s.SetLogger(nopLogger)
	s.Get("/i", handleI)
	s.Get("/s", handleS)
	return s
}

func TestParams_GetInt(t *testing.T) {
	testFull(t, paramsTestServer(), Test{
		method:         "GET",
		path:           "/i?i=40",
		expectedStatus: 200,
		expectedBody:   "40",
	})
}

func TestParams_GetInt_Illegal(t *testing.T) {
	testFull(t, paramsTestServer(), Test{
		method:         "GET",
		path:           "/i?i=asdf",
		expectedStatus: 400,
		expectedBody:   "Illegal integer parameter i",
	})
}

func TestParams_GetString(t *testing.T) {
	testFull(t, paramsTestServer(), Test{
		method:         "GET",
		path:           "/s?s=asdf",
		expectedStatus: 200,
		expectedBody:   "asdf",
	})
}

func TestParams_GetString_Missing(t *testing.T) {
	testFull(t, paramsTestServer(), Test{
		method:         "GET",
		path:           "/s",
		expectedStatus: 400,
		expectedBody:   "Required parameter s missing",
	})
}
