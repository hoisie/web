package web

import (
    "bytes"
    "http"
    "io"
    "io/ioutil"
    "log"
    "os"
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

var servererror = Response{
    Status: "500 Internal Server Error",
    StatusCode: 500,
    Header: map[string]string{"Content-Type": "text/plain; charset=utf-8"},
    Body: bytes.NewBufferString("Internal Server Error"),
}

var notfound = Response{
    Status: "404 Not Found",
    StatusCode: 404,
    Header: map[string]string{"Content-Type": "text/plain; charset=utf-8"},
    Body: bytes.NewBufferString("Page Not Found"),
}

var requestType reflect.Type

func init() {
    var r Request
    requestType = reflect.Typeof(r)
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
    path := req.URL.Path

    //try to serve a static file
    if strings.HasPrefix(path, "/static/") {
        staticFile := path[8:]
        if len(staticFile) > 0 {
            http.ServeFile(c, req, "static/"+staticFile)
            return
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

func routeHandler(req *Request) Response {
    log.Stdout(req.RawURL)
    path := req.URL.Path
    for cr, route := range routes {
        if !cr.MatchString(path) {
            continue
        }
        match := cr.MatchStrings(path)
        if len(match) > 0 {
            if len(match[0]) != len(path) {
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
                    if typ == requestType {
                        args[ai] = reflect.NewValue(req)
                        ai += 1
                        expectedIn -= 1
                    }
                }
            }

            actualIn := len(match) - 1
            if expectedIn != actualIn {
                log.Stderrf("Incorrect number of arguments for %s\n", path)
                return servererror
            }

            for _, arg := range match[1:] {
                args[ai] = reflect.NewValue(arg)
            }
            ret := route.handler.Call(args)[0].(*reflect.StringValue).Get()
            var buf bytes.Buffer
            buf.WriteString(ret)

            return Response{Status: "200 OK",
                StatusCode: 200,
                Header: make(map[string]string),
                Body: &buf,
            }
        }
    }

    return notfound
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


func RenderFile(filename string, context interface{}) (string, os.Error) {
    var templateBytes []uint8
    var err os.Error

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

    err := http.ListenAndServe(addr, nil)
    if err != nil {
        log.Exit("ListenAndServe:", err)
    }
}

func RunScgi(addr string) { listenAndServeScgi(addr) }

func Get(route string, handler interface{}) { addRoute(route, "GET", handler) }

func Post(route string, handler interface{}) { addRoute(route, "POST", handler) }

func Head(route string, handler interface{}) { addRoute(route, "HEAD", handler) }

func Put(route string, handler interface{}) { addRoute(route, "PUT", handler) }

func Delete(route string, handler interface{}) {
    addRoute(route, "DELETE", handler)
}
