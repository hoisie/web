---
layout: default
title: Quickstart
---

# Web.go

## Install it

Web.go assumes you have a working Go environment. If you do installation is a breeze:

    go get github.com/hoisie/web

## Getting started

This is a simple hello world web application, written in Go:

{% highlight go %}
package main

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

## Run it

Put the above code in a file called hello.go. To run it, simply run the following command

    go run hello.go

Then point your browser to http://localhost:9999/world

## Tutorial

For more detailed information about writing apps in web.go, check out the [tutorial](/tutorial.html)

## Development

Follow development of web.go on [github](http://github.com/hoisie/web)
