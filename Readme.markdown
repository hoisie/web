web.go is the simplest way to write web apps in go. Here is a simple example:

    package main

    import (
      "web";
    )

    var urls = map[string] interface {} {
      "/(.*)" : hello,
    }

    func hello (val string) string {
     return "hello "+val;
    }

    func main() {
      web.Run(urls, "0.0.0.0:9999");
    }
