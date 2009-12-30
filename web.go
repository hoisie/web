package web

import (
    "bytes"
    "http"
    "io/ioutil"
    "log"
    "os"
    "path"
    "reflect"
    "regexp"
    "strings"
    "template"
)

type Request http.Request

func (r *Request) ParseForm() (err os.Error) {
    req := (*http.Request)(r)
    return req.ParseForm()
}

type Conn interface {
    StartResponse(status int)
    SetHeader(hdr string, val string)
    Write(data []byte) (n int, err os.Error)
    WriteString(content string)
    Close()
}

type Context struct {
    *Request
    Conn
}

func (ctx *Context) Error(status int, body string) {
    //send an error
}

var contextType reflect.Type
var templateDir string
var staticDir string

func init() {
    contextType = reflect.Typeof(Context{})
    SetTemplateDir("templates")
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

func (c *httpConn) SetHeader(hdr string, val string) {
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
    routeHandler((*Request)(req), &conn)
}

func error(conn Conn, code int, body string) {
    conn.StartResponse(code)
    conn.WriteString(body)
}

func routeHandler(req *Request, conn Conn) {
    requestPath := req.URL.Path

    //log the request
    if len(req.URL.RawQuery) == 0 {
        log.Stdout(requestPath)
    } else {
        log.Stdout(requestPath + "?" + req.URL.RawQuery)
    }

    //parse the form data (if it exists)
    perr := req.ParseForm()
    if perr != nil {
        log.Stderrf("Failed to parse form data %q", perr.String())
    }

    ctx := Context{req, conn}

    //try to serve a static file
    staticFile := path.Join(staticDir, requestPath)
    if fileExists(staticFile) {
        serveFile(&ctx, staticFile)
        return
    }

    //set default encoding
    conn.SetHeader("Content-Type", "text/html; charset=utf-8")

    for cr, route := range routes {
        if !cr.MatchString(requestPath) {
            continue
        }
        match := cr.MatchStrings(requestPath)
        if len(match) > 0 {
            if len(match[0]) != len(requestPath) {
                continue
            }
            if req.Method != route.method {
                continue
            }
            ai := 0
            handlerType := route.handler.Type().(*reflect.FuncType)
            expectedIn := handlerType.NumIn()
            args := make([]reflect.Value, expectedIn)

            if expectedIn > 0 {
                a0 := handlerType.In(0)
                ptyp, ok := a0.(*reflect.PtrType)
                if ok {
                    typ := ptyp.Elem()
                    if typ == contextType {
                        args[ai] = reflect.NewValue(&ctx)
                        ai += 1
                        expectedIn -= 1
                    }
                }
            }

            actualIn := len(match) - 1
            if expectedIn != actualIn {
                log.Stderrf("Incorrect number of arguments for %s\n", requestPath)
                error(conn, 500, "Server Error")
                return
            }

            for _, arg := range match[1:] {
                args[ai] = reflect.NewValue(arg)
            }
            ret := route.handler.Call(args)[0].(*reflect.StringValue).Get()
            conn.StartResponse(200)
            conn.WriteString(ret)
            return
        }
    }

    error(conn, 404, "Page not found")
}

func render(tmplString string, context interface{}) (string, os.Error) {

    tmpl := template.New(nil)
    tmpl.SetDelims("{{","}}")

    if err := tmpl.Parse(tmplString); err != nil {
        return "", err
    }

    var buf bytes.Buffer

    tmpl.Execute(context, &buf)
    return buf.String(), nil
}


func Render(filename string, context interface{}) (string, os.Error) {
    var templateBytes []uint8
    var err os.Error

    if !strings.HasPrefix(filename, "/") {
        filename = path.Join(templateDir, filename)
    }

    if templateBytes, err = ioutil.ReadFile(filename); err != nil {
        return "", err
    }

    return render(string(templateBytes), context)
}

func RenderString(tmplString string, context interface{}) (string, os.Error) {
    return render(tmplString, context)
}

func Run(addr string) {
    http.Handle("/", http.HandlerFunc(httpHandler))

    log.Stdoutf("web.go serving %s", addr)
    err := http.ListenAndServe(addr, nil)
    if err != nil {
        log.Exit("ListenAndServe:", err)
    }
}

func RunScgi(addr string) {
    log.Stdoutf("web.go serving scgi %s", addr)
    listenAndServeScgi(addr)
}

func RunFcgi(addr string) {
    log.Stdoutf("web.go serving fcgi %s", addr)
    listenAndServeFcgi(addr)
}

func Get(route string, handler interface{}) { addRoute(route, "GET", handler) }

func Post(route string, handler interface{}) { addRoute(route, "POST", handler) }

func Head(route string, handler interface{}) { addRoute(route, "HEAD", handler) }

func Put(route string, handler interface{}) { addRoute(route, "PUT", handler) }

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

func SetStaticDir(dir string) os.Error {
    cwd := getCwd()
    sd := path.Join(cwd, dir)
    if !dirExists(sd) {
        return dirError(sd)

    }
    staticDir = sd

    return nil
}

func SetTemplateDir(dir string) os.Error {
    cwd := getCwd()
    td := path.Join(cwd, dir)
    if !dirExists(td) {
        return dirError(td)
    }
    templateDir = td
    return nil
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
