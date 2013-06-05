// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"sync"
)

// wrap http.ResponseWriter to allow function hooks that are executed after the
// response headers are set and after the body is sent.
type ResponseWriter struct {
	// callbacks to execute sequentially with reference to this object after
	// all headers have been set
	afterHeaders []func(*ResponseWriter)
	// closed when entire response is has been written to client (succesfully).
	// if multiple closers are set they are closed in reverse order because
	// closing an outer writer can flush pending data to an underlying writer.
	// would be more consistent if it had the same type as afterHeaders but
	// this is more explicit although technically the Close method can do
	// whatever it wants
	closers []io.Closer
	// lock to call the afterheaders functions exactly once before writing body
	once sync.Once
	// Underlying response writer, only use this for the headers
	http.ResponseWriter
	status int
	// body data is written here. can be wrapped by afterheaders functions
	BodyWriter io.Writer
}

func (w *ResponseWriter) triggerAfterHeaders() {
	w.once.Do(func() {
		for _, f := range w.afterHeaders {
			f(w)
		}
	})
}

func (w *ResponseWriter) Write(data []byte) (int, error) {
	w.triggerAfterHeaders()
	return w.BodyWriter.Write(data)
}

func (w *ResponseWriter) WriteHeader(status int) {
	w.status = status
	w.triggerAfterHeaders()
	w.ResponseWriter.WriteHeader(status)
}

func (w *ResponseWriter) Close() error {
	var err error
	for i := range w.closers {
		c := w.closers[len(w.closers)-i-1]
		err2 := c.Close()
		if err == nil && err2 != nil {
			err = err2
		}
	}
	return err
}

func (w *ResponseWriter) WrapBodyWriter(f func(w io.Writer) io.Writer) {
	w.BodyWriter = f(w.BodyWriter)
	if c, ok := w.BodyWriter.(io.Closer); ok {
		w.closers = append(w.closers, c)
	}
}

func (w *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

// Add callback to execute when all headers have been set and body data is
// about to be written
func (w *ResponseWriter) AddAfterHeaderFunc(f func(*ResponseWriter)) {
	w.afterHeaders = append(w.afterHeaders, f)
}

// Return true if the status code indicates succesful handling: 1xx, 2xx or
// 3xx.
func httpSuccess(status int) bool {
	return status >= 100 && status <= 399
}

// True if the writer has sent a status code to the client indicating success
func (w *ResponseWriter) Success() bool {
	return httpSuccess(w.status)
}
