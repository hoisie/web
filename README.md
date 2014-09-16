web.go
======

[![Build Status](https://travis-ci.org/xyproto/web.svg?branch=master)](https://travis-ci.org/xyproto/web)
[![GoDoc](https://godoc.org/github.com/xyproto/web?status.svg)](http://godoc.org/github.com/xyproto/web)

web.go is the simplest way to write web applications in the Go programming language. It's ideal for writing simple, performant backend web services. 

## Overview

web.go should be familiar to people who've developed websites with higher-level web frameworks like sinatra or web.py. It is designed to be a lightweight web framework that doesn't impose any scaffolding on the user. Some features include:

* Routing to url handlers based on regular expressions
* Secure cookies
* Support for fastcgi and scgi
* Web applications are compiled to native code. This means very fast execution and page render speed
* Efficiently serving static files

## Installation

Make sure you have the a working Go environment. See the [install instructions](http://golang.org/doc/install.html). web.go targets the Go `release` branch.

To install web.go, simply run:

    go get github.com/xyproto/web

To compile it from source:

    git clone git://github.com/xyproto/web.git
    cd web && go build

## Example
```go
package main
    
import (
    "github.com/xyproto/web"
)
    
func hello(val string) string { return "hello " + val } 
    
func main() {
    web.Get("/(.*)", hello)
    web.Run(":3000")
}
```

To run the application, put the code in a file called hello.go and run:

    go run hello.go
    
You can point your browser to http://localhost:3000/world . 

### Getting parameters

Route handlers may contain a pointer to web.Context as their first parameter. This variable serves many purposes -- it contains information about the request, and it provides methods to control the http connection. For instance, to iterate over the web parameters, either from the URL of a GET request, or the form data of a POST request, you can access `ctx.Params`, which is a `map[string]string`:

```go
package main

import (
    "github.com/xyproto/web"
)
    
func hello(ctx *web.Context, val string) { 
    for k,v := range ctx.Params {
		println(k, v)
	}
}   
    
func main() {
    web.Get("/(.*)", hello)
    web.Run(":3000")
}
```

In this example, if you visit `http://localhost:3000/?a=1&b=2`, you'll see the following printed out in the terminal:

    a 1
    b 2

## Documentation

API docs are hosted at http://webgo.io

If you use web.go, Michael Hoisie would greatly appreciate a quick message about what you're building with it. This will help him get a sense of usage patterns, and helps him to focus development efforts on features that people will actually use. 

## About

web.go was originally written by [Michael Hoisie](http://hoisie.com). 


