package main

import (
    "bytes"
    "crypto/md5"
    "fmt"
    "github.com/jmrobles/web.go"
)

func Md5(b []byte) string {
    hash := md5.New()
    hash.Write(b)
    return fmt.Sprintf("%x", hash.Sum())

}

var page = `
<html>
<head><title>Multipart Test</title></head>
<body>
<form action="/multipart" enctype="multipart/form-data" method="POST">

<label for="file"> Please select a File </label>
<input id="file" type="file" name="file"/>
<br>
<label for="input1"> Please write some text </label>
<input id="input1" type="text" name="input1"/>
<br>
<label for="input2"> Please write some more text </label>
<input id="input2" type="text" name="input2"/>
<br>
<input type="submit" name="Submit" value="Submit"/>

</body>
</html>
`

func index() string { return page }

func multipart(ctx *web.Context) string {
    var output bytes.Buffer
    output.WriteString("<p>input1: " + ctx.Params["input1"] + "</p>")
    output.WriteString("<p>input2: " + ctx.Params["input2"] + "</p>")
    output.WriteString("<p>file: " + ctx.Files["file"].Filename + " " + Md5(ctx.Files["file"].Data) + "</p>")
    return output.String()
}

func main() {
    web.Get("/", index)
    web.Post("/multipart", multipart)
    web.Run("0.0.0.0:9999")
}
