package main

import (
	"fmt"

	"github.com/xyproto/web"
)

// This function will be called prior to each web request
func AuthHandler(ctx *web.Context) error {
	ctx.User = "Passed from AuthHandler"
	fmt.Println(ctx.Request.Header)
	return nil
}

func Hello(ctx *web.Context, s string) string {
	return " " + s + ":" + ctx.User.(string)
}

func main() {
	// Add AuthHandler to our PreModule list
	web.AddPreModule(AuthHandler)
	web.Get("/(.*)", Hello)
	web.Run("0.0.0.0:9999")
}
