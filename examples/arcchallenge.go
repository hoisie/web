package main

import (
	"github.com/hoisie/web"
	"math/rand"
	"strconv"
	"time"
)

var form = `<form action="say" method="POST"><input name="said"><input type="submit"></form>`

var users = map[string]string{}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	web.Config.CookieSecret = "7C19QRmwf3mHZ9CPAaPQ0hsWeufKd"
	web.Get("/said", func() string { return form })
	web.Post("/say", func(ctx *web.Context) string {
		uid := strconv.FormatInt(rand.Int63(), 10)
		ctx.SetSecureCookie("user", uid, 3600)
		users[uid] = ctx.Params["said"]
		return `<a href="/final">Click Here</a>`
	})
	web.Get("/final", func(ctx *web.Context) string {
		uid, _ := ctx.GetSecureCookie("user")
		return "You said " + users[uid]
	})
	web.Run("0.0.0.0:9999")
}
