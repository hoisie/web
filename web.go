package web

import (
    "bytes"
    "container/vector"
    "fmt"
    "http"
    "log"
    "os"
    "path"
    "rand"
    "reflect"
    "regexp"
    "strings"
    "time"
)

var rgen = rand.New(rand.NewSource(time.Nanoseconds()))

type conn interface {
    StartResponse(status int)
    SetHeader(hdr string, val string, unique bool)
    Write(data []byte) (n int, err os.Error)
    Close()
}

type Context struct {
    *Request
    *conn
    Session         *session
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
    return ctx.conn.Write(data)
}
func (ctx *Context) WriteString(content string) {
    ctx.Write(strings.Bytes(content))
}

func (ctx *Context) Abort(status int, body string) {
    ctx.StartResponse(status)
    ctx.WriteString(body)
}

func (ctx *Context) Redirect(status int, url string) {
    //note := "<a href=\"%v\">" + statusText[code] + "</a>.\n"

    ctx.SetHeader("Location", url, true)
    ctx.StartResponse(status)
    ctx.WriteString("")
}
//Sets a cookie -- duration is the amount of time in seconds. 0 = forever
func (ctx *Context) SetCookie(name string, value string, duration int64) {
    if duration == 0 {
        //do some really long time
    }

    utctime := time.UTC()
    utc1 := time.SecondsToUTC(utctime.Seconds() + 60*30)
    expires := utc1.RFC1123()
    expires = expires[0:len(expires)-3] + "GMT"
    cookie := fmt.Sprintf("%s=%s; expires=%s", name, value, expires)
    ctx.SetHeader("Set-Cookie", cookie, false)
}

var sessionMap = make(map[string]*session)

func randomString(length int) string {
    pop := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
    var res bytes.Buffer

    for i := 0; i < length; i++ {
        rnd := rgen.Intn(len(pop))
        res.WriteByte(pop[rnd])
    }

    return res.String()
}

type session struct {
    Data map[string]interface{}
    Id   string
}

func newSession() *session {
    s := session{
        Data: make(map[string]interface{}),
        Id: randomString(10),
    }

    return &s
}

func (s *session) save() { sessionMap[s.Id] = s }

var contextType reflect.Type
var staticDir string

const sessionKey = "wgosession"

func init() {
    contextType = reflect.Typeof(Context{})
    SetStaticDir("static")
}

type route struct {
    r       string
    cr      *regexp.Regexp
    method  string
    handler *reflect.FuncValue
}

var routes = make(map[*regexp.Regexp]route)

func addRoute(r string, method string, handler interface{}) {
    cr, err := regexp.Compile(r)
    if err != nil {
        log.Stderrf("Error in route regex %q\n", r)
        return
    }
    fv := reflect.NewValue(handler).(*reflect.FuncValue)
    routes[cr] = route{r, cr, method, fv}
}

