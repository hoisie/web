package web

import (
	"bytes";
	"fmt";
	"http";
	"io/ioutil";
	"log";
	"os";
	"reflect";
	"regexp";
	"strings";
	"template";
)

var compiledRoutes = map[*regexp.Regexp]*reflect.FuncValue{}

func compileRoutes(urls map[string]interface{}) {
	for r, f := range urls {
		regex, err := regexp.Compile(r);
		if err != nil {
			println("Error in route")
		}
		fv := reflect.NewValue(f).(*reflect.FuncValue);
		compiledRoutes[regex] = fv;
	}
}

func routeHandler(c *http.Conn, req *http.Request) {
	println(req.RawURL);

	//try to serve a static file
	if strings.HasPrefix(req.RawURL, "/static/") {
		staticFile := req.RawURL[8:];
		if len(staticFile) > 0 {
			http.ServeFile(c, req, "static/"+staticFile)
		}
	}

	var route string = req.RawURL;
	for r, fv := range compiledRoutes {
		if !r.MatchString(route) {
			continue
		}
		match := r.MatchStrings(route);
		if len(match) > 0 {
			if len(match[0]) != len(route) {
				continue
			}
			args := make([]reflect.Value, len(match)-1);

			expectedIn := fv.Type().(*reflect.FuncType).NumIn();
			actualIn := len(match) - 1;

			if expectedIn != actualIn {
				message := fmt.Sprintf("%s - Incorrect number of arguments", req.RawURL);
				println(message);
				return;
			}

			for i, arg := range match[1:] {
				args[i] = reflect.NewValue(arg)
			}
			ret := fv.Call(args)[0].(*reflect.StringValue).Get();
			var buf bytes.Buffer;
			buf.WriteString(ret);
			c.Write(buf.Bytes());
		}
	}

}

func Render(filename string, context interface{}) (string, os.Error) {
	var templateBytes []uint8;
	var err os.Error;

	if templateBytes, err = ioutil.ReadFile(filename); err != nil {
		return "", err
	}

	var templ *template.Template;
	if templ, err = template.Parse(string(templateBytes), nil); err != nil {
		return "", err
	}

	var buf bytes.Buffer;

	templ.Execute(context, &buf);
	return buf.String(), nil;
}

func Run(urls map[string]interface{}, addr string) {
	compileRoutes(urls);
	http.Handle("/", http.HandlerFunc(routeHandler));

	err := http.ListenAndServe(addr, nil);
	if err != nil {
		log.Exit("ListenAndServe:", err)
	}
}
