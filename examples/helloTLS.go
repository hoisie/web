// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/hraban/web"
)

func hello(val string) string { return "hello " + val }

func main() {
	web.Get("/(.*)", hello)
	// You can create an example cert using generate_cert.go include in pkg crypto/tls
	web.RunTLS(":9999", "/tmp/cert.pem", "/tmp/key.pem")
}
