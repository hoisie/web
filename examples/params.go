// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/hraban/web"
)

// Accessing GET and POST parameters

const page = `
<form action="/process?foo=bar" method="POST">

<p> a <input name=a> 
<p> b <input name=b>
<p> <input type=submit>
`

func root() string {
	return page
}

func process(ctx *web.Context) string {
	ctx.ContentType("txt")
	return fmt.Sprintf("%#v", ctx.Params)
}

func main() {
	web.Get("/", root)
	web.Post("/process", process)
	web.Run("127.0.0.1:9999")
}
