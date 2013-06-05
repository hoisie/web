// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"path"
)

// Guess the mime type of the response based on the request path if not
// explicitly set by the handler
func GuessMimetypeWrapper(h SimpleHandler, ctx *Context) error {
	ctx.Response.AddAfterHeaderFunc(func(w *ResponseWriter) {
		// dont override existing
		if !w.Success() || w.Header().Get("content-type") != "" {
			return
		}
		if ext := path.Ext(ctx.Request.URL.Path); ext != "" {
			ctx.ContentType(ext)
		}
		// if that didnt work let the http package sort it out its got
		// reasonable heuristics
	})
	return h(ctx)
}
