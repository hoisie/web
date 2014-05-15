package web

type Flash struct {
	Alert  string
	Notice string
}

var FlashAlertKey string = "ZQFA"
var FlashNoticeKey string = "ZQFN"

func (ctx *Context) SetFlashAlert(msg string) {
	ctx.SetSecureCookie(FlashAlertKey, msg, 60)
	return
}

func (ctx *Context) SetFlashNotice(msg string) {
	ctx.SetSecureCookie(FlashNoticeKey, msg, 60)
	return

}

func (ctx *Context) GetFlash() *Flash {
	flash := &Flash{}
	var ok bool
	flash.Alert, ok = ctx.GetSecureCookie(FlashAlertKey)
	if ok {
		ctx.RemoveCookie(FlashAlertKey)
	}

	flash.Notice, ok = ctx.GetSecureCookie(FlashNoticeKey)
	if ok {
		ctx.RemoveCookie(FlashNoticeKey)
	}
	return flash
}
