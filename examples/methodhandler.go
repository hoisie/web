package main

import (
    "web"
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
