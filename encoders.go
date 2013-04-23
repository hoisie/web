// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"encoding/json"
	"encoding/xml"
	"io"
)

// Encode arbitrary data to a response
type Encoder interface {
	Encode(data interface{}) error
}

type MimeEncoder func(w io.Writer) Encoder

var encoders = map[string]MimeEncoder{
	"application/json": encodeJSON,
	"application/xml":  encodeXML,
	"text/xml":         encodeXML,
}

// Register a new mimetype and how it should be encoded
func RegisterMimeParser(mimetype string, enc MimeEncoder) {
	encoders[mimetype] = enc
}

func encodeJSON(w io.Writer) Encoder {
	return Encoder(json.NewEncoder(w))
}

func encodeXML(w io.Writer) Encoder {
	return Encoder(xml.NewEncoder(w))
}
