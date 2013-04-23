// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/hraban/web"
)

var users = map[string]string{}

func root() string {
	return `store something in a secure cookie: 
<form action="/say" method="POST">
  <input name="said">
  <input type="submit" value="go">
</form>`
}

func say(ctx *web.Context) string {
	uid := strconv.FormatInt(rand.Int63(), 10)
	ctx.SetSecureCookie("user", uid, 3600)
	users[uid] = ctx.Params["said"]
	ctx.Redirect(303, "/final")
	return `<a href="/final">Click Here</a>`
}

func final(ctx *web.Context) string {
	uid, _ := ctx.GetSecureCookie("user")
	return "You said " + users[uid]
}

func main() {
	rand.Seed(time.Now().UnixNano())
	web.Config.CookieSecret = "7C19QRmwf3mHZ9CPAaPQ0hsWeufKd"
	web.Get("/", root)
	web.Post("/say", say)
	web.Get("/final", final)
	web.Run("127.0.0.1:9999")
}
