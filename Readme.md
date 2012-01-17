# web.go

web.go is the simplest way to write web applications in the Go programming language. It's ideal for writing simple, performant backend web services. 

## Overview

web.go should be familiar to people who've developed websites with higher-level web frameworks like sinatra or web.py. It is designed to be a lightweight web framework that doesn't impose any scaffolding on the user. Some features include:

* Routing to url handlers based on regular expressions
* Secure cookies
* Support for fastcgi and scgi
* Web applications are compiled to native code. This means very fast execution and page render speed
* Serving static files

## Installation

Make sure you have the a working Go environment. See the [install instructions](http://golang.org/doc/install.html). web.go targets the Go `release` branch. If you use the `weekly` branch you may have difficulty compiling web.go. There's an alternative web.go branch, `weekly`, that attempts to keep up with the weekly branch.

To install web.go, simply run:

    goinstall github.com/hoisie/web.go

To compile it from source:

    git clone git://github.com/hoisie/web.go.git
    cd web.go && make install

## Example
    
    package main
    
    import (
        "github.com/hoisie/web.go"
    )
    
    func hello(val string) string { return "hello " + val } 
    
    func main() {
        web.Get("/(.*)", hello)
        web.Run("0.0.0.0:9999")
    }


To run the application, put the code in a file called hello.go and run:

    8g hello.go && 8l -o hello hello.8 && ./hello

You can point your browser to http://localhost:9999/world . 

### Getting parameters

Route handlers may contain a pointer to web.Context as their first parameter. This variable serves many purposes -- it contains information about the request, and it provides methods to control the http connection. For instance, to iterate over the web parameters, either from the URL of a GET request, or the form data of a POST request, you can do the following:

    package main
    
    import (
        "github.com/hoisie/web.go"
    )
    
    func hello(ctx *web.Context, val string) { 
	    for k,v := range ctx.Params {
			println(k, v)
		}
	}
    
    func main() {
        web.Get("/(.*)", hello)
        web.Run("0.0.0.0:9999")
    }

In this example, if you visit `http://localhost:9999/?a=1&b=2`, you'll see the following printed out in the terminal:

    a 1
    b 2

## Documentation

For a quickstart guide, check out [web.go's home page](http://www.getwebgo.com)

There is also a [tutorial](http://www.getwebgo.com/tutorial)

If you use web.go, I'd greatly appreciate a quick message about what you're building with it. This will help me get a sense of usage patterns, and helps me focus development efforts on features that people will actually use. 

## About

web.go was written by [Michael Hoisie](http://hoisie.com). 

Follow me on [Twitter](http://www.twitter.com/hoisie)!

