package tpl

import (
	"bytes"
)

func Index(XSRFToken string) string {
	var _buffer bytes.Buffer
	_buffer.WriteString("\n<html>\n    <head>\n        <title>XSRF</title>\n    </head>\n    <body>\n        <p>The form contains a hidden _xsrf form value that will be submitted with this form.</p>\n        <form action=\"/protected\", method=\"post\">\n          <input type=\"text\" name=\"foo\" placeholder=\"CC Number\">\n          <input type=\"text\" name=\"bar\" placeholder=\"Amount\">\n		  ")
	_buffer.WriteString((XSRFToken))
	_buffer.WriteString("\n          <input type=\"submit\" value=\"Submit\">\n        </form>\n    </body>\n</html>\n\n")

	return _buffer.String()
}