type httpConn struct {
    conn *http.Conn
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

func httpHandler(c *http.Conn, req *http.Request) {
    conn := httpConn{c}

    wreq := newRequest(req)

    routeHandler(wreq, &conn)
}

func routeHandler(req *Request, c conn) {
    requestPath := req.URL.Path

    //log the request
    if len(req.URL.RawQuery) == 0 {
        log.Stdout(requestPath)
    } else {
        log.Stdout(requestPath + "?" + req.URL.RawQuery)
    }

    //parse the form data (if it exists)
    perr := req.ParseParams()
    if perr != nil {
        log.Stderrf("Failed to parse form data %q", perr.String())
    }

    //check the cookies for a session id
    perr = req.ParseCookies()
    if perr != nil {
        log.Stderrf("Failed to parse cookies %q", perr.String())
    }

    s := newSession()

    for k, v := range (req.Cookies) {
        if k == sessionKey {
            if sess, ok := sessionMap[v]; ok {
                s = sess
            }
        }
    }

    ctx := Context{req, &c, s, false}

    //try to serve a static file
    staticFile := path.Join(staticDir, requestPath)
    if fileExists(staticFile) {
        serveFile(&ctx, staticFile)
        return
    }

    //set default encoding
    ctx.SetHeader("Content-Type", "text/html; charset=utf-8", true)
    ctx.SetHeader("Server", "web.go", true)

    for cr, route := range routes {
        if req.Method != route.method {
            continue
        }

        if !cr.MatchString(requestPath) {
            continue
        }
        match := cr.MatchStrings(requestPath)

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

        if len(args) != handlerType.NumIn() {
            log.Stderrf("Incorrect number of arguments for %s\n", requestPath)
            ctx.Abort(500, "Server Error")
            return
        }

        valArgs := make([]reflect.Value, len(args))
        for i, j := range (args) {
            valArgs[i] = j.(reflect.Value)
        }

        ret := route.handler.Call(valArgs)

        if len(ret) == 0 {
            return
        }

        sval, ok := ret[0].(*reflect.StringValue)

        if ok && !ctx.responseStarted {
            //check if session data is stored
            if len(s.Data) > 0 {
                s.save()
                //set the session for half an hour
                ctx.SetCookie(sessionKey, s.Id, 1800)
            }

            ctx.StartResponse(200)
            ctx.WriteString(sval.Get())
        }

        return
    }

    ctx.Abort(404, "Page not found")
}

//runs the web application and serves http requests
func Run(addr string) {
    http.Handle("/", http.HandlerFunc(httpHandler))

    log.Stdoutf("web.go serving %s", addr)
    err := http.ListenAndServe(addr, nil)
    if err != nil {
        log.Exit("ListenAndServe:", err)
    }
}

//runs the web application and serves scgi requests
func RunScgi(addr string) {
    log.Stdoutf("web.go serving scgi %s", addr)
    listenAndServeScgi(addr)
}

//runs the web application by serving fastcgi requests
func RunFcgi(addr string) {
    log.Stdoutf("web.go serving fcgi %s", addr)
    listenAndServeFcgi(addr)
}

//Adds a handler for the 'GET' http method.
func Get(route string, handler interface{}) { addRoute(route, "GET", handler) }

//Adds a handler for the 'POST' http method.
func Post(route string, handler interface{}) { addRoute(route, "POST", handler) }

//Adds a handler for the 'PUT' http method.
func Put(route string, handler interface{}) { addRoute(route, "PUT", handler) }

//Adds a handler for the 'DELETE' http method.
func Delete(route string, handler interface{}) {
    addRoute(route, "DELETE", handler)
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
    d, e := os.Stat(dir)
    switch {
    case e != nil:
        return false
    case !d.IsRegular():
        return false
    }

    return true
}

type dirError string

func (path dirError) String() string { return "Failed to set directory " + string(path) }

func getCwd() string { return os.Getenv("PWD") }

//changes the location of the static directory. by default, it's under the 'static' folder
//of the directory containing the web application
func SetStaticDir(dir string) os.Error {
    cwd := getCwd()
    sd := path.Join(cwd, dir)
    if !dirExists(sd) {
        return dirError(sd)

    }
    staticDir = sd

    return nil
}

func Urlencode ( data map[string]string ) string {
        var buf bytes.Buffer;
        for k,v := range ( data ) {
                buf.WriteString ( http.URLEscape(k) )
                buf.WriteByte('=' )
                buf.WriteString ( http.URLEscape(v) )
                buf.WriteByte('&' )
        }
        s := buf.String()
        return s[0:len(s) - 1]
}

//copied from go's http package, because it's not public
var statusText = map[int]string{
    http.StatusContinue: "Continue",
    http.StatusSwitchingProtocols: "Switching Protocols",

    http.StatusOK: "OK",
    http.StatusCreated: "Created",
    http.StatusAccepted: "Accepted",
    http.StatusNonAuthoritativeInfo: "Non-Authoritative Information",
    http.StatusNoContent: "No Content",
    http.StatusResetContent: "Reset Content",
    http.StatusPartialContent: "Partial Content",

    http.StatusMultipleChoices: "Multiple Choices",
    http.StatusMovedPermanently: "Moved Permanently",
    http.StatusFound: "Found",
    http.StatusSeeOther: "See Other",
    http.StatusNotModified: "Not Modified",
    http.StatusUseProxy: "Use Proxy",
    http.StatusTemporaryRedirect: "Temporary Redirect",

    http.StatusBadRequest: "Bad Request",
    http.StatusUnauthorized: "Unauthorized",
    http.StatusPaymentRequired: "Payment Required",
    http.StatusForbidden: "Forbidden",
    http.StatusNotFound: "Not Found",
    http.StatusMethodNotAllowed: "Method Not Allowed",
    http.StatusNotAcceptable: "Not Acceptable",
    http.StatusProxyAuthRequired: "Proxy Authentication Required",
    http.StatusRequestTimeout: "Request Timeout",
    http.StatusConflict: "Conflict",
    http.StatusGone: "Gone",
    http.StatusLengthRequired: "Length Required",
    http.StatusPreconditionFailed: "Precondition Failed",
    http.StatusRequestEntityTooLarge: "Request Entity Too Large",
    http.StatusRequestURITooLong: "Request URI Too Long",
    http.StatusUnsupportedMediaType: "Unsupported Media Type",
    http.StatusRequestedRangeNotSatisfiable: "Requested Range Not Satisfiable",
    http.StatusExpectationFailed: "Expectation Failed",

    http.StatusInternalServerError: "Internal Server Error",
    http.StatusNotImplemented: "Not Implemented",
    http.StatusBadGateway: "Bad Gateway",
    http.StatusServiceUnavailable: "Service Unavailable",
    http.StatusGatewayTimeout: "Gateway Timeout",
    http.StatusHTTPVersionNotSupported: "HTTP Version Not Supported",
}
