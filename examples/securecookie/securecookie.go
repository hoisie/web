// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Sample web.go application that stores user data in a secure cookie
package main

import (
	"github.com/xyproto/web"
)

// Store user data in a secure cookie. Fake data cannot be supplied, only
// original data or no data.

const form = `store something in a secure cookie:
<form action="/say" method="POST">
  <input name="said">
  <input type="submit" value="go">
</form>`

func root(ctx *web.Context) string {
	msg := form
	if said, ok := ctx.GetSecureCookie("said"); ok {
		msg = "You said " + said + "<p>" + msg
	}
	return msg
}

func say(ctx *web.Context) {
	ctx.SetSecureCookie("said", ctx.Params["said"], 3600)
	ctx.Redirect(303, "/")
	return
}

func main() {
	web.Config.CookieSecret = "7C19QRmwf3mHZ9CPAaPQ0hsWeufKd"
	web.Get("/", root)
	web.Post("/say", say)
	web.Run(":3000")
}
