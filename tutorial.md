---
layout: default
title: Tutorial
---

# Web.go tutorial

## Getting started

For installation insructions, check out the [quickstart guide](/index.html)

This is the hello world web application, written in Go, which this tutorial uses:

package main

{% highlight go %}
import (
    "github.com/hoisie/web"
)

func hello(val string) string { 
    return "hello " + val 
} 

func main() {
    web.Get("/(.*)", hello)
    web.Run("0.0.0.0:9999")
}
{% endhighlight %}

## URL Handling

The most important part of a web application is knowing how to respond to a given URL path. Web.go provides several methods to add handlers based on a route pattern. These include:

* web.Get( pattern, handler ) - adds a handler for the most common GET method
* web.Post( pattern, handler ) - adds a handler for the POST method
* There's also web.Put and web.Delete with a similar signature

### Route handlers

In the hello example above, the `/(.*)` url pattern is matched to the following function:

{% highlight go %}
func hello(val string) string { 
    return "hello " + val 
} 
{% endhighlight %}

In that example, everything after the `/` is passed to the argument _val_ in the hello function. A handler function is required to have the same number of arguments as regular expression groups in its url pattern. 

Although handlers usually return a string, web.go also accepts route handlers that have no return value. These methods are responsible for writing data to the client:

{% highlight go %}
func hello(ctx *web.Context, val string) { 
    ctx.WriteString ( "hello " + val) 
} 
{% endhighlight %}

Writing to the context variable is explained in a later section.  

## The web.Context

There's an optional first variable in every route handler - a pointer to web.Context. This variable serves many purposes -- it contains information about the request, and it provides methods to control the http connection.

Here is a brief summary of web.Context:

* ctx.Request - a struct which has information about the request, such as the params, headers, and files
* ctx.Request.Params - a map which contains either GET or POST params
* ctx.Request.Files - a map which contains multipart-encoded files
* ctx.Request.Headers - a map which has the request headers

### Methods on web.Context:

* ctx.Write - writes a byte array to the http connection. If the response is not set, it automatically sends a response with code 200
* ctx.WriteString - writes a string to the connection. If the response is not set, it automatically sends a response with code 200
* ctx.SetHeader (name, val, unique) - sets a response HTTP header
* ctx.SetCookie (name, val, age) - sets a cookie (name) val with age in seconds
* ctx.Close - closes the underlying http connection
* ctx.Abort (code, message) - used to send 5xx status messages
* ctx.Redirect ( code, url ) - used to send 3xx satus messages

## Templates

Web.go doesn't include a templating library. However, there are several good ones available, such as [mustache.go](http://github.com/hoisie/mustache). The template package in Go is not recommended for web.go because it doesn't allow templates to be embedded within each other, which causes a lot of duplicated text. 

## Static Files

Web.go has the ability to serve static files in a very efficient way. If you place files in the `static` directory of your web application, web.go will serve them if a request matches the name of the file.

For example, if you have a web app in `$HOME/app` being served from `myapp.com`, and there's a file `$HOME/app/static/image.jpg`, requesting `myapp.com/image.jpg` will serve the static file. A common practice is to have `static/images`, `static/stylesheets`, and `static/javascripts` that contain static files. 

Note that Web.go is looking for the `static` directory in both the directory where the web app is located and your current working directory. You can also change web.ServerConfig.StaticDir to specify a directory.

## Shared hosts

Web.go provides methods to run web applications using the SCGI or FastCGI protocols. This
enables web.go apps to run in shared hosts environments. 

These methods are similar to web.Run:

* web.RunScgi(addr) - serves SCGI requests
* web.RunFcgi(addr) - serves FCGI request
* web.RunTLS(addr, tlsContext) - serves HTTPS request

For instance, to serve the hello example above running Scgi, just write the following:

{% highlight go %}
func main() {
    web.Get("/(.*)", hello)
    web.RunScgi("0.0.0.0:6580")
}
{% endhighlight %}

Next you need to configure your web server to pass requests along to port 6580. 

## Developing web.go

If you have an issue you'd like to debug in web.go, you'll want to modify the source code of the project.

By default, when you run `go get package`, the installer will fetch the source code and install the package in the directory specified by `$GOPATH`. If `$GOPATH` is not set, the library is installed in the `$GOROOT/pkg` folder, which defaults to where Go is built. As a developer of Go libraries, it is much more convenient to set a `$GOPATH` variable to a location like `$HOME/golibs` or `$HOME/projects/golibs`. See [how to write Go code](http://golang.org/doc/code.html) for more details. The first step to developing web.go is to ensure that you have an appropriate `$GOPATH` with two sub-directories: `src` and `pkg`. 

Next, you should run `cd $GOPATH/src && git clone github.com/hoisie/web`. This creates a clone of the web.go repo in `$GOPATH/src/web`. Next, run `cd GOPATH/src/web && go install`. This will install the `web` package into `$GOPATH/pkg`. 

From now on, if you include web.go, you should use `import web`, and this will link to the files that are in `$GOPATH/pkg`. You can now modify the web.go source code in `$GOPATH/src/web`, rebuild with `go install`, and then rebuild your target appication.


