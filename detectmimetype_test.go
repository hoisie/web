// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"
	"testing"
)

func handleCss() string {
	return `* { color: red; }`
}

func handleAnyExt(ctx *Context, ext string) string {
	switch ext {
	case "txt":
		return "this is text"
	case "html":
		return "<strong>this is html"
	case "xml":
		return "<outie><innie>you liek XML?</innie></outie>"
	}
	ctx.NotFound("unknown extension")
	return ""
}

func guessMimeTestServer() *Server {
	s := NewServer()
	s.SetLogger(nopLogger)
	s.AddWrapper(GuessMimetypeWrapper)
	s.Get(`/red\.css`, handleCss)
	s.Get(`/anything\.(.+)`, handleAnyExt)
	return s
}

func TestGuessMime_Explicit(t *testing.T) {
	header := http.Header{}
	header.Set("content-type", "text/css; charset=utf-8")
	testFull(t, guessMimeTestServer(), Test{
		method:         "GET",
		path:           "/red.css",
		expectedStatus: 200,
		expectedBody:   "* { color: red; }",
		headers:        header,
	})
}

func TestGuessMime_Free(t *testing.T) {
	header := http.Header{}
	header.Set("content-type", "text/html; charset=utf-8")
	testFull(t, guessMimeTestServer(), Test{
		method:         "GET",
		path:           "/anything.html",
		expectedStatus: 200,
		expectedBody:   "<strong>this is html",
		headers:        header,
	})
	header.Set("content-type", "text/plain; charset=utf-8")
	testFull(t, guessMimeTestServer(), Test{
		method:         "GET",
		path:           "/anything.txt",
		expectedStatus: 200,
		expectedBody:   "this is text",
		headers:        header,
	})
}

func TestGuessMime_Fail(t *testing.T) {
	header := http.Header{}
	// The mime type should not be set on failing resources
	header.Set("content-type", "text/plain; charset=utf-8")
	testFull(t, guessMimeTestServer(), Test{
		method:         "GET",
		path:           "/anything.js",
		expectedStatus: 404,
		expectedBody:   "unknown extension",
		headers:        header,
	})
}
