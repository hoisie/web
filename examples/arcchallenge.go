package main

import (
    "web"
)

var input = ""

var tmpl = `<form action="say" method="POST"><input name="said"><input type="submit"></form>`

func main() {
    web.Get("/said", func() string { return tmpl })
    web.Post("/say", func(req *web.Request) string {
        input = req.Form["said"][0]
        return `<a href="/final">Click Here</a>`
    })
    web.Get("/final", func() string { return "You said " + input })
    web.Run("0.0.0.0:9999")
}
