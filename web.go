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
    "strconv"
    "strings"
    "time"
)

//secret key used to store cookies
var secret = ""

type conn interface {
    StartResponse(status int)
    SetHeader(hdr string, val string, unique bool)
    Write(data []byte) (n int, err os.Error)
    Close()
}

type Context struct {
    *Request
    *conn
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
    ctx.Write(strings.Bytes(content))
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

func (ctx *Context) NotFound(message string) {
    ctx.StartResponse(404)
    ctx.WriteString(message)
}

//Sets a cookie -- duration is the amount of time in seconds. 0 = forever
func (ctx *Context) SetCookie(name string, value string, age int64) {
    if age == 0 {
        //do some really long time
    }

    utctime := time.UTC()
    utc1 := time.SecondsToUTC(utctime.Seconds() + 60*30)
    expires := utc1.Format(time.RFC1123)
    expires = expires[0:len(expires)-3] + "GMT"
    cookie := fmt.Sprintf("%s=%s; expires=%s", name, value, expires)
    ctx.SetHeader("Set-Cookie", cookie, false)
}

func SetCookieSecret(key string) { secret = key }

func getCookieSig(val []byte, timestamp string) string {
    hm := hmac.NewSHA1(strings.Bytes(secret))

    hm.Write(val)
    hm.Write(strings.Bytes(timestamp))

    hex := fmt.Sprintf("%02x", hm.Sum())
    return hex
}

func (ctx *Context) SetSecureCookie(name string, val string, age int64) {
    //base64 encode the val
    if len(secret) == 0 {
        log.Stderrf("Secret Key for secure cookies has not been set. Please call web.SetCookieSecret\n")
        return
    }
    var buf bytes.Buffer
    encoder := base64.NewEncoder(base64.StdEncoding, &buf)
    encoder.Write(strings.Bytes(val))
    encoder.Close()
    vs := buf.String()
    vb := buf.Bytes()

    timestamp := strconv.Itoa64(time.Seconds())

    sig := getCookieSig(vb, timestamp)

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

    if getCookieSig(strings.Bytes(val), timestamp) != sig {
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

var contextType reflect.Type
var staticDir string

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
        log.Stdout(req.Method + " " + requestPath)
    } else {
        log.Stdout(requestPath + "?" + req.URL.RawQuery)
    }

    //parse the form data (if it exists)
    perr := req.parseParams()
    if perr != nil {
        log.Stderrf("Failed to parse form data %q", perr.String())
    }

    //parse the cookies
    perr = req.parseCookies()
    if perr != nil {
        log.Stderrf("Failed to parse cookies %q", perr.String())
    }

    ctx := Context{req, &c, false}

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
        //if the methods don't match, skip this handler (except HEAD can be used in place of GET)
        if req.Method != route.method && !(req.Method == "HEAD" && route.method == "GET") {
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

        if args.Len() != handlerType.NumIn() {
            log.Stderrf("Incorrect number of arguments for %s\n", requestPath)
            ctx.Abort(500, "Server Error")
            return
        }

        valArgs := make([]reflect.Value, args.Len())
        for i := 0; i < args.Len(); i++ {
            valArgs[i] = args.At(i).(reflect.Value)
        }

        ret := route.handler.Call(valArgs)

        if len(ret) == 0 {
            return
        }

        sval, ok := ret[0].(*reflect.StringValue)

        if ok && !ctx.responseStarted {
            outbytes := strings.Bytes(sval.Get())
            ctx.SetHeader("Content-Length", strconv.Itoa(len(outbytes)), true)
            ctx.StartResponse(200)
            ctx.Write(outbytes)
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

func Urlencode(data map[string]string) string {
    var buf bytes.Buffer
    for k, v := range (data) {
        buf.WriteString(http.URLEscape(k))
        buf.WriteByte('=')
        buf.WriteString(http.URLEscape(v))
        buf.WriteByte('&')
    }
    s := buf.String()
    return s[0 : len(s)-1]
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
