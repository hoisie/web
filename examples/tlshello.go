package main

import (
    "github.com/rday/web"
    "crypto/tls"
    "fmt"
)

// Generate you key and cert
// openssl req -new -x509 -key ~/.ssh/id_rsa -out cacert.pem -days 1095
var SERVER_CERT = []byte(`----- YOUR CERTIFICATE -----`)
var SERVER_KEY = []byte(`----- YOUR PRIVATE KEY -----`)

func hello(val string) (string, error) { 
    return "hello " + val, nil
}
func main() {

    config := tls.Config{
                Time: nil,
                }

    config.Certificates = make([]tls.Certificate, 1)
    var err error
    config.Certificates[0], err = tls.X509KeyPair(SERVER_CERT, SERVER_KEY)
    if err != nil {
        fmt.Println(err)
    }

    web.Get("/(.*)", hello)
    web.RunSecure("0.0.0.0:9998", config)
}
