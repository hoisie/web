// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"

	"github.com/hraban/web"
)

// Different types of error handling in web.go

func notfound() error {
	return web.WebError{404, "Not Found, no sir!"}
}

func teapot() error {
	return web.WebError{418, "je suis un teapot"}
}

func generic() error {
	return errors.New("gaston!")
}

func root() string {
	return `errors: <li><a href=404>not found</a><li><a href=teapot>ima teapot
		<li><a href=generic>unexpected server-side error`
}

func main() {
	web.Get("/", root)
	web.Get("/404", notfound)
	web.Get("/teapot", teapot)
	web.Get("/generic", generic)
	web.Run("0.0.0.0:8081")
}
