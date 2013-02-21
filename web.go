package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
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

func (err WebError) Error() string {
	return err.Err
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

func (ctx *Context) WriteString(content string) {
	ctx.ResponseWriter.Write([]byte(content))
}

func (ctx *Context) Abort(status int, body string) {
	ctx.ResponseWriter.WriteHeader(status)
	ctx.ResponseWriter.Write([]byte(body))
}

func (ctx *Context) Redirect(status int, url_ string) {
	ctx.ResponseWriter.Header().Set("Location", url_)
	ctx.ResponseWriter.WriteHeader(status)
	ctx.ResponseWriter.Write([]byte("Redirecting to: " + url_))
}

func (ctx *Context) NotModified() {
	ctx.ResponseWriter.WriteHeader(304)
}

func (ctx *Context) NotFound(message string) {
	ctx.ResponseWriter.WriteHeader(404)
	ctx.ResponseWriter.Write([]byte(message))
}

func (ctx *Context) NotAcceptable(message string) {
	ctx.ResponseWriter.WriteHeader(406)
	ctx.ResponseWriter.Write([]byte(message))
}

func (ctx *Context) Unauthorized(message string) {
	ctx.ResponseWriter.WriteHeader(401)
	ctx.ResponseWriter.Write([]byte(message))
}

//Sets the content type by extension, as defined in the mime package. 
//For example, ctx.ContentType("json") sets the content-type to "application/json"
func (ctx *Context) ContentType(ext string) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	ctype := mime.TypeByExtension(ext)
	if ctype != "" {
		ctx.Header().Set("Content-Type", ctype)
	}
}

func (ctx *Context) SetHeader(hdr string, val string, unique bool) {
	if unique {
		ctx.Header().Set(hdr, val)
	} else {
		ctx.Header().Add(hdr, val)
	}
}

