package main

import (
    "fmt"
    "web"
)

type mytype struct {
    A   string
    B   string
    C   int
    D   int64
}

var page = `
<html>
<head><title>Multipart Test</title></head>
<body>
<form action="/process" method="POST">

<label for="a"> Please write some text </label>
<input id="a" type="text" name="a"/>
<br>
<label for="b"> Please write some more text </label>
<input id="b" type="text" name="b"/>
<br>
<label for="c"> Please write a number </label>
<input id="c" type="text" name="c"/>
<br>
<label for="d"> Please write another number </label>
<input id="d" type="text" name="d"/>
<br>
<input type="submit" name="Submit" value="Submit"/>

</body>
</html>
`

func index() string { return page }

func process(ctx *web.Context) string {
    var data mytype
    ctx.UnmarshalParams(&data)
    return fmt.Sprintf("%v\n", data)
}

func main() {
    web.Get("/", index)
    web.Post("/process", process)
    web.Run("0.0.0.0:9999")
}
