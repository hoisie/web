// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"mime"
	"path"
	"reflect"
	"strings"
)

/**
This is really only useful for specific requests and should probably
be thought out a little more. If a client accepts * / *, how do we
determine what the best Content-Type is for the particular data?
Right now we would just send the original content
*/
func MarshalResponse(ctx *Context, content interface{}) (interface{}, error) {
	if len(ctx.Request.Header["Accept"]) > 0 {
		for _, accepts := range ctx.Request.Header["Accept"] {

			/**
			If no specific type is specified, we will try to guess based
			on the extension of the resource. */
			if accepts == "*/*" {
				mimetype := mime.TypeByExtension(path.Ext(ctx.Request.URL.Path))
				accepts = mimetype
			}

			encoder, ok := Encoders[accepts]
			if ok {
				encoded, err := encoder(content)
				ctx.SetHeader("Content-Type", accepts, true)
				//ctx.SetHeader("Content-Length", fmt.Sprintf("%d", len(encoded)), true)
				if err != nil {
					ctx.Server.Logger.Printf("MarshalResponse encoder failed: %s", err)
					return nil, &WebError{500, err.Error()}
				}
				return encoded, nil
			}
		}
	}

	// If no mimetype was found, just try to convert to []byte
	if content != nil {
		if reflect.TypeOf(content).String() == "string" {
			return []byte(content.(string)), nil
		} else {
			return content, nil
		}
	}
	return []byte(""), nil
}

/**
Attempts to encode the response according to the client's Accept-Encoding
header. If there is an error, or if the encoding requests aren't supported
then the original content is returned.

Encoding type:
 * deflate (zlib stream)
 * gzip

This should be the last module loaded
*/
func EncodeResponse(ctx *Context, content interface{}) (interface{}, error) {
	var compressed bytes.Buffer
	var output io.WriteCloser

	if len(ctx.Request.Header["Accept-Encoding"]) > 0 {
		for _, opt := range ctx.Request.Header["Accept-Encoding"] {
			if strings.Index(opt, "gzip") >= 0 {
				output = gzip.NewWriter(&compressed)
				ctx.SetHeader("Content-Encoding", "gzip", true)
			} else if strings.Index(opt, "deflate") >= 0 {
				output = zlib.NewWriter(&compressed)
				ctx.SetHeader("Content-Encoding", "deflate", true)
			}
		}
	}

	if output != nil {
		_, err := output.Write(content.([]byte))
		if err != nil {
			ctx.Server.Logger.Printf("EncodeResponse write failed: %s", err)
			return content, &WebError{500, err.Error()}
		}
		err = output.Close()
		return compressed.Bytes(), nil
	}

	return content, nil
}
