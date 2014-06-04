package tpl

import (
	"bytes"
)

func Login() string {
	var _buffer bytes.Buffer
	_buffer.WriteString("<html>\n    <head>\n        <title>Martini CSRF</title>\n    </head>\n    <body>\n       <p>This form simulates a login and generates a new session id. You can put whatever you want in these inputs,\n       it doesn't matter.</p>\n       <form action=\"/login\" method=\"post\">\n           <input type=\"text\" name=\"username\" placeholder=\"username\">\n           <input type=\"password\" name=\"password\" placeholder=\"password\">\n           <input type=\"submit\" valid=\"Login\">\n       </form>\n    </body>\n</html>\n\n")

	return _buffer.String()
}
