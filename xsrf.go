// XSRF support
// call web.SetXSRFOption to setup option
// call XSRFFormField to get a hidden form field, then add to the form
// before process POST request, use XSRFValidate to validate _xsrf form value

package web

import (
	"net/http"
	"time"

	"code.google.com/p/xsrftoken"
)

func getXSRFToken(server *Server, ctx *Context) {
	if token, ok := ctx.GetSecureCookie("_xsrf"); ok && token != "" {
		ctx.XSRFToken = token
	} else {
		if ctx.Server.XSRFGetUid == nil {
			return
		}
		uid := ctx.Server.XSRFGetUid(ctx)
		if uid == "" {
			return
		}
		ctx.XSRFToken = xsrftoken.Generate(ctx.Server.XSRFSecret, uid, "POST")
		ctx.SetSecureCookie("_xsrf", ctx.XSRFToken, int64(xsrftoken.Timeout/time.Second))
	}
}

func XSRFValidate(ctx *Context) bool {
	if ctx.XSRFToken == "" {
		return false
	}
	if ctx.XSRFToken == XSRFGetFormToken(ctx.Request) {
		return true
	}
	return false
}

func XSRFFormField(ctx *Context) string {
	return "<input type=\"hidden\" name=\"_xsrf\" value=\"" +
		ctx.XSRFToken + "\"/>"
}

func XSRFGetFormToken(r *http.Request) string {
	return r.FormValue("_xsrf")
}
