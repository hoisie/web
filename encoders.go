// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
)

type MimeEncoder func(interface{}) ([]byte, error)

var encoders = map[string]MimeEncoder{
	"application/json": encodeJSON,
	"application/xml":  encodeXML,
	"text/xml":         encodeXML,
}

// Register a new mimetype and how it should be encoded
func RegisterMimeParser(mimetype string, enc MimeEncoder) {
	encoders[mimetype] = enc
}

func encodeJSON(content interface{}) ([]byte, error) {
	var encoded bytes.Buffer
	enc := json.NewEncoder(&encoded)
	err := enc.Encode(content)
	if err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

func encodeXML(content interface{}) ([]byte, error) {
	var encoded bytes.Buffer
	enc := xml.NewEncoder(&encoded)
	err := enc.Encode(content)
	if err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}
