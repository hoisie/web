package web

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"strings"
)

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
