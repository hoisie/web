package web

import (
    "bytes"
    "container/vector"
    "crypto/hmac"
    "encoding/base64"
    "fmt"
    "http"
    "io/ioutil"
    "log"
    "os"
    "path"
    "reflect"
    "regexp"
    "runtime"
    "strconv"
    "strings"
    "time"
)

type conn interface {
    StartResponse(status int)
    SetHeader(hdr string, val string, unique bool)
    Write(data []byte) (n int, err os.Error)
    Close()
}

type Context struct {
    *Request
    *Server
    conn
    responseStarted bool
}

func (ctx *Context) StartResponse(status int) {
    ctx.conn.StartResponse(status)
    ctx.responseStarted = true
}

func (ctx *Context) Write(data []byte) (n int, err os.Error) {
    if !ctx.responseStarted {
        ctx.StartResponse(200)
    }

    //if it's a HEAD request, we just write blank data
    if ctx.Request.Method == "HEAD" {
        data = []byte{}
    }

    return ctx.conn.Write(data)
}
func (ctx *Context) WriteString(content string) {
    ctx.Write([]byte(content))
}

func (ctx *Context) Abort(status int, body string) {
    ctx.StartResponse(status)
    ctx.WriteString(body)
}

func (ctx *Context) Redirect(status int, url string) {
    ctx.SetHeader("Location", url, true)
    ctx.StartResponse(status)
    ctx.WriteString("Redirecting to: " + url)
}

func (ctx *Context) NotModified() {
    ctx.StartResponse(304)
}

func (ctx *Context) NotFound(message string) {
    ctx.StartResponse(404)
    ctx.WriteString(message)
}

//Sets a cookie -- duration is the amount of time in seconds. 0 = forever
func (ctx *Context) SetCookie(name string, value string, age int64) {
	var utctime *time.Time
	if age == 0 {
		// 2^31 - 1 seconds (roughly 2038)
		utctime = time.SecondsToUTC(2147483647)
	} else {
		utctime = time.SecondsToUTC(time.UTC().Seconds() + age)
	}
	cookie := fmt.Sprintf("%s=%s; expires=%s", name, value, webTime(utctime))
	ctx.SetHeader("Set-Cookie", cookie, false)
}

func getCookieSig(key string, val []byte, timestamp string) string {
    hm := hmac.NewSHA1([]byte(key))

    hm.Write(val)
    hm.Write([]byte(timestamp))

    hex := fmt.Sprintf("%02x", hm.Sum())
    return hex
}

func (ctx *Context) SetSecureCookie(name string, val string, age int64) {
    //base64 encode the val
    if len(ctx.Server.Config.CookieSecret) == 0 {
        ctx.Logger.Println("Secret Key for secure cookies has not been set. Please call web.SetCookieSecret")
        return
    }
    var buf bytes.Buffer
    encoder := base64.NewEncoder(base64.StdEncoding, &buf)
    encoder.Write([]byte(val))
    encoder.Close()
    vs := buf.String()
    vb := buf.Bytes()
    timestamp := strconv.Itoa64(time.Seconds())
    sig := getCookieSig(ctx.Server.Config.CookieSecret, vb, timestamp)
    cookie := strings.Join([]string{vs, timestamp, sig}, "|")
    ctx.SetCookie(name, cookie, age)
}

func (ctx *Context) GetSecureCookie(name string) (string, bool) {
    cookie, ok := ctx.Request.Cookies[name]
    if !ok {
        return "", false
    }

    parts := strings.Split(cookie, "|", 3)

    val := parts[0]
    timestamp := parts[1]
    sig := parts[2]

    if getCookieSig(ctx.Server.Config.CookieSecret, []byte(val), timestamp) != sig {
        return "", false
    }

    ts, _ := strconv.Atoi64(timestamp)

    if time.Seconds()-31*86400 > ts {
        return "", false
    }

    buf := bytes.NewBufferString(val)
    encoder := base64.NewDecoder(base64.StdEncoding, buf)

    res, _ := ioutil.ReadAll(encoder)
    return string(res), true
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
    contextType = reflect.Typeof(Context{})
    //find the location of the exe file
    arg0 := path.Clean(os.Args[0])
    wd, _ := os.Getwd()
    if strings.HasPrefix(arg0, "/") {
        exeFile = arg0
    } else {
        //TODO for robustness, search each directory in $PATH
        exeFile = path.Join(wd, arg0)
    }
}

type route struct {
    r       string
    cr      *regexp.Regexp
    method  string
    handler *reflect.FuncValue
}

func (s *Server) addRoute(r string, method string, handler interface{}) {
    cr, err := regexp.Compile(r)
    if err != nil {
        s.Logger.Printf("Error in route regex %q\n", r)
        return
    }
    fv := reflect.NewValue(handler).(*reflect.FuncValue)
    s.routes.Push(route{r, cr, method, fv})
}

type httpConn struct {
    conn http.ResponseWriter
}

func (c *httpConn) StartResponse(status int) { c.conn.WriteHeader(status) }

func (c *httpConn) SetHeader(hdr string, val string, unique bool) {
    //right now unique can't be implemented through the http package.
    //see issue 488
    c.conn.SetHeader(hdr, val)
}

func (c *httpConn) WriteString(content string) {
    buf := bytes.NewBufferString(content)
    c.conn.Write(buf.Bytes())
}

func (c *httpConn) Write(content []byte) (n int, err os.Error) {
    return c.conn.Write(content)
}

func (c *httpConn) Close() {
    rwc, buf, _ := c.conn.Hijack()
    if buf != nil {
        buf.Flush()
    }

    if rwc != nil {
        rwc.Close()
    }
}