// Set a cookie with an explicit path. Duration is the cookie time-to-live in
// seconds (0 = forever).
func (ctx *Context) SetCookiePath(name string, value string, age int64, path string) {
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

//Sets a cookie -- duration is the amount of time in seconds. 0 = forever
func (ctx *Context) SetCookie(name string, value string, age int64) {
	ctx.SetCookiePath(name, value, age, "")
}

func getCookieSig(key string, val []byte, timestamp string) string {
	hm := hmac.New(sha1.New, []byte(key))

	hm.Write(val)
	hm.Write([]byte(timestamp))

	hex := fmt.Sprintf("%02x", hm.Sum(nil))
	return hex
}

func (ctx *Context) SetSecureCookiePath(name string, val string, age int64, path string) {
	//base64 encode the val
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

func (ctx *Context) SetSecureCookie(name string, val string, age int64) {
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

// small optimization: cache the context type instead of repeteadly calling reflect.Typeof
var contextType reflect.Type

var exeFile string

// default
func defaultStaticDir() string {
	root, _ := path.Split(exeFile)
	return path.Join(root, "static")
}

func init() {
	contextType = reflect.TypeOf(Context{})
	//find the location of the exe file
	arg0 := path.Clean(os.Args[0])
	wd, _ := os.Getwd()
	if strings.HasPrefix(arg0, "/") {
		exeFile = arg0
	} else {
		//TODO for robustness, search each directory in $PATH
		exeFile = path.Join(wd, arg0)
	}

	/* Handle different Accept: types */
	AddPostModule(MarshalResponse)
	RegisterMimeParser("application/json", JSONparser)
	RegisterMimeParser("application/xml", XMLparser)
	RegisterMimeParser("text/xml", XMLparser)
	RegisterMimeParser("image/jpeg", Binaryparser)

	/* Handle different Accept-Encoding: types */
	AddPostModule(EncodeResponse)
}

type route struct {
	r       string
	cr      *regexp.Regexp
	method  string
	handler reflect.Value
}

func (s *Server) addRoute(r string, method string, handler interface{}) {
	cr, err := regexp.Compile(r)
	if err != nil {
		s.Logger.Printf("Error in route regex %q\n", r)
		return
	}

	if fv, ok := handler.(reflect.Value); ok {
		s.routes = append(s.routes, route{r, cr, method, fv})
	} else {
		fv := reflect.ValueOf(handler)
		s.routes = append(s.routes, route{r, cr, method, fv})
	}
}

type responseWriter struct {
	http.ResponseWriter
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

//Calls a function with recover block
func (s *Server) safelyCall(function reflect.Value, args []reflect.Value) (resp []reflect.Value, e interface{}) {
	defer func() {
		if err := recover(); err != nil {
			if !s.Config.RecoverPanic {
				// go back to panic
				s.Logger.Printf("Panic: %v", err)
				panic(err)
			} else {
				e = err
				resp = nil
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
	return function.Call(args), nil
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
	ctx := Context{req, nil, map[string]string{}, s, w, nil, false}

	//log the request
	var logEntry bytes.Buffer
	fmt.Fprintf(&logEntry, "\033[32;1m%s %s\033[0m", req.Method, requestPath)

	//ignore errors from ParseForm because it's usually harmless.
	req.ParseForm()
	if len(req.Form) > 0 {
		for k, v := range req.Form {
			ctx.Params[k] = v[0]
		}
		fmt.Fprintf(&logEntry, "\n\033[37;1mParams: %v\033[0m\n", ctx.Params)
	} else {
		// If ParseForm was successful, than the Body will be empty
		if req.Body != nil {
			var err error
			ctx.RawBody, err = ioutil.ReadAll(req.Body)
			if err != nil {
				req.Body = nil
			}
		}
		if req.Body == nil {
			ctx.RawBody = make([]byte, 0)
		}
	}

	ctx.Server.Logger.Print(logEntry.String())

	//set some default headers
	ctx.SetHeader("Server", "web.go", true)
	tm := time.Now().UTC()
	ctx.SetHeader("Date", webTime(tm), true)

	//try to serve a static file
	staticDir := s.Config.StaticDir
	if staticDir == "" {
		staticDir = defaultStaticDir()
	}
	staticFile := path.Join(staticDir, requestPath)
	if fileExists(staticFile) && (req.Method == "GET" || req.Method == "HEAD") {
		http.ServeFile(&ctx, req, staticFile)
		return
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

		// lets call our pre modules to do any processing
		// before we start the request
		for _, module := range preModules {
			// If a module returns an error, we stop process the request
			err := module(&ctx)
			if err != nil {
				ctx.Abort(err.(WebError).Code, err.Error())
				return
			}
		}

		var args []reflect.Value
		handlerType := route.handler.Type()
		if requiresContext(handlerType) {
			args = append(args, reflect.ValueOf(&ctx))
		}
		for _, arg := range match[1:] {
			args = append(args, reflect.ValueOf(arg))
		}

		ret, err := s.safelyCall(route.handler, args)

		if len(ret) == 0 {
			s.Logger.Printf("Handler gave 0 return values")
			ctx.Abort(500, "Server Error")
			return
		}

		// Backwards compatability, if there is only one return,
		// assume there was no error
		if len(ret) > 1 && !ret[1].IsNil() {
			err = ret[1].Interface()
			//there was an error or panic while calling the handler
			s.Logger.Printf("Handler returned error: (%s)%v", reflect.TypeOf(err).String(), err)
			if reflect.TypeOf(err).String() == "web.WebError" {
				ctx.Abort(err.(WebError).Code, err.(WebError).Error())
			} else {
				ctx.Abort(500, fmt.Sprintf("%v", err))
			}
			return
		}
		sval := ret[0]

		// Now we have the content from our response. We should run
		// our post processing modules now
		content := sval.Interface()
		if ctx.wroteData {
			// Data was already sent to the client; do not transform anything
			content = []byte(content.(string))
		} else {
			for _, module := range postModules {
				// If a module returns an error, we stop process the request
				content, err = module(&ctx, content)
				if err != nil {
					s.Logger.Printf("PostModule Error: %v", err)
					ctx.Abort(err.(WebError).Code, err.(WebError).Error())
					return
				}
			}
		}

		if content != nil {
			typed_content, ok := content.([]byte)
			if ok {
				_, err := ctx.Write(typed_content)
				if err != nil {
					s.Logger.Printf("Content write error: %v", err)
					ctx.Abort(500, err.Error())
				}
			} else {
				ctx.Abort(406, "Could not marshal response")
			}
		}
		return
	}

	//try to serve index.html || index.htm
	if indexPath := path.Join(path.Join(staticDir, requestPath), "index.html"); fileExists(indexPath) {
		http.ServeFile(&ctx, ctx.Request, indexPath)
		return
	}

	if indexPath := path.Join(path.Join(staticDir, requestPath), "index.htm"); fileExists(indexPath) {
		http.ServeFile(&ctx, ctx.Request, indexPath)
		return
	}

	ctx.Abort(404, "Page not found")
}

var Config = &ServerConfig{
	RecoverPanic: true,
}

var mainServer = NewServer()

type Server struct {
	Config *ServerConfig
	routes []route
	Logger *log.Logger
	Env    map[string]interface{}
	//save the listener so it can be closed
	l net.Listener
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
}

//Runs the web application and serves http requests
func (s *Server) Run(addr string) {
	s.initServer()

	mux := http.NewServeMux()
	/*
		mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	*/
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

//Runs the web application and serves http requests
func Run(addr string) {
	mainServer.Run(addr)
}

//Runs the secure web application and serves https requests
func RunSecure(addr string, config tls.Config) {
	mainServer.RunSecure(addr, config)
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

var preModules = []func(*Context) error{}
var postModules = []func(*Context, interface{}) (interface{}, error){}

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

//Runs the web application and serves scgi requests for this Server object.
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

type ServerConfig struct {
	StaticDir    string
	Addr         string
	Port         int
	CookieSecret string
	RecoverPanic bool
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
