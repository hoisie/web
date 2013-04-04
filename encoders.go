package web

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
)

type MimeEncoder func(interface{}) ([]byte, error)

var Encoders = make(map[string]MimeEncoder, 100)

/**
Register a new mimetype and how it should be encoded
*/
func RegisterMimeParser(mimetype string, enc MimeEncoder) error {
	Encoders[mimetype] = enc
	return nil
}

/**
Default encoders
*/
func JSONparser(content interface{}) ([]byte, error) {
	var encoded bytes.Buffer

	enc := json.NewEncoder(&encoded)
	err := enc.Encode(content)
	if err != nil {
		return nil, err
	}

	return encoded.Bytes(), nil
}

func XMLparser(content interface{}) ([]byte, error) {
	var encoded bytes.Buffer

	enc := xml.NewEncoder(&encoded)
	err := enc.Encode(content)
	if err != nil {
		return nil, err
	}

	return encoded.Bytes(), nil
}

func Binaryparser(content interface{}) ([]byte, error) {
	return content.([]byte), nil
}
