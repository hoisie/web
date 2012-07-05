package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"github.com/fiber/web.go"
)

var form = `<form action="say" method="POST"><input name="said"><input type="submit"></form>`

var users = map[string]string{}

var maxRand = big.NewInt(9223372036854775807) // 2^63 - 1

func main() {
	//rand.Seed(time.Now())
	web.Config.CookieSecret = "7C19QRmwf3mHZ9CPAaPQ0hsWeufKd"
	web.Get("/said", func() string { return form })
	web.Post("/say", func(ctx *web.Context) string {
		n, err := rand.Int(rand.Reader, maxRand)
		if err != nil {
			return fmt.Sprintf(`<b style='color: red'>Error when generating number: %d</b>`, err.Error())
		}

		uid := n.String()
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
