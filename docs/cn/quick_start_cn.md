##Web.go
---
###安装
---
在安装 web.go 之前，请先安装 Go 环境，安装完成之后，通过下面的命令可以轻松的安装 web.go:
	
	go get github.com/hoisie/web
	
###开始
先从一个简单的 hello world 程序开始：
	
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
	
###运行
将上面的代码保存到 hello.go，然后在这个目录下面运行如下命令：

	go run hello.go
	
这样，在你的浏览器打开 <http://localhost:9999/world>

###开发指南
更详细的开发指南，[参见这里](./tutorial_cn.md)

###开发
web.go 的源代码在 [github](https://github.com/hoisie/web),欢迎参与开发。