// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/xyproto/web"
)

// Wrap handlers (WARNING: Obsolete code! need to write example for new API)

// This function will be called prior to each web request
func AuthHandler(h web.SimpleHandler, ctx *web.Context) error {
	ctx.User = "Passed from AuthHandler"
	fmt.Println(ctx.Request.Header)
	return nil
}

func Hello(ctx *web.Context, s string) string {
	return " " + s + ":" + ctx.User.(string)
}

func main() {
	// Wrap everything in the AuthHandler
	web.AddWrapper(AuthHandler)
	web.Get("/(.*)", Hello)
	web.Run(":3000")
}
