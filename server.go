// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// this file is about the actual handling of a request: it comes in, what
// happens? routing determines which handler is responsible and that is then
// wrapped appropriately and invoked.

package web

import (
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"
	"time"

	"code.google.com/p/go.net/websocket"
)

type route struct {
	rex     *regexp.Regexp
	method  string
	handler handlerf
}

type ServerConfig struct {
	StaticDirs   []string
	Addr         string
	Port         int
	CookieSecret string
	RecoverPanic bool
	Cert         string
	Key          string
	ColorOutput  bool
}

type Server struct {
	Config ServerConfig
	routes []*route
	Logger *log.Logger
	Env    map[string]interface{}
	// Save the listener so it can be closed
	l net.Listener
	// Passed verbatim to every handler on every request
	User interface{}
	// All requests are passed through this wrapper if defined
	Wrappers []Wrapper
}

var mainServer = NewServer()

// Configuration of the shared server
var Config = &mainServer.Config
var exeFile string

//Stops the web server
func (s *Server) Close() {
	if s.l != nil {
		s.l.Close()
	}
}

// Queue response wrapper that is called after all other wrappers
func (s *Server) AddWrapper(wrap Wrapper) {
	s.Wrappers = append(s.Wrappers, wrap)
}

func (s *Server) SetLogger(logger *log.Logger) {
	s.Logger = logger
}

func (s *Server) addRoute(rawrex string, method string, handler interface{}) {
	rex, err := regexp.Compile(rawrex)
	if err != nil {
		s.Logger.Printf("Error in route regex %q: %v", rawrex, err)
		return
	}
	s.routes = append(s.routes, &route{
		rex:     rex,
		method:  method,
		handler: fixHandlerSignature(handler),
	})
}

// Calls function with recover block. The first return value is whatever the
// function returns if it didnt panic. The second is what was passed to panic()
// if it did.
func (s *Server) safelyCall(f func() error) (softerr error, harderr interface{}) {
	defer func() {
		if err := recover(); err != nil {
			if !s.Config.RecoverPanic {
				// go back to panic
				s.Logger.Printf("Panic: %v", err)
				panic(err)
			} else {
				harderr = err
				s.Logger.Println("Handler crashed with error: ", err)
				for i := 1; ; i += 1 {
					_, file, line, ok := runtime.Caller(i)
					if !ok {
						break
					}
					s.Logger.Println(file, line)
				}
			}
		}
	}()
	return f(), nil
}

// Determine if this route matches this request purely on the basis of the method
func matchRouteMethods(req *http.Request, route *route) bool {
	if req.Method == route.method {
		return true
	}
	if req.Method == "HEAD" && route.method == "GET" {
		return true
	}
	if req.Header.Get("Upgrade") == "websocket" && route.method == "WEBSOCKET" {
		return true
	}
	return false
}

// If this request matches this route return the group matches from the regular
// expression otherwise return an empty slice. note on success the return value
// includes the entire match as the first element.
func matchRoute(req *http.Request, route *route) []string {
	if !matchRouteMethods(req, route) {
		return nil
	}
	match := route.rex.FindStringSubmatch(req.URL.Path)
	if match == nil || len(match[0]) != len(req.URL.Path) {
		return nil
	}
	return match
}

func findMatchingRoute(req *http.Request, routes []*route) (*route, []string) {
	for _, route := range routes {
		if match := matchRoute(req, route); match != nil {
			return route, match
		}
	}
	return nil, nil
}

// Apply the handler to this context and try to handle errors where possible
func (s *Server) applyHandler(f closedhandlerf, ctx *Context) (err error) {
	softerr, harderr := s.safelyCall(func() error {
		return f(ctx)
	})
	if harderr != nil {
		//there was an error or panic while calling the handler
		ctx.Abort(500, "Server Error")
		return fmt.Errorf("%v", harderr)
	}
	if softerr != nil {
		s.Logger.Printf("Handler returned error: %v", softerr)
		if werr, ok := softerr.(WebError); ok {
			ctx.Abort(werr.Code, werr.Error())
		} else {
			// Non-web errors are not leaked to the outside
			ctx.Abort(500, "Server Error")
			err = softerr
		}
	}
	return
}

func dirExists(dir string) bool {
	d, e := os.Stat(dir)
	switch {
	case e != nil:
		return false
	case !d.IsDir():
		return false
	}

	return true
}

