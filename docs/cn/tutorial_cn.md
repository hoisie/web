##Web.go 开发指南
---
###开始
---
安装说明可以[参见这里](./quick_start_cn.md)。
几乎在所有的开发指南都会以 Hello world 作为学习的起点，下面来看一下用 Go 写的这个 web 程序：

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
	
###URL处理 （URL Handling）
---
对于 Web 应用程序来说，非常重要的部分就是当给出一个 URL 路径的时候，要知道怎样的去响应这个请求。 Web.go 提供了一些基于路由模型的方法来处理这些事情。

* web.Get 用于处理大部分的普通 GET 请求。它有两个参数：一个是包含URL地址匹配的正则表达式，另一个是处理响应的函数。
* web.Post 用于处理 POST 请求的方法，参数设置和 `web.Get` 一样。
* web.Put 和 web.Delete 分别用于处理 PUT 和 DELETE 请求的方法。
* web.Match 能匹配任意的 HTTP 方法，它有三个参数，第一个是 HTTP 的方法名，第二个是路径匹配的正则表达式，第三个是处理函数。其实和其他方法的区别就在于这个第一个参数是 HTTP 的方法名。在源代码中其实都是封装了 `mainServer.addRoute(route, method, handler)` 而 web.Get,web.Post等传递的 method 参数是响应的 `GET`, `POST` 等。

####路由处理程序(Route handlers)
---
在上面的 hello world 程序中，对于能够匹配正则表达式 `/(.*)` 的地址的 HTTP 请求，回去调用下面的函数：

	func hello(val string) string { 
    	return "hello " + val 
	}

由于通配符正则表达式，所以`/` 之后的所有的内容都会作为 `val` 参数传递给 hello 程序。 这就要求我们的处理函数的参数个数要和我们的 URL 模型中的正则表达式的组数相同。

虽然通常情况下处理函数会返回一个字符串，但是 web.go 也允许处理函数不返回任何值。 这种情况下，这些方法主要负责将数据写入到响应中。如下面的程序等同于前面的程序：

	package main

	import (
    	"github.com/hoisie/web"
	)

	func hello(ctx *web.Context, val string) { 
    	ctx.WriteString("hello " + val)
	} 

	func main() {
    	web.Get("/(.*)", hello)
	    web.Run("0.0.0.0:9999")
	}
	
上面代码中写入 Context 变量的详细内容，我们在后面会做进一步分说明。

###web.Context类型(The web.Context type)
---
在 web.go 中，任何的处理函数都可以添加一个 `web.Context` 指针作为第一个参数。 这个对象包含了一些请求的信息，并且提供了一些控制响应的方法。

这里对 `web.Context` 包含的信息做了些简短的摘要：

* ctx.Request 是 `http.Request` 类型的结构体，可以用于检索 HTTP 请求的详细信息，包括参数 `params`,头信息`headers` 和 文件`files`。
* ctx.Params 是 `map[string]string` 类型，包含了请求的参数。它是 `ctx.Request.Form` 的扁平版本，主要就是方便。
* ctx.ResponseWriter 是一个用于连接的 `http.ResponseWriter`。 可以用它来设置响应码 `reponse code`,头信息`headers` 或者写入正文 `write to the body`。
* ctx.Server 是当前连接的 `web.Server` 对象。 它包含了一些 server 配置的详细信息。

通过 `web.Context` 对象，可以访问 HTTP 的响应，通过它你也可以设置状态码，响应的头，还能直接写入到连接中。

####设置响应码(Setting a response code)
---
通过传递一个数字的状态码作为 `ctx.WriteHeader` 的参数来设置 HTTP 响应的状态码。这里还有一些比较方便的方法来设置一些常用的状态码：

* ctx.NotFound 返回一个错误信息和 404 错误码.
* ctx.Abort 可以传递一个状态码和信息作为参数. 对于返回 5xx 的错信息很有用。
* ctx.Redirect 通常用于地址跳转，可以传入错误码 301 或者 302 要转入的 URL 地址.
* ctx.NotModified 设置 304 状态码。

当然你也可以通过 `ctx.WriteHeader(code)`设置任意的 `header code`.其实上面的几个方法都在内部封装了 `ctx.WriteHeader(code)` 方法。

你只能调用 `ctx.WriteHeader` 一次，一旦被调用过，那么之后通过它设置的 HTTP 的response headers将无效。

####设置响应头(Setting response headers)
---
通过 `web.Context` 的 `SetHeader` ，可以很方便的设置HTTP的 response headers. 它的前两个参数是 header 的键和值，第三个参数 unique，是一个布尔值，用来觉得是否覆盖当前的已有的值。

你也可以通过调用 `Header` 来得到当前的 response header，它返回的是一个map类型值。并且可以通过 Add 和 Set 来设置 HTTP headers. 当 `WriteHeader` 被调用的时候，这个map类型的值就会被写入到响应中。

	package main

	import (
    	"github.com/hoisie/web"
	)

	func hello(ctx *web.Context, val string) string {
    	ctx.SetHeader("X-Powered-By", "web.go", true)
	    ctx.SetHeader("X-Frame-Options", "DENY", true)
    	ctx.SetHeader("Connection", "close", true)
	    return "hello " + val 
	} 

	func main() {
    	web.Get("/(.*)", hello)
	    web.Run("0.0.0.0:9999")
	}
	
