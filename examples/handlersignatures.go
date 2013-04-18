package main

import (
	"fmt"
	"net/http"

	"github.com/hraban/web"
)

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

func g(ctx *web.Context) error {
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
	web.Get("/f", myhandlertype("non-nil error"))
	web.Get("/g", g)
	web.Run("0.0.0.0:8081")
}