func fileExists(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

// Default
func defaultStaticDir() string {
	root, _ := path.Split(exeFile)
	return path.Join(root, "static")
}

// If this request corresponds to a static file return its path
func (s *Server) findFile(req *http.Request) string {
	//try to serve static files
	staticDirs := s.Config.StaticDirs
	if len(staticDirs) == 0 {
		staticDirs = []string{defaultStaticDir()}
	}
	for _, staticDir := range staticDirs {
		staticFile := path.Join(staticDir, req.URL.Path)
		if fileExists(staticFile) && (req.Method == "GET" || req.Method == "HEAD") {
			return staticFile
		}
	}

	// Try to serve index.html || index.htm
	indexFilenames := []string{"index.html", "index.htm"}
	for _, staticDir := range staticDirs {
		for _, indexFilename := range indexFilenames {
			if indexPath := path.Join(path.Join(staticDir, req.URL.Path), indexFilename); fileExists(indexPath) {
				return indexPath
			}
		}
	}
	return ""
}

// Fully clothed request handler
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := &Context{
		Request:        req,
		RawBody:        nil,
		Params:         map[string]string{},
		Server:         s,
		ResponseWriter: w,
		User:           s.User,
		wroteData:      false,
	}

	//log the request
	if s.Config.ColorOutput {
		s.Logger.Printf("\033[32;1m%s %s\033[0m", req.Method, req.URL.Path)
	} else {
		s.Logger.Printf("%s %s", req.Method, req.URL.Path)
	}

	//ignore errors from ParseForm because it's usually harmless.
	req.ParseForm()
	if len(req.Form) > 0 {
		for k, v := range req.Form {
			ctx.Params[k] = v[0]
		}
		if s.Config.ColorOutput {
			s.Logger.Printf("\033[37;1mParams: %v\033[0m\n", ctx.Params)
		} else {
			s.Logger.Printf("Params: %v\n", ctx.Params)
		}

	}

	var simpleh closedhandlerf
	route, match := findMatchingRoute(req, s.routes)
	if route != nil {
		if route.method == "WEBSOCKET" {
			// Wrap websocket handler
			openh := func(ctx *Context, args ...string) (err error) {
				// yo dawg we heard you like wrapped functions
				websocket.Handler(func(ws *websocket.Conn) {
					ctx.WebsockConn = ws
					err = route.handler(ctx, args...)
				}).ServeHTTP(ctx.ResponseWriter, req)
				return err
			}
			simpleh = closeHandler(openh, match[1:]...)
		} else {
			// Set the default content-type
			ctx.ContentType("text/html; charset=utf-8")
			simpleh = closeHandler(route.handler, match[1:]...)
		}
	} else if path := s.findFile(req); path != "" {
		// no custom handler found but there is a file with this name
		simpleh = func(ctx *Context) error {
			http.ServeFile(ctx, ctx.Request, path)
			return nil
		}
	} else {
		// hopeless, 404
		simpleh = func(ctx *Context) error {
			return WebError{404, "Page not found"}
		}
	}
	for _, wrap := range s.Wrappers {
		simpleh = wrapHandler(wrap, simpleh)
	}
	s.applyHandler(simpleh, ctx)
	return
}

func webTime(t time.Time) string {
	ftime := t.Format(time.RFC1123)
	if strings.HasSuffix(ftime, "UTC") {
		ftime = ftime[0:len(ftime)-3] + "GMT"
	}
	return ftime
}

func NewServer() *Server {
	conf := ServerConfig{
		RecoverPanic: true,
		Cert:         "",
		Key:          "",
		ColorOutput:  true,
	}
	s := &Server{
		Config: conf,
		Logger: log.New(os.Stdout, "", log.Ldate|log.Ltime),
		Env:    map[string]interface{}{},
	}
	// Set two commonly used mimetypes that are often not set by default
	// Handy for robots.txt and favicon.ico
	mime.AddExtensionType(".txt", "text/plain; charset=utf-8")
	mime.AddExtensionType(".ico", "image/x-icon")
	// Set some default headers
	s.AddWrapper(func(h closedhandlerf, ctx *Context) error {
		ctx.Header().Set("Server", "web.go")
		tm := time.Now().UTC()
		ctx.Header().Set("Date", webTime(tm))
		return h(ctx)
	})
	return s
}

// Package wide proxy functions for global web server object

// Stop the global web server
func Close() {
	mainServer.Close()
}

// Set a logger to be used by the global web server
func SetLogger(logger *log.Logger) {
	mainServer.Logger = logger
}

// The global web server as an object implementing the http.Handler interface
func GetHTTPHandler() http.Handler {
	return mainServer
}

func init() {
	// find the location of the executable
	arg0 := path.Clean(os.Args[0])
	wd, _ := os.Getwd()
	if strings.HasPrefix(arg0, "/") {
		exeFile = arg0
	} else {
		// TODO For robustness, search each directory in $PATH
		exeFile = path.Join(wd, arg0)
	}
	RegisterMimeParser("application/json", JSONparser)
	RegisterMimeParser("application/xml", XMLparser)
	RegisterMimeParser("text/xml", XMLparser)
	RegisterMimeParser("image/jpeg", Binaryparser)
}
