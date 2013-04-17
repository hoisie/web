package main

import (
	"fmt"

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
	fmt.Fprint(ctx, "<a href=f>non-nil error")
	return nil
}

func f(ctx *web.Context) error {
	return fmt.Errorf("oh no!")
}

func root() string {
	return "<a href=a>start testing: simple handler"
}

func main() {
	web.Get("/", root)
	web.Get("/a", a)
	web.Get("/b", b)
	web.Get("/c", c)
	web.Get("/d", d)
	web.Get("/e", e)
	web.Get("/f", f)
	web.Run("0.0.0.0:8081")
}
