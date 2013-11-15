package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"github.com/hoisie/web"
	"io"
)

func Md5(r io.Reader) string {
	hash := md5.New()
	io.Copy(hash, r)
	return fmt.Sprintf("%x", hash.Sum(nil))
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
</form>
</body>
</html>
`

func index() string { return page }

func multipart(ctx *web.Context) string {
	ctx.Request.ParseMultipartForm(10 * 1024 * 1024)
	form := ctx.Request.MultipartForm
	var output bytes.Buffer
	output.WriteString("<p>input1: " + form.Value["input1"][0] + "</p>")
	output.WriteString("<p>input2: " + form.Value["input2"][0] + "</p>")

	fileHeader := form.File["file"][0]
	filename := fileHeader.Filename
	file, err := fileHeader.Open()
	if err != nil {
		return err.Error()
	}

	output.WriteString("<p>file: " + filename + " " + Md5(file) + "</p>")
	return output.String()
}

func main() {
	web.Get("/", index)
	web.Post("/multipart", multipart)
	web.Run("0.0.0.0:9999")
}
