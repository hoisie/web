// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"log"
	"net/http"
)

// Log one request by calling every method in the order defined below. Logging
// may be done in a separate goroutine from handling. Arguments are passed by
// reference for efficiency but MUST NOT be changed!
type OneAccessLogger interface {
	// Called with the raw incoming request
	LogRequest(*http.Request)
	// Parameters as parsed by web.go
	LogParams(Params)
	// Called when headers are set by handler and will be written to client
	LogHeader(status int, header http.Header)
	// Called when response has been written to client. If an error occurred at
	// any point during handling it is passed as an argument. Otherwise err is
	// nil.
	LogDone(err error)
}

type plainOneAccessLogger struct{ *log.Logger }

func (l plainOneAccessLogger) LogRequest(req *http.Request) {
	l.Printf("%s %s", req.Method, req.URL.Path)
}

func (l plainOneAccessLogger) LogParams(p Params) {
	l.Printf("Params: %v\n", p)
}

func (l plainOneAccessLogger) LogHeader(status int, h http.Header) {
}

func (l plainOneAccessLogger) LogDone(err error) {
}

type coloredOneAccessLogger struct{ *log.Logger }

func (l coloredOneAccessLogger) LogRequest(req *http.Request) {
	l.Printf("\033[32;1m%s %s\033[0m", req.Method, req.URL.Path)
}

func (l coloredOneAccessLogger) LogParams(p Params) {
	l.Printf("\033[37;1mParams: %v\033[0m\n", p)
}

func (l coloredOneAccessLogger) LogHeader(status int, h http.Header) {
}

func (l coloredOneAccessLogger) LogDone(err error) {
}

// Factory function that generates new one-shot access loggers
type AccessLogger func(*Server) OneAccessLogger

// Simple stateless access logger that prints all requests to server.Logger
func DefaultAccessLogger(s *Server) OneAccessLogger {
	if s.Config.ColorOutput {
		return coloredOneAccessLogger{s.Logger}
	} else {
		return plainOneAccessLogger{s.Logger}
	}
}
