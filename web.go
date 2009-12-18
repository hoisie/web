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

var compiledRoutes = map[*regexp.Regexp]*reflect.FuncValue{}

func compileRoutes(urls map[string]interface{}) {
    for r, f := range urls {
        regex, err := regexp.Compile(r)
        if err != nil {
            println("Error in route")
        }
        fv := reflect.NewValue(f).(*reflect.FuncValue)
        compiledRoutes[regex] = fv
    }
}

func routeHandler(c *http.Conn, req *http.Request) {
    println(req.RawURL)
    //try to serve a static file
    if strings.HasPrefix(req.RawURL, "/static/") {
        staticFile := req.RawURL[8:]
        if len(staticFile) > 0 {
            http.ServeFile(c, req, "static/"+staticFile)
        }
    }

    var route string = req.URL.Path
    for r, fv := range compiledRoutes {
        if !r.MatchString(route) {
            continue
        }
        match := r.MatchStrings(route)
        if len(match) > 0 {
            if len(match[0]) != len(route) {
                continue
            }
            ai := 0
            routeHandler := fv.Type().(*reflect.FuncType)
            expectedIn := routeHandler.NumIn()
            args := make([]reflect.Value, expectedIn)

            if expectedIn > 0 {
                a0 := routeHandler.In(0)
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
                message := fmt.Sprintf("%s - Incorrect number of arguments", req.RawURL)
                println(message)
                return
            }

            for _, arg := range match[1:] {
                args[ai] = reflect.NewValue(arg)
            }
            ret := fv.Call(args)[0].(*reflect.StringValue).Get()
            var buf bytes.Buffer
            buf.WriteString(ret)
            c.Write(buf.Bytes())
            return
        }
    }
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

func Run(urls map[string]interface{}, addr string) {
    compileRoutes(urls)
    http.Handle("/", http.HandlerFunc(routeHandler))

    err := http.ListenAndServe(addr, nil)
    if err != nil {
        log.Exit("ListenAndServe:", err)
    }
}
