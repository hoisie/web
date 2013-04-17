package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type ResponseWriter interface {
	Header() http.Header
	WriteHeader(status int)
	Write(data []byte) (n int, err error)
	Close()
}

type WebError struct {
	Code int
	Err  string
}

type responseWriter struct {
	http.ResponseWriter
}

type Context struct {
	Request *http.Request
	RawBody []byte
	Params  map[string]string
	Server  *Server
	ResponseWriter
	User interface{}
	// False iff 0 bytes of body data have been written so far
	wroteData bool
}

// internal handler type. handler of slightly differing signatures are accepted
// but transformed (wrapped) early on to match this one.
type handlerf func(ctx *Context, arg ...string) error

type route struct {
	r       string
	cr      *regexp.Regexp
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
	routes []route
	Logger *log.Logger
	Env    map[string]interface{}
	// Save the listener so it can be closed
	l net.Listener
	// Passed verbatim to every handler on every request
	User interface{}
}

var (
	// Small optimization: cache the context type instead of repeteadly calling reflect.Typeof
	contextType reflect.Type

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
	ctx.Write([]byte(body))
}

func (ctx *Context) Redirect(status int, url_ string) {
	ctx.Header().Set("Location", url_)
	ctx.WriteHeader(status)
	ctx.Write([]byte("Redirecting to: " + url_))
}

func (ctx *Context) NotModified() {
	ctx.WriteHeader(304)
}

func (ctx *Context) NotFound(message string) {
	ctx.WriteHeader(404)
	ctx.Write([]byte(message))
}

func (ctx *Context) NotAcceptable(message string) {
	ctx.WriteHeader(406)
	ctx.Write([]byte(message))
}

