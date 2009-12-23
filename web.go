package web

import (
    "bytes"
    "http"
    "io"
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

type Response struct {
    Status     string
    StatusCode int
    Header     map[string]string
    Body       io.Reader
}

func newResponse(statusCode int, body string) *Response {
    text := statusText[statusCode]
    resp := Response{StatusCode: statusCode,
        Status: text,
        Header: map[string]string{"Content-Type": "text/plain; charset=utf-8"},
    }
    if len(body) == 0 {
        resp.Body = bytes.NewBufferString(text)
    } else {
        resp.Body = bytes.NewBufferString(body)
    }
    return &resp
}

type Context struct {
    *Request
    *Response
}

func (ctx *Context) Abort(code int, message string) {
    ctx.Response = newResponse(code, message)
}

var contextType reflect.Type
var templateDir string
var staticDir string
var staticRoute *regexp.Regexp

func init() {
    contextType = reflect.Typeof(Context{})

    SetTemplateDir("templates")
    SetStaticRoute("/static/(.*)")
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

func httpHandler(c *http.Conn, req *http.Request) {
    requestPath := req.URL.Path

    //try to serve a static file
    if staticRoute.MatchString(requestPath) {
        match := staticRoute.MatchStrings(requestPath)
	if len(match) > 0 {
            staticFile := match[1]
            if len(staticFile) > 0 {
                http.ServeFile(c, req, path.Join(staticDir, staticFile))
                return
            }
	}
    }

    req.ParseForm()
    resp := routeHandler((*Request)(req))
    c.WriteHeader(resp.StatusCode)
    if resp.Body != nil {
        body, _ := ioutil.ReadAll(resp.Body)
        c.Write(body)
    }
}

func routeHandler(req *Request) *Response {
    log.Stdout(req.RawURL)
    requestPath := req.URL.Path

    ctx := Context{req, newResponse(200, "")}

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
                return newResponse(500, "")
            }

            for _, arg := range match[1:] {
                args[ai] = reflect.NewValue(arg)
            }
            ret := route.handler.Call(args)[0].(*reflect.StringValue).Get()
            var buf bytes.Buffer
            buf.WriteString(ret)
            resp := ctx.Response
            resp.Body = &buf
            return resp
        }
    }

    return newResponse(404, "")
}

func render(tmplString string, context interface{}) (string, os.Error) {

    var tmpl *template.Template
    var err os.Error

    if tmpl, err = template.Parse(tmplString, nil); err != nil {
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

func Get(route string, handler interface{}) { addRoute(route, "GET", handler) }

func Post(route string, handler interface{}) { addRoute(route, "POST", handler) }

func Head(route string, handler interface{}) { addRoute(route, "HEAD", handler) }

func Put(route string, handler interface{}) { addRoute(route, "PUT", handler) }

func Delete(route string, handler interface{}) {
    addRoute(route, "DELETE", handler)
}

func currentDirectory() string {
    return os.Getenv("PWD")
}

func checkDirectory(dir string) {
    d, e := os.Stat(dir)
    switch {
    case e != nil:
        log.Stderr(e)
    case !d.IsDirectory():
        log.Stderrf("%s is not a directory", dir)
    }
}

func SetTemplateDir(dir string) {
    cwd := currentDirectory()
    templateDir = path.Join(cwd, dir)
    checkDirectory(templateDir)
}

func SetStaticDir(dir string) {
    cwd := currentDirectory()
    staticDir = path.Join(cwd, dir)
    checkDirectory(staticDir)
}

func SetStaticRoute(route string) {
    cr, err := regexp.Compile(route)
    if err != nil {
        log.Stderrf("Error in static route regex %q\n", route)
    } else {
        staticRoute = cr
    }
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
