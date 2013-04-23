// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Simple web framework.
//
// At the core of web.go are request handlers:
//
//     func helloworld() string {
//         return "hello, world"
//     }
//
// These are hooked up to the routing table using web.go:
//
//     func main() {
//         web.Get("/", helloworld)
//         web.Run("127.0.0.1:9999")
//     }
//
// Now visit http://127.0.0.1:9999 to see the greeting
//
// The routing table is based on regular expressions and allows parameter
// groups:
//
//     func hello(name string) string {
//         return "hello, " + name
//     }
//
//     func main() {
//         web.Get("/(.*)", hello)
//         web.Run("127.0.0.1:9999")
//     }
// 
// Visit http://127.0.0.1:9999/fidodido to see 'hello, fidodido'
//
// Route handlers may contain a pointer to web.Context as their first parameter.
// This variable serves many purposes -- it contains information about the
// request, and it provides methods to control the http connection.
//
// See the examples directory for more examples.
package web
