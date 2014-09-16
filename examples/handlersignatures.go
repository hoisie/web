// Copyright © 2009--2014 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/xyproto/web"
)

// Explore the different possible handler signatures

func a() string {
	return "<a href=b>return byte array"
}

func b() []byte {
	return []byte("<a href=c>string and error return values")
}

func c() (string, error) {
	return "<a href=d>context argument", nil
}

func d(ctx *web.Context) (string, error) {
	// should be concatenated in output
	fmt.Fprint(ctx, "<a href=e>")
	return "only return error value", nil
}

func e(ctx *web.Context) error {
	fmt.Fprint(ctx, "<a href=f>net/http.Handler type")
	return nil
}

type myhandlertype string

func (m myhandlertype) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("<a href=g>" + m))
}

func g(ctx *web.Context) io.Reader {
	return strings.NewReader("<a href=h>return io.WriterTo")
}

type towriter struct{ data []byte }

func (t towriter) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(t.data)
	return int64(n), err
}

func h(ctx *web.Context) io.WriterTo {
	return towriter{[]byte("<a href=i>no return value")}
}

func i(ctx *web.Context) {
	ctx.Write([]byte("<a href=j>non-nil error"))
}

func j(ctx *web.Context) error {
	return fmt.Errorf("oh no!")
}

func root() string {
	return "<a href=a>start: simple handler"
}

func main() {
	web.Get("/", root)
	web.Get("/a", a)
	web.Get("/b", b)
	web.Get("/c", c)
	web.Get("/d", d)
	web.Get("/e", e)
	web.Get("/f", myhandlertype("return io.Reader"))
	web.Get("/g", g)
	web.Get("/h", h)
	web.Get("/i", i)
	web.Get("/j", j)
	web.Run(":3000")
}
