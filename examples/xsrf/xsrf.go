package main

import (
	"github.com/sipin/web"
	"github.com/sipin/web/examples/xsrf/tpl"
)

func getUid(ctx *web.Context) string {
	uid, _ := ctx.GetSecureCookie("user")
	return uid
}

func index(ctx *web.Context) {
	uid := getUid(ctx)
	if uid == "" {
		ctx.Redirect("/login")
		return
	}

	ctx.WriteString(tpl.Index(web.XSRFFormField(ctx)))
}

func login(ctx *web.Context) {
	ctx.WriteString(tpl.Login())
}

func loginPost(ctx *web.Context) {
	ctx.SetSecureCookie("user", "user", 0)
	ctx.Redirect("/")
	return
}

func protectedPost(ctx *web.Context) {
	uid := getUid(ctx)
	if uid == "" {
		ctx.Redirect("/login")
		return
	}
	if !web.XSRFValidate(ctx) {
		ctx.Unauthorized()
		return
	}
	ctx.WriteString(tpl.Result("You submitted a valid token"))
}

func main() {
	web.Config.CookieSecret = "cv$2!"
	web.SetXSRFOption("ab12#3", getUid)

	web.Get("/", index)
	web.Get("/login", login)
	web.Post("/login", loginPost)
	web.Post("/protected", protectedPost)

	web.Run("0.0.0.0:9999")
}
