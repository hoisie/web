// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"code.google.com/p/go.net/websocket"
)

type WebError struct {
	Code int
	Err  string
}

type Context struct {
	Request *http.Request
	RawBody []byte
	Params  map[string]string
	Server  *Server
	User    interface{}
	// False iff 0 bytes of body data have been written so far
	wroteData bool
	http.ResponseWriter
	WebsockConn *websocket.Conn
}

type route struct {
	rex     *regexp.Regexp
	method  string
	handler handlerf
}

var (
	preModules  = []func(*Context) error{}
	postModules = []func(*Context, interface{}) (interface{}, error){}

	Config = &ServerConfig{
		RecoverPanic: true,
		Cert:         "",
		Key:          "",
		ColorOutput:  true,
	}
)

func (err WebError) Error() string {
	return err.Err
}

func (ctx *Context) Write(data []byte) (int, error) {
	ctx.wroteData = true
	return ctx.ResponseWriter.Write(data)
}

func (ctx *Context) WriteString(content string) (int, error) {
	return ctx.Write([]byte(content))
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

func init() {
	// Handle different Accept: types
	AddPostModule(MarshalResponse)
	RegisterMimeParser("application/json", JSONparser)
	RegisterMimeParser("application/xml", XMLparser)
	RegisterMimeParser("text/xml", XMLparser)
	RegisterMimeParser("image/jpeg", Binaryparser)

	// Handle different Accept-Encoding: types
	AddPostModule(EncodeResponse)
}

func AddPreModule(module func(*Context) error) {
	preModules = append(preModules, module)
}

func AddPostModule(module func(*Context, interface{}) (interface{}, error)) {
	postModules = append(postModules, module)
}

func ResetModules() {
	preModules = []func(*Context) error{}
	postModules = []func(*Context, interface{}) (interface{}, error){}
}

func Urlencode(data map[string]string) string {
	var buf bytes.Buffer
	for k, v := range data {
		buf.WriteString(url.QueryEscape(k))
		buf.WriteByte('=')
		buf.WriteString(url.QueryEscape(v))
		buf.WriteByte('&')
	}
	s := buf.String()
	return s[0 : len(s)-1]
}
