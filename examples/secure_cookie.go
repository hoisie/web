package main

import (
	"fmt"
	"github.com/hoisie/web"
	"html"
)

var cookieName = "cookie"

var notice = `
<div>%v</div>
`
var form = `
<form method="POST" action="update">
  <div class="field">
    <label for="cookie"> Set a cookie: </label>
    <input id="cookie" name="cookie"> </input>
  </div>

  <input type="submit" value="Submit"></input>
  <input type="submit" name="submit" value="Delete"></input>
</form>
`

func index(ctx *web.Context) string {
	cookie, ok := ctx.GetSecureCookie(cookieName)
	var top string
	if !ok {
		top = fmt.Sprintf(notice, "The cookie has not been set")
	} else {
		var val = html.EscapeString(cookie)
		top = fmt.Sprintf(notice, "The value of the cookie is '"+val+"'.")
	}
	return top + form
}

func update(ctx *web.Context) {
	if ctx.Params["submit"] == "Delete" {
		ctx.SetCookie(web.NewCookie(cookieName, "", -1))
	} else {
		ctx.SetSecureCookie(cookieName, ctx.Params["cookie"], 0)
	}
	ctx.Redirect(301, "/")
}

func main() {
	web.Config.CookieSecret = "a long secure cookie secret"
	web.Get("/", index)
	web.Post("/update", update)
	web.Run("0.0.0.0:9999")
}
