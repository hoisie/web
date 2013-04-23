// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Simple hello world application in web.go
package main

import (
	"github.com/hraban/web"
)

func hello(val string) string {
	return "Hello " + val
}

func main() {
	web.Get("/(.*)", hello)
	web.Run("127.0.0.1:9999")
}
