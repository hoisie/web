package web

import (
  "bytes";
	"http";
	"log";
  "reflect";
  "regexp";
)

var compiledRoutes = map[*regexp.Regexp] *reflect.FuncValue{}

func compileRoutes(urls map[string] interface {}) {
  for r, f := range urls {
    regex,err := regexp.Compile(r);
    if (err != nil) {
      println("Error in route");
    }
    fv := reflect.NewValue(f).(*reflect.FuncValue);
    compiledRoutes[regex]=fv;
  }
}

// hello world, the web server
func routeHandler(c *http.Conn, req *http.Request) {
  println(req.RawURL);
  var route string = req.RawURL;
  for r, fv := range compiledRoutes {
    if !r.MatchString(route) {
      continue;
    }
    match := r.MatchStrings(route);
    if len(match) > 0 {
      if len(match[0]) != len(route) {
        continue;
      }
      args := make([]reflect.Value, len(match)-1);
      for i, arg := range match[1:] {
        args[i] = reflect.NewValue(arg);
      }
      ret := fv.Call(args)[0].(*reflect.StringValue).Get();//.(*reflect.StringValue).Get();
      var buf bytes.Buffer;
      buf.WriteString(ret);
      c.Write(buf.Bytes());
    }
  }
  
}

func Run(urls map[string] interface {}, addr string) {
  compileRoutes(urls);
	http.Handle("/", http.HandlerFunc(routeHandler));
  
	err := http.ListenAndServe(addr, nil);
	if err != nil {
		log.Exit("ListenAndServe:", err);
	}
}
