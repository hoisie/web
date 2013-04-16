// Package web is a lightweight web framework for Go. It's ideal for
// writing simple, performant backend web services.
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

// A Context object is created for every incoming HTTP request, and is
// passed to handlers as an optional first argument. It provides information
// about the request, including the http.Request object, the GET and POST params,
// and acts as a Writer for the response.
type Context struct {
    Request *http.Request
    Params  map[string]string
    Server  *Server
    http.ResponseWriter
}

// WriteString writes string data into the response object.
func (ctx *Context) WriteString(content string) {
    ctx.ResponseWriter.Write([]byte(content))
}

// Abort is a helper method that sends an HTTP header and an optional
// body. It is useful for returning 4xx or 5xx errors.
// Once it has been called, any return value from the handler will
// not be written to the response.
func (ctx *Context) Abort(status int, body string) {
    ctx.ResponseWriter.WriteHeader(status)
    ctx.ResponseWriter.Write([]byte(body))
}

// Redirect is a helper method for 3xx redirects.
func (ctx *Context) Redirect(status int, url_ string) {
    ctx.ResponseWriter.Header().Set("Location", url_)
    ctx.ResponseWriter.WriteHeader(status)
    ctx.ResponseWriter.Write([]byte("Redirecting to: " + url_))
}

// Notmodified writes a 304 HTTP response
func (ctx *Context) NotModified() {
    ctx.ResponseWriter.WriteHeader(304)
}

