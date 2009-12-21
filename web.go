package web

import (
    "bytes"
    "fmt"
    "http"
    "io/ioutil"
    "log"
    "os"
    "reflect"
    "regexp"
    "strings"
    "template"
)

type Request http.Request


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
        fmt.Printf("Error in route regex %q\n", r)
        return
    }
    fv := reflect.NewValue(handler).(*reflect.FuncValue)
    routes[cr] = route{r, cr, method, fv}
}

func routeHandler(c *http.Conn, req *http.Request) {
    println(req.RawURL)
    //try to serve a static file
    var path string = req.URL.Path

    if strings.HasPrefix(path, "/static/") {
        staticFile := path[8:]
        if len(staticFile) > 0 {
            http.ServeFile(c, req, "static/"+staticFile)
            return
        }
    }

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
                    if typ.PkgPath() == "web" && typ.Name() == "Request" {
                        req.ParseForm()
                        wr := (*Request)(req)
                        args[ai] = reflect.NewValue(wr)
                        ai += 1
                        expectedIn -= 1
                    }
                }
            }

            actualIn := len(match) - 1
            if expectedIn != actualIn {
                fmt.Printf("%s - Incorrect number of arguments\n", path)
                return
            }

            for _, arg := range match[1:] {
                args[ai] = reflect.NewValue(arg)
            }
            ret := route.handler.Call(args)[0].(*reflect.StringValue).Get()
            var buf bytes.Buffer
            buf.WriteString(ret)
            c.Write(buf.Bytes())
            return
        }
    }
    
    // return a 404
    http.NotFound(c, req)
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
    http.Handle("/", http.HandlerFunc(routeHandler))

    err := http.ListenAndServe(addr, nil)
    if err != nil {
        log.Exit("ListenAndServe:", err)
    }
}

func Get(route string, handler interface{}) { addRoute(route, "GET", handler) }

func Post(route string, handler interface{}) { addRoute(route, "POST", handler) }

func Head(route string, handler interface{}) { addRoute(route, "HEAD", handler) }

func Put(route string, handler interface{}) { addRoute(route, "PUT", handler) }

func Delete(route string, handler interface{}) {
    addRoute(route, "DELETE", handler)
}
