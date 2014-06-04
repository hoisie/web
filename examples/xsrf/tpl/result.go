package tpl

import (
	"bytes"
	"github.com/sipin/gorazor/gorazor"
)

func Result(result string) string {
	var _buffer bytes.Buffer
	_buffer.WriteString("\n<html>\n    <head>\n        <title>XSRF</title>\n    </head>\n    <body>\n        <h3>")
	_buffer.WriteString(gorazor.HTMLEscape(result))
	_buffer.WriteString("</h3>\n    </body>\n</html>\n\n")

	return _buffer.String()
}
