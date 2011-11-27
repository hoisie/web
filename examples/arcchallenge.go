package main

import (
    "math/rand"
    "strconv"
    "time"
    "web"
)

var form = `<form action="say" method="POST"><input name="said"><input type="submit"></form>`

var users = map[string]string{}

func main() {
    rand.Seed(time.Nanoseconds())
    web.Config.CookieSecret = "7C19QRmwf3mHZ9CPAaPQ0hsWeufKd"
    web.Get("/said", func() string { return form })
    web.Post("/say", func(ctx *web.Context) string {
        uid := strconv.Itoa64(rand.Int63())
        ctx.SetSecureCookie("user", uid, 3600)
        users[uid] = ctx.Request.Params["said"]
        return `<a href="/final">Click Here</a>`
    })
    web.Get("/final", func(ctx *web.Context) string {
        uid, _ := ctx.GetSecureCookie("user")
        return "You said " + users[uid]
    })
    web.Run("0.0.0.0:9999")
}