// NotFound writes a 404 HTTP response
func (ctx *Context) NotFound(message string) {
    ctx.ResponseWriter.WriteHeader(404)
    ctx.ResponseWriter.Write([]byte(message))
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

// SetHeader sets a response header. If `unique` is true, the current value
// of that header will be overwritten . If false, it will be appended.
func (ctx *Context) SetHeader(hdr string, val string, unique bool) {
    if unique {
        ctx.Header().Set(hdr, val)
    } else {
        ctx.Header().Add(hdr, val)
    }
}

// SetCookie adds a cookie header to the response.
func (ctx *Context) SetCookie(cookie *http.Cookie) {
    ctx.SetHeader("Set-Cookie", cookie.String(), false)
}

func getCookieSig(key string, val []byte, timestamp string) string {
    hm := hmac.New(sha1.New, []byte(key))

    hm.Write(val)
    hm.Write([]byte(timestamp))

    hex := fmt.Sprintf("%02x", hm.Sum(nil))
    return hex
}

func (ctx *Context) SetSecureCookie(name string, val string, age int64) {
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
    ctx.SetCookie(NewCookie(name, cookie, age))
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

var defaultStaticDirs []string

func init() {
    contextType = reflect.TypeOf(Context{})
    //find the location of the exe file
    wd, _ := os.Getwd()
    arg0 := path.Clean(os.Args[0])
    var exeFile string
    if strings.HasPrefix(arg0, "/") {
        exeFile = arg0
    } else {
        //TODO for robustness, search each directory in $PATH
        exeFile = path.Join(wd, arg0)
    }
    parent, _ := path.Split(exeFile)
    defaultStaticDirs = append(defaultStaticDirs, path.Join(parent, "static"))
    defaultStaticDirs = append(defaultStaticDirs, path.Join(wd, "static"))
    return
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

// ServeHTTP is the interface method for Go's http server package
func (s *Server) ServeHTTP(c http.ResponseWriter, req *http.Request) {
    s.Process(c, req)
}

// Process invokes the main server's routing system.
func Process(c http.ResponseWriter, req *http.Request) {
    mainServer.Process(c, req)
}

// safelyCall invokes `function` in recover block
func (s *Server) safelyCall(function reflect.Value, args []reflect.Value) (resp []reflect.Value, e interface{}) {
    defer func() {
        if err := recover(); err != nil {
            if !s.Config.RecoverPanic {
                // go back to panic
                panic(err)
            } else {
                e = err
                resp = nil
                s.Logger.Println("Handler crashed with error", err)
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

// requiresContext determines whether 'handlerType' contains
// an argument to 'web.Ctx' as its first argument
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

// tryServingFile attempts to serve a static file, and returns
// whether or not the operation is successful.
// It checks the following directories for the file, in order:
// 1) Config.StaticDir
// 2) The 'static' directory in the parent directory of the executable.
// 3) The 'static' directory in the current working directory
func (s *Server) tryServingFile(name string, req *http.Request, w http.ResponseWriter) bool {
    //try to serve a static file
    if s.Config.StaticDir != "" {
        staticFile := path.Join(s.Config.StaticDir, name)
        if fileExists(staticFile) {
            http.ServeFile(w, req, staticFile)
            return true
        }
    } else {
        for _, staticDir := range defaultStaticDirs {
            staticFile := path.Join(staticDir, name)
            if fileExists(staticFile) {
                http.ServeFile(w, req, staticFile)
                return true
            }
        }
    }
    return false
}

// the main route handler in web.go
func (s *Server) routeHandler(req *http.Request, w http.ResponseWriter) {
    requestPath := req.URL.Path
    ctx := Context{req, map[string]string{}, s, w}

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
    }
    ctx.Server.Logger.Print(logEntry.String())

    //set some default headers
    ctx.SetHeader("Server", "web.go", true)
    tm := time.Now().UTC()
    ctx.SetHeader("Date", webTime(tm), true)

    if req.Method == "GET" || req.Method == "HEAD" {
        if s.tryServingFile(requestPath, req, w) {
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

        var args []reflect.Value
        handlerType := route.handler.Type()
        if requiresContext(handlerType) {
            args = append(args, reflect.ValueOf(&ctx))
        }
        for _, arg := range match[1:] {
            args = append(args, reflect.ValueOf(arg))
        }

        ret, err := s.safelyCall(route.handler, args)
        if err != nil {
            //there was an error or panic while calling the handler
            ctx.Abort(500, "Server Error")
        }
        if len(ret) == 0 {
            return
        }

        sval := ret[0]

        var content []byte

        if sval.Kind() == reflect.String {
            content = []byte(sval.String())
        } else if sval.Kind() == reflect.Slice && sval.Type().Elem().Kind() == reflect.Uint8 {
            content = sval.Interface().([]byte)
        }
        ctx.SetHeader("Content-Length", strconv.Itoa(len(content)), true)
        ctx.Write(content)
        return
    }

    // try serving index.html or index.htm
    if req.Method == "GET" || req.Method == "HEAD" {
        if s.tryServingFile(path.Join(requestPath, "index.html"), req, w) {
            return
        } else if s.tryServingFile(path.Join(requestPath, "index.htm"), req, w) {
            return
        }
    }
    ctx.Abort(404, "Page not found")
}

// ServerConfig is configuration for server objects.
type ServerConfig struct {
    StaticDir    string
    Addr         string
    Port         int
    CookieSecret string
    RecoverPanic bool
}

// Server represents a web.go server.
type Server struct {
    Config *ServerConfig
    routes []route
    Logger *log.Logger
    Env    map[string]interface{}
    //save the listener so it can be closed
    l   net.Listener
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

// Process invokes the routing system for server s
func (s *Server) Process(c http.ResponseWriter, req *http.Request) {
    s.routeHandler(req, c)
}

// Run starts the web application and serves HTTP requests for s
func (s *Server) Run(addr string) {
    s.initServer()

    mux := http.NewServeMux()
    mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
    mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
    mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
    mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
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

// Run starts the web application and serves HTTP requests for the main server.
func Run(addr string) {
    mainServer.Run(addr)
}

// RunTLS starts the web application and serves HTTPS requests for s.
func (s *Server) RunTLS(addr string, config *tls.Config) error {
    s.initServer()
    mux := http.NewServeMux()
    mux.Handle("/", s)
    l, err := tls.Listen("tcp", addr, config)
    if err != nil {
        log.Fatal("Listen:", err)
        return err
    }

    s.l = l
    return http.Serve(s.l, mux)
}

// RunTLS starts the web application and serves HTTPS requests for the main server.
func RunTLS(addr string, config *tls.Config) {
    mainServer.RunTLS(addr, config)
}

// RunScgi starts the web application and serves SCGI requests for s.
func (s *Server) RunScgi(addr string) {
    s.initServer()
    s.Logger.Printf("web.go serving scgi %s\n", addr)
    s.listenAndServeScgi(addr)
}

// RunScgi starts the web application and serves SCGI requests for the main server.
func RunScgi(addr string) {
    mainServer.RunScgi(addr)
}

// RunFcgi starts the web application and serves FastCGI requests for s.
func (s *Server) RunFcgi(addr string) {
    s.initServer()
    s.Logger.Printf("web.go serving fcgi %s\n", addr)
    s.listenAndServeFcgi(addr)
}

// RunFcgi starts the web application and serves FastCGI requests for the main server.
func RunFcgi(addr string) {
    mainServer.RunFcgi(addr)
}

// Close stops server s.
func (s *Server) Close() {
    if s.l != nil {
        s.l.Close()
    }
}

// Close stops the main server.
func Close() {
    mainServer.Close()
}

// Get adds a handler for the 'GET' http method for server s.
func (s *Server) Get(route string, handler interface{}) {
    s.addRoute(route, "GET", handler)
}

// Post adds a handler for the 'POST' http method for server s.
func (s *Server) Post(route string, handler interface{}) {
    s.addRoute(route, "POST", handler)
}

// Put adds a handler for the 'PUT' http method for server s.
func (s *Server) Put(route string, handler interface{}) {
    s.addRoute(route, "PUT", handler)
}

// Delete adds a handler for the 'DELETE' http method for server s.
func (s *Server) Delete(route string, handler interface{}) {
    s.addRoute(route, "DELETE", handler)
}

// Match adds a handler for an arbitrary http method for server s.
func (s *Server) Match(method string, route string, handler interface{}) {
    s.addRoute(route, method, handler)
}

// Get adds a handler for the 'GET' http method in the main server.
func Get(route string, handler interface{}) {
    mainServer.Get(route, handler)
}

// Post adds a handler for the 'POST' http method in the main server.
func Post(route string, handler interface{}) {
    mainServer.addRoute(route, "POST", handler)
}

// Put adds a handler for the 'PUT' http method in the main server.
func Put(route string, handler interface{}) {
    mainServer.addRoute(route, "PUT", handler)
}

// Delete adds a handler for the 'DELETE' http method in the main server.
func Delete(route string, handler interface{}) {
    mainServer.addRoute(route, "DELETE", handler)
}

// Match adds a handler for an arbitrary http method in the main server.
func Match(method string, route string, handler interface{}) {
    mainServer.addRoute(route, method, handler)
}

// SetLogger sets the logger for server s
func (s *Server) SetLogger(logger *log.Logger) {
    s.Logger = logger
}

// SetLogger sets the logger for the main server.
func SetLogger(logger *log.Logger) {
    mainServer.Logger = logger
}

// Config is the configuration of the main server.
var Config = &ServerConfig{
    RecoverPanic: true,
}

var mainServer = NewServer()

// internal utility methods
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

// Urlencode is a helper method that converts a map into URL-encoded form data.
// It is a useful when constructing HTTP POST requests.
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

var slugRegex = regexp.MustCompile(`(?i:[^a-z0-9\-_])`)

// Slug is a helper function that returns the URL slug for string s.
// It's used to return clean, URL-friendly strings that can be
// used in routing.
func Slug(s string, sep string) string {
    if s == "" {
        return ""
    }
    slug := slugRegex.ReplaceAllString(s, sep)
    if slug == "" {
        return ""
    }
    quoted := regexp.QuoteMeta(sep)
    sepRegex := regexp.MustCompile("(" + quoted + "){2,}")
    slug = sepRegex.ReplaceAllString(slug, sep)
    sepEnds := regexp.MustCompile("^" + quoted + "|" + quoted + "$")
    slug = sepEnds.ReplaceAllString(slug, "")
    return strings.ToLower(slug)
}

// NewCookie is a helper method that returns a new http.Cookie object.
// Duration is specified in seconds. If the duration is zero, the cookie is permanent.
// This can be used in conjunction with ctx.SetCookie.
func NewCookie(name string, value string, age int64) *http.Cookie {
    var utctime time.Time
    if age == 0 {
        // 2^31 - 1 seconds (roughly 2038)
        utctime = time.Unix(2147483647, 0)
    } else {
        utctime = time.Unix(time.Now().Unix()+age, 0)
    }
    return &http.Cookie{Name: name, Value: value, Expires: utctime}
}
