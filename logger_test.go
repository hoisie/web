// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
)

func handleFoo(ctx *Context) (string, error) {
	return "foo", WebError{123, "error!"}
}

// Expected log line
const expectedLog = "GET /foo (a=b) 123 (error!)\n"

type testlogger struct{ *bytes.Buffer }

func (l testlogger) LogRequest(req *http.Request) {
	fmt.Fprint(l, req.Method, " ", req.URL.Path)
}

func (l testlogger) LogParams(p Params) {
	l.WriteRune(' ')
	// If there is exactly one parameter log it
	if len(p) == 1 {
		l.WriteRune('(')
		for k, v := range p {
			fmt.Fprint(l, k, "=", v)
		}
		l.WriteRune(')')
	}
}

func (l testlogger) LogHeader(status int, h http.Header) {
	fmt.Fprint(l, " ", status)
}

func (l testlogger) LogDone(err error) {
	if err != nil {
		fmt.Fprintf(l, " (%v)", err)
	}
	fmt.Fprintln(l)
}

func TestLogger(t *testing.T) {
	s := NewServer()
	s.SetLogger(nopLogger)
	var buf bytes.Buffer
	s.AccessLogger = func(s *Server) OneAccessLogger {
		return testlogger{&buf}
	}
	s.Get("/foo", handleFoo)
	testRouting(t, s, Test{
		method:         "GET",
		path:           "/foo?a=b",
		expectedStatus: 123,
		expectedBody:   "error!",
	})
	// also inspect the log
	logstr := buf.String()
	if logstr != expectedLog {
		t.Errorf("Expected log: %q, got: %q", expectedLog, logstr)
	}
}
