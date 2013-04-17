package web

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"
	"time"

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
	Config *ServerConfig
	routes []*route
	Logger *log.Logger
	Env    map[string]interface{}
	// Save the listener so it can be closed
	l net.Listener
	// Passed verbatim to every handler on every request
	User interface{}
}

var (
	exeFile string

	preModules  = []func(*Context) error{}
	postModules = []func(*Context, interface{}) (interface{}, error){}

	Config = &ServerConfig{
		RecoverPanic: true,
		Cert:         "",
		Key:          "",
		ColorOutput:  true,
	}

	mainServer = NewServer()
)

func (err WebError) Error() string {
	return err.Err
}

func (ctx *Context) WriteString(content string) {
	ctx.Write([]byte(content))
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
func (ctx *Context) ContentType(ext string) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	ctype := mime.TypeByExtension(ext)
	if ctype != "" {
		ctx.Header().Set("Content-Type", ctype)
	}
}

func (ctx *Context) SetHeader(hdr, val string, unique bool) {
	if unique {
		ctx.Header().Set(hdr, val)
	} else {
		ctx.Header().Add(hdr, val)
	}
}

// Default
func defaultStaticDir() string {
	root, _ := path.Split(exeFile)
	return path.Join(root, "static")
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

	// Handle different Accept: types
	AddPostModule(MarshalResponse)
	RegisterMimeParser("application/json", JSONparser)
	RegisterMimeParser("application/xml", XMLparser)
	RegisterMimeParser("text/xml", XMLparser)
	RegisterMimeParser("image/jpeg", Binaryparser)

	// Handle different Accept-Encoding: types
	AddPostModule(EncodeResponse)
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

func (ctx *Context) Write(data []byte) (int, error) {
	ctx.wroteData = true
	return ctx.ResponseWriter.Write(data)
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
func (s *Server) applyHandler(f handlerf, ctx *Context, groups []string) (err error) {
	softerr, harderr := s.safelyCall(func() error {
		return f(ctx, groups...)
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

// Fully clothed request handler
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	requestPath := req.URL.Path

	ctx := &Context{
		Request:        req,
		RawBody:        nil,
		Params:         map[string]string{},
		Server:         s,
		ResponseWriter: w,
		User:           s.User,
		wroteData:      false,
	}

	// shortcut if websocket
	if req.Header.Get("Upgrade") == "websocket" {
		s.Logger.Println("handling websocket request: ", requestPath)
		route, match := findMatchingRoute(req, s.routes)
		if route != nil {
			h := func(ctx *Context, args ...string) (err error) {
				websocket.Handler(func(ws *websocket.Conn) {
					ctx.WebsockConn = ws
					err = route.handler(ctx, args...)
				}).ServeHTTP(ctx.ResponseWriter, req)
				return err
			}
			s.applyHandler(h, ctx, match[1:])
		}
	}

	//log the request
	var logEntry bytes.Buffer
	if s.Config.ColorOutput {
		fmt.Fprintf(&logEntry, "\033[32;1m%s %s\033[0m", req.Method, requestPath)
	} else {
		fmt.Fprintf(&logEntry, "%s %s", req.Method, requestPath)
	}

	//ignore errors from ParseForm because it's usually harmless.
	req.ParseForm()
	if len(req.Form) > 0 {
		for k, v := range req.Form {
			ctx.Params[k] = v[0]
		}
		if s.Config.ColorOutput {
			fmt.Fprintf(&logEntry, "\n\033[37;1mParams: %v\033[0m\n", ctx.Params)
		} else {
			fmt.Fprintf(&logEntry, "\nParams: %v\n", ctx.Params)
		}

	}

	ctx.Server.Logger.Print(logEntry.String())

	//set some default headers
	ctx.SetHeader("Server", "web.go", true)
	tm := time.Now().UTC()
	ctx.SetHeader("Date", webTime(tm), true)

	//try to serve static files
	staticDirs := s.Config.StaticDirs
	if len(staticDirs) == 0 {
		staticDirs = []string{defaultStaticDir()}
	}
	for _, staticDir := range staticDirs {
		staticFile := path.Join(staticDir, requestPath)
		if fileExists(staticFile) && (req.Method == "GET" || req.Method == "HEAD") {
			http.ServeFile(ctx, req, staticFile)
			return
		}
	}

	//Set the default content-type
	ctx.SetHeader("Content-Type", "text/html; charset=utf-8", true)

	route, match := findMatchingRoute(req, s.routes)
	if route != nil {
		s.applyHandler(route.handler, ctx, match[1:])
		return
	}
	// Try to serve index.html || index.htm
	indexFilenames := []string{"index.html", "index.htm"}
	for _, staticDir := range staticDirs {
		for _, indexFilename := range indexFilenames {
			if indexPath := path.Join(path.Join(staticDir, requestPath), indexFilename); fileExists(indexPath) {
				http.ServeFile(ctx, ctx.Request, indexPath)
				return
			}
		}
	}
	ctx.Abort(404, "Page not found")
	return
	return
}

func NewServer() *Server {
	return &Server{
		Config: Config,
		Logger: log.New(os.Stdout, "", log.Ldate|log.Ltime),
		Env:    map[string]interface{}{},
	}
}

func (s *Server) initServer() {
	if s.Config == nil {
		s.Config = &ServerConfig{}
	}

	if s.Logger == nil {
		s.Logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	}
	// Set two commonly used mimetypes that are often not set by default
	// Handy for robots.txt and favicon.ico
	mime.AddExtensionType(".txt", "text/plain; charset=utf-8")
	mime.AddExtensionType(".ico", "image/x-icon")
}

func (s *Server) createServeMux(addr string) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/", s)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("Listen:", err)
	}
	s.l = l

	return mux, err
}

//Runs the web application and serves http requests
func (s *Server) Run(addr string) {
	s.initServer()

	mux := http.NewServeMux()
	mux.Handle("/", s)

	s.Logger.Printf("web.go serving %s\n", addr)

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
	s.l = l
	err = http.Serve(s.l, mux)
	s.l.Close()
}

//Runs the secure web application and serves https requests
func (s *Server) RunSecure(addr string, config tls.Config) error {
	s.initServer()
	mux := http.NewServeMux()
	mux.Handle("/", s)

	l, err := tls.Listen("tcp4", addr, &config)
	if err != nil {
		return err
	}

	s.l = l
	return http.Serve(s.l, mux)
}
func (s *Server) RunTLS(addr string, cert string, key string) {
	s.initServer()

	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	mux.Handle("/", s)

	s.Logger.Printf("web.go serving %s\n", addr)
	/*
	   l, err := net.Listen("tcp", addr)
	   if err != nil {
	       log.Fatal("ListenAndServe:", err)
	   }
	   s.l = l
	   err = http.Serve(s.l, mux)
	   s.l.Close()
	*/
	err := http.ListenAndServeTLS(addr, cert, key, mux)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}

}

//Runs the web application and serves http requests
func Run(addr string) {
	mainServer.Run(addr)
}

//Runs the secure web application and serves https requests
func RunSecure(addr string, config tls.Config) {
	mainServer.RunSecure(addr, config)
}

func (s *Server) runTLS(addr, certFile, keyFile string) {
	s.initServer()

	mux, err := s.createServeMux(addr)
	s.Logger.Printf("web.go serving with TLS %s\n", addr)

	srv := &http.Server{Handler: mux}

	config := &tls.Config{}
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)

	if err != nil {
		log.Fatal("TLS error:", err)
	}

	tlsListener := tls.NewListener(s.l, config)
	err = srv.Serve(tlsListener)
	s.l.Close()
}

