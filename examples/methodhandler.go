// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/xyproto/web"
)

type Greeter struct {
	greeting string
}

func (g *Greeter) Greet(s string) string {
	return g.greeting + " " + s
}

func main() {
	g := &Greeter{"hello"}
	web.Get("/(.*)", web.MethodHandler(g, "Greet"))
	web.Run("0.0.0.0:9999")
}