####设置cookies(Setting cookies)
---
`web.Context` 有一个 `SetCookie` 方法，它设置cookie的过程是通过获取 `http.Cookie` 对象，然后将其作为 header的 `Set-Cookie`值来进行。它经常和 `web.NewCookie`一起使用。 比如下面我们修改一下 hello world 例子，设置一个cookie 到响应中：

	package main

	import (
    	"github.com/hoisie/web"
	)

	func hello(ctx *web.Context, val string) string {
    	ctx.SetCookie(web.NewCookie("value", val))
	    return "hello " + val 
	} 

	func main() {
    	web.Get("/(.*)", hello)
	    web.Run("0.0.0.0:9999")
	}
	
####写入正文(Wirting to the body)
----
通过 `web.Context` 中的 `Write` 和 `WriteString`，你可以直接写入当前的连接的正文. `WriteHeader(200)` 将会在你第一次调用 `Write` 的时候，除非你在这之前调用过。

一个 `web.Context`对象可以作为 `io.Writer` interface 使用。在我们将 `io.Reader` 对象写入到 HTTP 响应的正文的时候非常有用。通常用于将缓冲区的内容写到响应中，或用于将缓冲区中的内容写到文件或者管道(pipe)中.

	package main

	import (
	    "bytes"
	    "github.com/hoisie/web"
	    "io"
	)
	
	func hello(ctx *web.Context, val string) {
	    var buf bytes.Buffer
	    buf.WriteString("hello " + val)
	    //copy buf directly into the HTTP response
	    io.Copy(ctx, &buf)
	} 
	
	func main() {
	    web.Get("/(.*)", hello)
	    web.Run("0.0.0.0:9999")
	}

使用 `ctx.Write` ，web.go 允许你维护一个长连接，并且定期的返回一些结果。详细的情况可以参见 [streaming](https://github.com/hoisie/web/blob/master/examples/streaming.go)实例.

注意，当调用 `ctx.Write` 和 `ctx.WriteString` 的时候，他们的内容会一直被缓存起来直到处理方法完成。 很多 ResponseWriter 都有一个 Flush 方法，通过这个方法可以清除掉 HTTP 连接中的内容。参见 [streaming](https://github.com/hoisie/web/blob/master/examples/streaming.go)实例.

###模板(Templates)
---
在渲染 HTML 的时候，模板库非常重要。 web.go 没有包含自己的模板库。 但是网上有些不错的模板库可以使用，比如 [Go’s http/template library](http://golang.org/pkg/html/template/)或者[mustache](http://github.com/hoisie/mustache)

###静态文件(Static Files)
---
Web.go 能很高效的实现静态文件的服务。 如果你将文件放到你的应用程序的静态文件目录，当你请求的文件名和目录下文件同名的时候，web.go就会做出响应。

比如，你有一个web应用，网络地址是 myapp.com，在服务器上的路径是 $HOME/app. 在你的服务器路径 $HOME/app/static 下有图片image.jpg.那么当你请求 myapp.com/image.jpg的时候，应用程序就会去调用服务器上的$HOME/app/static/image.jpg 文件。一种通用的做法是在你的这个 static路径下建立诸如 static/images, static/sylesheets 和 static/javascripts 等文件夹来存放你的静态文件。

注意，web.go 会同时在你的应用程序的 static 路径和你当前的工作路径下查找静态文件。 你也可以设置 `web.ServerConfig.StaticDir` 来指定一个特殊的目录为静态文件目录。

###共享主机(Shared hosts)
---
web.go 为应用程序能够使用 `SCGI` 和 `FastCGI` 协议提供了一些方法。这就使得 web.go 应用程序能够运行在共享主机的环境下。

这些方法类似 `web.Run`:

* web.RunScgi(addr) 提供 `SCGI` 服务请求，实例如下。
* web.RunFcgi(addr) 提供 `FastCGI` 服务请求.
* web.RunTLS(addr, tlsContext) 提供 `HTTPS` 服务请求. 详情参加 [example](https://github.com/hoisie/web/blob/master/examples/tls.go).

比如使上面的 hello world 程序运行在 SCGI 协议下，只要做如下改动即可：

	package main
	
	import (
	    "github.com/hoisie/web"
	)
	
	func hello(val string) string { 
	    return "hello " + val 
	} 
	
	func main() {
	    web.Get("/(.*)", hello)
	    web.RunScgi("0.0.0.0:6580")
	}

接下来就是你要设置你的服务器通过端口 6580 来发送请求。

###开发web.go(Developing web.go)
---
如果你有一个问题需要在 web.go 下面调试，你又想去修改 web.go 的源代码。

默认情况下，当你通过 `go get` 命令来安装程序，那么安装程序会获取所有的源码，并且将 package 安装在你的 `$GOPATH` 路径下。如果你的 `$GOPATH` 没有设置，那么会将库安装到 `$GOROOT/pkg` 文件夹下,这个路径是 Go 默认编译路径。 作为一个 Go 程序库的开发者来说，设置 `$GOPATH` 环境变量到诸如 $HOME/golibs 或 $HOME/projects/golibs 下是很有必要的，也是很方便的。详情请参考[怎样写 Go 代码](http://golang.org/doc/code.html)。 开发 web.go 的第一步就是确定你设置了 `$GOPATH` 并且在此路径下包括了子文件夹 `src` 和 `pkg`。

下一步就是到你的 `$GOPATH/src` 下，使用 `git clone github.com/hoisie/web` 来克隆源代码。当源代码克隆完之后，到路径 `$GOPATH/src/web` 下使用 `go install` 来安装，我们的 web 包将被安装到 `$GOPATH/pkg` 下。

从现在开始，如果你想使用 web.go ，那么你应该 import web. 它这个时候就会链接所有 $GOPATH/pkg 下的文件。 你可以修改 `$GOPATH/src/web` 下 web.go 的源代码，重新通过 `go install` 编译，然后再重新编译你自己的目标程序。