func RunTLS(addr, certFile, keyFile string) {
	mainServer.runTLS(addr, certFile, keyFile)
}

//Stops the web server
func (s *Server) Close() {
	if s.l != nil {
		s.l.Close()
	}
}

//Stops the web server
func Close() {
	mainServer.Close()
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

// Runs a single request, used for testing
func AdHoc(c http.ResponseWriter, req *http.Request) {
	mainServer.ServeHTTP(c, req)
}

func (s *Server) RunScgi(addr string) {
	s.initServer()
	s.Logger.Printf("web.go serving scgi %s\n", addr)
	s.listenAndServeScgi(addr)
}

//Runs the web application and serves scgi requests
func RunScgi(addr string) {
	mainServer.RunScgi(addr)
}

//Runs the web application and serves fcgi requests for this Server object.
func (s *Server) RunFcgi(addr string) {
	s.initServer()
	s.Logger.Printf("web.go serving fcgi %s\n", addr)
	s.listenAndServeFcgi(addr)
}

//Runs the web application by serving fastcgi requests
func RunFcgi(addr string) {
	mainServer.RunFcgi(addr)
}

//Adds a handler for the 'OPTIONS' http method.
func (s *Server) Options(route string, handler interface{}) {
	s.addRoute(route, "OPTIONS", handler)
}

//Adds a handler for the 'GET' http method.
func (s *Server) Get(route string, handler interface{}) {
	s.addRoute(route, "GET", handler)
}

//Adds a handler for the 'POST' http method.
func (s *Server) Post(route string, handler interface{}) {
	s.addRoute(route, "POST", handler)
}

//Adds a handler for the 'PUT' http method.
func (s *Server) Put(route string, handler interface{}) {
	s.addRoute(route, "PUT", handler)
}

//Adds a handler for the 'DELETE' http method.
func (s *Server) Delete(route string, handler interface{}) {
	s.addRoute(route, "DELETE", handler)
}

//Adds a handler for websocket
func (s *Server) Websocket(route string, handler interface{}) {
	s.addRoute(route, "WEBSOCKET", handler)
}

//Adds a handler for the 'OPTIONS' http method.
func Options(route string, handler interface{}) {
	mainServer.addRoute(route, "OPTIONS", handler)
}

//Adds a handler for the 'GET' http method.
func Get(route string, handler interface{}) {
	mainServer.Get(route, handler)
}

//Adds a handler for the 'POST' http method.
func Post(route string, handler interface{}) {
	mainServer.addRoute(route, "POST", handler)
}

//Adds a handler for the 'PUT' http method.
func Put(route string, handler interface{}) {
	mainServer.addRoute(route, "PUT", handler)
}

//Adds a handler for the 'DELETE' http method.
func Delete(route string, handler interface{}) {
	mainServer.addRoute(route, "DELETE", handler)
}

//Adds a handler for websocket
func Websocket(route string, handler interface{}) {
	mainServer.addRoute(route, "WEBSOCKET", handler)
}

func (s *Server) SetLogger(logger *log.Logger) {
	s.Logger = logger
}

func SetLogger(logger *log.Logger) {
	mainServer.Logger = logger
}

func webTime(t time.Time) string {
	ftime := t.Format(time.RFC1123)
	if strings.HasSuffix(ftime, "UTC") {
		ftime = ftime[0:len(ftime)-3] + "GMT"
	}
	return ftime
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
