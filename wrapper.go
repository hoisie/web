// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import ()

// Called for every request and passed the handler that web.go thinks should be
// called to process this specific request. Use this to do some global
// tinkering like:
//
// * specialized error pages (if werr, ok := err.(WebError); ok { ... })
//
// * encode data if client supports it (gzip etc)
//
// * set site-wide headers
//
// Note that when a wrapper is called by web.go the actual handler itself is
// NOT called by web.go it must be called by the wrapper. This allows
// fine-grained control over the context in which to call it and what to do
// with potential errors.
//
// The handler does not have to be a user-defined handler object: web.go
// creates handlers on the fly to handle static files, 404 situations and
// handler signature mismatches. It is whatever web.go WOULD have called if a
// wrapper were not defined.
type Wrapper func(SimpleHandler, *Context) error

// Bind a simple request handler to a wrapper
func wrapHandler(wrapper Wrapper, bareh SimpleHandler) SimpleHandler {
	return func(ctx *Context) error {
		return wrapper(bareh, ctx)
	}
}
