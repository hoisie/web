// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// any mime type shares a prefix with a string in this array will be compressed
var compressableTypePrefixes = [...]string{
	"text/",
	"application/json",
	"application/xml",
	"application/javascript",
}

func compressable(ctype string) bool {
	for _, t := range compressableTypePrefixes {
		if strings.HasPrefix(ctype, t) {
			return true
		}
	}
	return false
}

// conditionally compress the HTTP response. this function must be executed
// after all response headers have been set by the underlying handler (because
// it needs to inspect them) but before they have been written to the client
// (because it needs to change the headers and the data writer).
func compressResponse(w *ResponseWriter, req *http.Request) {
	// rudimentary "can this be compressed" check
	if !compressable(w.Header().Get("Content-Type")) {
		return
	}
	// do not re-encode
	if w.Header().Get("Content-Encoding") != "" {
		return
	}
	ae := req.Header.Get("Accept-Encoding")
	// no q for u
	switch {
	case strings.Contains(ae, "gzip"):
		w.WrapBodyWriter(func(w io.Writer) io.Writer {
			return gzip.NewWriter(w)
		})
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("content-length")
		break
	case strings.Contains(ae, "deflate"):
		w.WrapBodyWriter(func(w io.Writer) io.Writer {
			def, _ := flate.NewWriter(w, flate.DefaultCompression)
			return def
		})
		w.Header().Set("Content-Encoding", "deflate")
		w.Header().Del("content-length")
		break
	}
}

// Compress response data when applicable (client wants it and response is
// suitable)
func CompressWrapper(h SimpleHandler, ctx *Context) error {
	ctx.Response.AddAfterHeaderFunc(func(w *ResponseWriter) {
		compressResponse(w, ctx.Request)
	})
	return h(ctx)
}
