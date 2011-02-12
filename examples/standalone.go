package main

import (
    "web"
)

func hello(val string) string { return "hello " + val }

func main() {
    web.Get("/(.*)", hello)
    web.RunApp("0.0.0.0:9999")
}
