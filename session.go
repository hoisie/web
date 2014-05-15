package web

import (
	"github.com/sipin/web/randbo"
)

var SessionKey string = "ZQSESSID"
var sessionIDLen int = 36

func newSessionID() string {
	return randbo.GenString(sessionIDLen / 2)
}

func (ctx *Context) SetNewSessionID() (sessionID string) {
	sessionID = newSessionID()
	ctx.SetCookie(NewSessionCookie(SessionKey, sessionID))
	return
}

// SetCookie adds a cookie header to the response.
func (ctx *Context) GetSessionID() (sessionID string) {
	cookie, _ := ctx.Request.Cookie(SessionKey)

	if cookie == nil || len(cookie.Value) != sessionIDLen {
		return ctx.SetNewSessionID()
	}
	return cookie.Value
}
