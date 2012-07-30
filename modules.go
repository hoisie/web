package web

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"strings"
)

/**
 Attempts to encode the response according to the client's Accept-Encoding
 header. If there is an error, or if the encoding requests aren't supported
 then the original content is returned.

 Encoding type:
  * deflate (zlib stream)
  * gzip

 This should be the last module loaded
*/
func EncodeResponse(ctx *Context, content []byte) ([]byte, error) {
	var compressed bytes.Buffer
	var output io.WriteCloser

	if len(ctx.Request.Header["Accept-Encoding"]) > 0 {
		for _, opt := range ctx.Request.Header["Accept-Encoding"] {
			if strings.Index(opt, "gzip") >= 0 {
				output = gzip.NewWriter(&compressed)
				ctx.SetHeader("Content-Encoding", "gzip", true)
			}
			if strings.Index(opt, "deflate") >= 0 {
				output = zlib.NewWriter(&compressed)
				ctx.SetHeader("Content-Encoding", "deflate", true)
			}
		}
	}

	if output != nil {
		_, err := output.Write(content)
		if err != nil {
			return content, err
		}
		err = output.Close()
		return compressed.Bytes(), nil
	}

	return content, nil
}