func (s *Server) ServeHTTP(c http.ResponseWriter, req *http.Request) {
    conn := httpConn{c}
    wreq := newRequest(req, c)
    s.routeHandler(wreq, &conn)
}

func (s *Server) safelyCall(function *reflect.FuncValue, args []reflect.Value) (resp []reflect.Value, e interface{}) {
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

func (s *Server) routeHandler(req *Request, c conn) {
    requestPath := req.URL.Path

    //log the request
    if len(req.URL.RawQuery) == 0 {
        s.Logger.Println(req.Method + " " + requestPath)
    } else {
        s.Logger.Println(req.Method + " " + requestPath + "?" + req.URL.RawQuery)
    }

    //parse the form data (if it exists)
    perr := req.parseParams()
    if perr != nil {
        s.Logger.Printf("Failed to parse form data %q\n", perr.String())
    }

    //parse the cookies
    perr = req.parseCookies()
    if perr != nil {
        s.Logger.Printf("Failed to parse cookies %q", perr.String())
    }

    ctx := Context{req, s, c, false}

    //set some default headers
    ctx.SetHeader("Content-Type", "text/html; charset=utf-8", true)
    ctx.SetHeader("Server", "web.go", true)

    tm := time.UTC()
    ctx.SetHeader("Date", webTime(tm), true)

    //try to serve a static file
    staticDir := s.Config.StaticDir
    if staticDir == "" {
        staticDir = defaultStaticDir()
    }
    staticFile := path.Join(staticDir, requestPath)
    if fileExists(staticFile) && (req.Method == "GET" || req.Method == "HEAD") {
        serveFile(&ctx, staticFile)
        return
    }

    for i := 0; i < s.routes.Len(); i++ {
        route := s.routes.At(i).(route)
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

        var args vector.Vector

        handlerType := route.handler.Type().(*reflect.FuncType)

        //check if the first arg in the handler is a context type
        if handlerType.NumIn() > 0 {
            if a0, ok := handlerType.In(0).(*reflect.PtrType); ok {
                typ := a0.Elem()
                if typ == contextType {
                    args.Push(reflect.NewValue(&ctx))
                }
            }
        }

        for _, arg := range match[1:] {
            args.Push(reflect.NewValue(arg))
        }

        if args.Len() != handlerType.NumIn() {
            s.Logger.Printf("Incorrect number of arguments for %s\n", requestPath)
            ctx.Abort(500, "Server Error")
            return
        }

        valArgs := make([]reflect.Value, args.Len())
        for i := 0; i < args.Len(); i++ {
            valArgs[i] = args.At(i).(reflect.Value)
        }

        ret, err := s.safelyCall(route.handler, valArgs)
        if err != nil {
            //there was a panic in the handler
            ctx.Abort(500, "Server Error")
        }

        if len(ret) == 0 {
            return
        }

        sval, ok := ret[0].(*reflect.StringValue)

        if ok && !ctx.responseStarted {
            content := []byte(sval.Get())
            ctx.SetHeader("Content-Length", strconv.Itoa(len(content)), true)
            ctx.StartResponse(200)
            ctx.Write(content)
        }

        return
    }

    //try to serve index.html || index.htm
    if indexPath := path.Join(path.Join(staticDir, requestPath), "index.html"); fileExists(indexPath) {
        serveFile(&ctx, indexPath)
        return
    }

    if indexPath := path.Join(path.Join(staticDir, requestPath), "index.htm"); fileExists(indexPath) {
        serveFile(&ctx, indexPath)
        return
    }

    ctx.Abort(404, "Page not found")
}

var Config = &ServerConfig{
    RecoverPanic: true,
}

var mainServer = Server{
    Config: Config,
    Logger: log.New(os.Stdout, "", log.Ldate|log.Ltime),
}

type Server struct {
    Config *ServerConfig
    routes vector.Vector
    Logger *log.Logger
}

func (s *Server) initServer() {
    if s.Config == nil {
        s.Config = &ServerConfig{}
    }

    if s.Logger == nil {
        s.Logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)
    }
}

func (s *Server) Run(addr string) {
    s.initServer()

    mux := http.NewServeMux()
    mux.Handle("/", s)
    s.Logger.Printf("web.go serving %s\n", addr)
    err := http.ListenAndServe(addr, mux)
    if err != nil {
        log.Exit("ListenAndServe:", err)
    }
}

//runs the web application and serves http requests
func Run(addr string) {
    mainServer.Run(addr)
}

func (s *Server) RunScgi(addr string) {
    s.initServer()
    s.Logger.Printf("web.go serving scgi %s\n", addr)
    s.listenAndServeScgi(addr)
}

//runs the web application and serves scgi requests
func RunScgi(addr string) {
    mainServer.RunScgi(addr)
}

func (s *Server) RunFcgi(addr string) {
    s.initServer()
    s.Logger.Printf("web.go serving fcgi %s\n", addr)
    s.listenAndServeFcgi(addr)
}

//runs the web application by serving fastcgi requests
func RunFcgi(addr string) {
    mainServer.RunFcgi(addr)
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

//Adds a handler for the 'GET' http method.
func Get(route string, handler interface{}) {
    mainServer.Get(route, handler)
    //addRoute(route, "GET", handler) 
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

func webTime(t *time.Time) string {
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
    case !d.IsDirectory():
        return false
    }

    return true
}

func fileExists(dir string) bool {
    info, err := os.Stat(dir)
    if err != nil {
        return false
    } else if !info.IsRegular() {
        return false
    }

    return true
}

func Urlencode(data map[string]string) string {
    var buf bytes.Buffer
    for k, v := range data {
        buf.WriteString(http.URLEscape(k))
        buf.WriteByte('=')
        buf.WriteString(http.URLEscape(v))
        buf.WriteByte('&')
    }
    s := buf.String()
    return s[0 : len(s)-1]
}
