# web.go for appengine

this appengine branch for web.go

## Installation

    # cd /path/go/appengine/project/example
    # mkdir -p github.com/hoisie
    # cd github.com/hoisie
    # git clone https://github.com/hoisie/web.go
    # cd web.go
    # git checkout appengine

## Example

    package example
    
    import (
    	"appengine"
    	"appengine/urlfetch"
    	"github.com/hoisie/web.go"
    	"http"
    	"io/ioutil"
    )
    
    func init() {
    	var c appengine.Context
    
    	web.Get("/", func(ctx *web.Context) {
    		client := urlfetch.Client(c)
    
    		r, _, err := client.Get("http://www.getwebgo.com/")
    		if err != nil {
    			ctx.Abort(500, err.String())
    			return
    		}
    		defer r.Body.Close()
    		b, err := ioutil.ReadAll(r.Body)
    		if err != nil {
    			ctx.Abort(500, err.String())
    			return
    		}
    		ctx.Write(b)
    	})
    
    	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    		c = appengine.NewContext(r)
    		web.ServeHTTP(w, r)
    	})
    }