func (ctx *Context) Unauthorized(message string) {
	ctx.WriteHeader(401)
	ctx.Write([]byte(message))
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

// Set a cookie with an explicit path. Duration is the cookie time-to-live in
// seconds (0 = forever).
func (ctx *Context) SetCookiePath(name, value string, age int64, path string) {
	var utctime time.Time
	if age == 0 {
		// 2^31 - 1 seconds (roughly 2038)
		utctime = time.Unix(2147483647, 0)
	} else {
		utctime = time.Unix(time.Now().Unix()+age, 0)
	}
	cookie := http.Cookie{Name: name, Value: value, Expires: utctime, Path: path}
	ctx.SetHeader("Set-Cookie", cookie.String(), false)
}

// Sets a cookie -- duration is the amount of time in seconds. 0 = forever
func (ctx *Context) SetCookie(name, value string, age int64) {
	ctx.SetCookiePath(name, value, age, "")
}

func getCookieSig(key string, val []byte, timestamp string) string {
	hm := hmac.New(sha1.New, []byte(key))

	hm.Write(val)
	hm.Write([]byte(timestamp))

	hex := fmt.Sprintf("%02x", hm.Sum(nil))
	return hex
}

func (ctx *Context) SetSecureCookiePath(name, val string, age int64, path string) {
	// base64 encode the value
	if len(ctx.Server.Config.CookieSecret) == 0 {
		ctx.Server.Logger.Println("Secret Key for secure cookies has not been set. Please assign a cookie secret to web.Config.CookieSecret.")
		return
	}
	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	encoder.Write([]byte(val))
	encoder.Close()
	vs := buf.String()
	vb := buf.Bytes()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	sig := getCookieSig(ctx.Server.Config.CookieSecret, vb, timestamp)
	cookie := strings.Join([]string{vs, timestamp, sig}, "|")
	ctx.SetCookiePath(name, cookie, age, path)
}

func (ctx *Context) SetSecureCookie(name, val string, age int64) {
	ctx.SetSecureCookiePath(name, val, age, "")
}

func (ctx *Context) GetSecureCookie(name string) (string, bool) {
	for _, cookie := range ctx.Request.Cookies() {
		if cookie.Name != name {
			continue
		}

		parts := strings.SplitN(cookie.Value, "|", 3)

		val := parts[0]
		timestamp := parts[1]
		sig := parts[2]

		if getCookieSig(ctx.Server.Config.CookieSecret, []byte(val), timestamp) != sig {
			return "", false
		}

		ts, _ := strconv.ParseInt(timestamp, 0, 64)

		if time.Now().Unix()-31*86400 > ts {
			return "", false
		}

		buf := bytes.NewBufferString(val)
		encoder := base64.NewDecoder(base64.StdEncoding, buf)

		res, _ := ioutil.ReadAll(encoder)
		return string(res), true
	}
	return "", false
}

// Default
func defaultStaticDir() string {
	root, _ := path.Split(exeFile)
	return path.Join(root, "static")
}

func init() {
	contextType = reflect.TypeOf(Context{})
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

// functions according to reflect
type valuefun func([]reflect.Value) []reflect.Value

// waiting for go1.1
func callableValue(fv reflect.Value) valuefun {
	if fv.Type().Kind() != reflect.Func {
		panic("not a function value")
	}
	return func(args []reflect.Value) []reflect.Value {
		return fv.Call(args)
	}
}

// Wrap f in a function that disregards its first arg
func disregardFirstArg(f valuefun) valuefun {
	return func(args []reflect.Value) []reflect.Value {
		return f(args[1:])
	}
}

var nilerr error
var nilerrv reflect.Value = reflect.ValueOf(&nilerr).Elem()

// Wrap f to return a nil error value in addition to current return values
func addNilErrorReturn(f valuefun) valuefun {
	return func(args []reflect.Value) []reflect.Value {
		ret := f(args)
		return append(ret, nilerrv)
	}
}

// Wrap f to write its string return value to the first arg (being an io.Writer)
// requires the original function signature to be:
//
// func (io.Writer, ...) (string, error)
//
// signature of wrapped function:
//
// func (io.Writer, ...) error
//
// if the error value of the original call is not nil that value is passed back
// verbatim and no further action is taken. If it is nil the wrapper writes the
// string to the writer and returns whatever error ocurred there, if any.
//
// Note that wherever it says string []byte is also okay.
func writeStringToFirstArg(f valuefun) valuefun {
	return func(args []reflect.Value) []reflect.Value {
		wv := args[0]
		w, ok := wv.Interface().(io.Writer)
		if !ok {
			panic("First argument must be an io.Writer")
		}
		ret := f(args)
		if len(ret) < 2 {
			panic("Two return values required for proper wrapping")
		}
		if i := ret[1].Interface(); i != nil {
			return ret[1:]
		}
		var ar []byte
		if i := ret[0].Interface(); i != nil {
			switch typed := i.(type) {
			case string:
				ar = []byte(typed)
				break
			case []byte:
				ar = typed
				break
			default:
				panic("First return value must be a byte array / string")
			}
		}
		_, err := w.Write(ar)
		if err != nil {
			return []reflect.Value{reflect.ValueOf(err)}
		}
		return []reflect.Value{nilerrv}
	}
}

var errtype reflect.Type = reflect.TypeOf((*error)(nil)).Elem()

func lastRetIsError(fv reflect.Value) bool {
	// type of fun
	t := fv.Type()
	if t.NumOut() == 0 {
		return false
	}
	// type of last return val
	t = t.Out(t.NumOut() - 1)
	return t.Implements(errtype)
}

func firstRetIsString(fv reflect.Value) bool {
	// type of fun
	t := fv.Type()
	if t.NumOut() == 0 {
		return false
	}
	// type of first return val
	t = t.Out(0)
	return t.AssignableTo(reflect.TypeOf("")) || t.AssignableTo(reflect.TypeOf([]byte{}))
}

// convert a value back to the original error interface. panics if value is not
// nil and also does not implement error.
func value2error(v reflect.Value) error {
	i := v.Interface()
	if i == nil {
		return nil
	}
	return i.(error)
}

// Beat the supplied handler into a uniform signature. panics if incompatible
// (may only happen when the wrapped fun is called)
func fixHandlerSignature(f interface{}) handlerf {
	fv := reflect.ValueOf(f)
	var callf valuefun = callableValue(fv)
	if !requiresContext(fv.Type()) {
		callf = disregardFirstArg(callf)
	}
	// now callf definitely accepts a *Context as its first arg
	if !lastRetIsError(fv) {
		callf = addNilErrorReturn(callf)
	}
	// now callf definitely returns an error as its last value
	if firstRetIsString(fv) {
		callf = writeStringToFirstArg(callf)
	}
	// now callf definitely does not return a string: just an error
	// wrap callf in a function with pretty signature
	return func(ctx *Context, args ...string) error {
		argvs := make([]reflect.Value, len(args)+1)
		argvs[0] = reflect.ValueOf(ctx)
		for i, arg := range args {
			argvs[i+1] = reflect.ValueOf(arg)
		}
		rets := callf(argvs)
		return value2error(rets[0])
	}
}

func (s *Server) addRoute(r string, method string, handler interface{}) {
	cr, err := regexp.Compile(r)
	if err != nil {
		s.Logger.Printf("Error in route regex %q\n", r)
		return
	}
	s.routes = append(s.routes, route{
		r:       r,
		cr:      cr,
		method:  method,
		handler: fixHandlerSignature(handler),
	})
}

func (c *responseWriter) Close() {
	rwc, buf, _ := c.ResponseWriter.(http.Hijacker).Hijack()
	if buf != nil {
		buf.Flush()
	}

	if rwc != nil {
		rwc.Close()
	}
}

func (ctx *Context) Write(data []byte) (int, error) {
	ctx.wroteData = true
	return ctx.ResponseWriter.Write(data)
}

func (s *Server) ServeHTTP(c http.ResponseWriter, req *http.Request) {
	w := responseWriter{c}
	s.routeHandler(req, &w)
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

//should the context be passed to the handler?
func requiresContext(handlerType reflect.Type) bool {
	//if the method doesn't take arguments, no
	if handlerType.NumIn() == 0 {
		return false
	}

	//if the first argument is not a pointer, no
	a0 := handlerType.In(0)
	if a0.Kind() != reflect.Ptr {
		return false
	}
	//if the first argument is a context, yes
	if a0.Elem() == contextType {
		return true
	}

	return false
}

func (s *Server) routeHandler(req *http.Request, w ResponseWriter) {
	requestPath := req.URL.Path

	ctx := Context{
		Request:        req,
		RawBody:        nil,
		Params:         map[string]string{},
		Server:         s,
		ResponseWriter: w,
		User:           s.User,
		wroteData:      false,
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
			http.ServeFile(&ctx, req, staticFile)
			return
		}
	}

	//Set the default content-type
	ctx.SetHeader("Content-Type", "text/html; charset=utf-8", true)

	for i := 0; i < len(s.routes); i++ {
		route := s.routes[i]
		cr := route.cr
		//if the methods don't match, skip this handler (except HEAD can be used in place of GET)
		if req.Method != route.method && !(req.Method == "HEAD" && route.method == "GET") {
			continue
		}

		if !cr.MatchString(requestPath) {
			continue
		}
		match := cr.FindStringSubmatch(requestPath)

		if len(match[0]) != len(requestPath) {
			continue
		}

		softerr, harderr := s.safelyCall(func() error {
			return route.handler(&ctx, match[1:]...)
		})
		if harderr != nil {
			//there was an error or panic while calling the handler
			ctx.Abort(500, "Server Error")
		}
		if softerr != nil {
			// TODO: if softer.(WebError) ...
			s.Logger.Printf("Handler returned error: %v", softerr)
			ctx.Abort(500, "Server Error")
		}
		return
	}

	// Try to serve index.html || index.htm
	indexFilenames := []string{"index.html", "index.htm"}
	for _, staticDir := range staticDirs {
		for _, indexFilename := range indexFilenames {
			if indexPath := path.Join(path.Join(staticDir, requestPath), indexFilename); fileExists(indexPath) {
				http.ServeFile(&ctx, ctx.Request, indexPath)
				return
			}
		}
	}

	ctx.Abort(404, "Page not found")
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

func MethodHandler(val interface{}, name string) reflect.Value {
	v := reflect.ValueOf(val)
	typ := v.Type()
	n := typ.NumMethod()
	for i := 0; i < n; i++ {
		m := typ.Method(i)
		if m.Name == name {
			return v.Method(i)
		}
	}

	return reflect.ValueOf(nil)
}
