package main

import (
    "web"
)

var form = `<form action="say" method="POST"><input name="said"><input type="submit"></form>`

func main() {
    web.Get("/said", func() string { return form })
    web.Post("/say", func(ctx *web.Context) string {
        ctx.Session.Data["said"] = ctx.Request.Params["said"][0]
        return `<a href="/final">Click Here</a>`
    })
    web.Get("/final", func(ctx *web.Context) string { return "You said " + ctx.Session.Data["said"].(string) })
    web.Run("0.0.0.0:9999")
}
