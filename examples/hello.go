// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/xyproto/web"
)

// Simple hello world application in web.go

func hello(val string) string {
	return "Hello " + val
}

func main() {
	web.Get("/(.*)", hello)
	web.Run(":3000")
}
