// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"code.google.com/p/go.net/websocket"
)

// The web.go custom request context that is passed to every request handler.
type Context struct {
	// The incoming request that led to this handler being invoked
	Request *http.Request
	RawBody []byte
	// Aggregated parameters from the query string and POST data.
	Params map[string]string
	Server *Server
	// Copied from Server.User before the handler is invoked. Use this to
	// communicate global state between your handlers.
	User interface{}
	// False iff 0 bytes of body data have been written so far
	wroteData bool
	// The response writer that the handler should write to.
	http.ResponseWriter
	// In the case of websocket: a reference to the connection object. Nil
	// otherwise.
	WebsockConn *websocket.Conn
}

func (ctx *Context) Write(data []byte) (int, error) {
	ctx.wroteData = true
	return ctx.ResponseWriter.Write(data)
}

func (ctx *Context) WriteString(content string) (int, error) {
	return ctx.Write([]byte(content))
}

// Best-effort serialization of response data
func (ctx *Context) writeAnything(i interface{}) error {
	switch typed := i.(type) {
	case string:
		_, err := ctx.Write([]byte(typed))
		return err
	case []byte:
		_, err := ctx.Write(typed)
		return err
	case io.WriterTo:
		_, err := typed.WriteTo(ctx)
		return err
	case io.Reader:
		_, err := io.Copy(ctx, typed)
		return err
	}
	return errors.New("cannot serialize data for writing to client")
}

func (ctx *Context) Abort(status int, body string) {
	ctx.WriteHeader(status)
	ctx.WriteString(body)
}

func (ctx *Context) Redirect(status int, url_ string) {
	ctx.Header().Set("Location", url_)
	ctx.Abort(status, "Redirecting to: "+url_)
}

func (ctx *Context) NotModified() {
	ctx.WriteHeader(304)
}

func (ctx *Context) NotFound(message string) {
	ctx.Abort(404, message)
}

func (ctx *Context) NotAcceptable(message string) {
	ctx.Abort(406, message)
}

func (ctx *Context) Unauthorized(message string) {
	ctx.Abort(401, message)
}

// Sets the content type by extension, as defined in the mime package.
// For example, ctx.ContentType("json") sets the content-type to "application/json"
// if the supplied extension contains a slash (/) it is set as the content-type
// verbatim without passing it to mime.  returns the content type as it was
// set, or an empty string if none was found.
func (ctx *Context) ContentType(ext string) string {
	ctype := ""
	if strings.ContainsRune(ext, '/') {
		ctype = ext
	} else {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		ctype = mime.TypeByExtension(ext)
	}
	if ctype != "" {
		ctx.Header().Set("Content-Type", ctype)
	}
	return ctype
}

func (ctx *Context) SetHeader(hdr, val string, unique bool) {
	if unique {
		ctx.Header().Set(hdr, val)
	} else {
		ctx.Header().Add(hdr, val)
	}
}
