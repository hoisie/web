package main

import (
	"crypto/tls"
	"github.com/hoisie/web"
)

// an arbitrary self-signed certificate, generated with
// `openssl req -x509 -nodes -days 365 -newkey rsa:1024 -keyout cert.pem -out cert.pem`

var pkey = `-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQDVBUw40q0zpF/zWzwBf0GFkXmnkw+YCNTiV8l7mso1DCv/VTYM
cqtvy0g2KNBV7SFLC+NHuxJkNOAtJ8Fxx1EpeIw5A3KeCRNb4lo6ecAkuDLiPYGO
qgAqjj8QmhmZA68qTIuWGYM1FTtUK3wO4wrHnqHEjs3cWNghmby6AgLHVQIDAQAB
AoGAcy5GJINlu4KpjwBJ1dVlLD+YtA9EY0SDN0+YVglARKasM4dzjg+CuxQDm6U9
4PgzBE0NO3/fVedxP3k7k7XeH73PosaxjWpfMawXR3wSLFKJBwxux/8gNdzeGRHN
X1sYsJ70WiZLFOAPQ9jctF1ejUP6fpLHsti6ZHQj/R1xqBECQQDrHxmpMoviQL6n
4CBR4HvlIRtd4Qr21IGEXtbjIcC5sgbkfne6qhqdv9/zxsoiPTi0859cr704Mf3y
cA8LZ8c3AkEA5+/KjSoqgzPaUnvPZ0p9TNx6odxMsd5h1AMIVIbZPT6t2vffCaZ7
R0ffim/KeWfoav8u9Cyz8eJpBG6OHROT0wJBAML54GLCCuROAozePI8JVFS3NqWM
OHZl1R27NAHYfKTBMBwNkCYYZ8gHVKUoZXktQbg1CyNmjMhsFIYWTTONFNMCQFsL
eBld2f5S1nrWex3y0ajgS4tKLRkNUJ2m6xgzLwepmRmBf54MKgxbHFb9dx+dOFD4
Bvh2q9RhqhPBSiwDyV0CQBxN3GPbaa8V7eeXBpBYO5Evy4VxSWJTpgmMDtMH+RUp
9eAJ8rUyhZ2OaElg1opGCRemX98s/o2R5JtzZvOx7so=
-----END RSA PRIVATE KEY-----
`

var cert = `-----BEGIN CERTIFICATE-----
MIIDXDCCAsWgAwIBAgIJAJqbbWPZgt0sMA0GCSqGSIb3DQEBBQUAMH0xCzAJBgNV
BAYTAlVTMQswCQYDVQQIEwJDQTEWMBQGA1UEBxMNU2FuIEZyYW5jaXNjbzEPMA0G
A1UEChMGV2ViLmdvMRcwFQYDVQQDEw5NaWNoYWVsIEhvaXNpZTEfMB0GCSqGSIb3
DQEJARYQaG9pc2llQGdtYWlsLmNvbTAeFw0xMzA0MDgxNjIzMDVaFw0xNDA0MDgx
NjIzMDVaMH0xCzAJBgNVBAYTAlVTMQswCQYDVQQIEwJDQTEWMBQGA1UEBxMNU2Fu
IEZyYW5jaXNjbzEPMA0GA1UEChMGV2ViLmdvMRcwFQYDVQQDEw5NaWNoYWVsIEhv
aXNpZTEfMB0GCSqGSIb3DQEJARYQaG9pc2llQGdtYWlsLmNvbTCBnzANBgkqhkiG
9w0BAQEFAAOBjQAwgYkCgYEA1QVMONKtM6Rf81s8AX9BhZF5p5MPmAjU4lfJe5rK
NQwr/1U2DHKrb8tINijQVe0hSwvjR7sSZDTgLSfBccdRKXiMOQNyngkTW+JaOnnA
JLgy4j2BjqoAKo4/EJoZmQOvKkyLlhmDNRU7VCt8DuMKx56hxI7N3FjYIZm8ugIC
x1UCAwEAAaOB4zCB4DAdBgNVHQ4EFgQURizcvrgUl8yhIEQvJT/1b5CzV8MwgbAG
A1UdIwSBqDCBpYAURizcvrgUl8yhIEQvJT/1b5CzV8OhgYGkfzB9MQswCQYDVQQG
EwJVUzELMAkGA1UECBMCQ0ExFjAUBgNVBAcTDVNhbiBGcmFuY2lzY28xDzANBgNV
BAoTBldlYi5nbzEXMBUGA1UEAxMOTWljaGFlbCBIb2lzaWUxHzAdBgkqhkiG9w0B
CQEWEGhvaXNpZUBnbWFpbC5jb22CCQCam21j2YLdLDAMBgNVHRMEBTADAQH/MA0G
CSqGSIb3DQEBBQUAA4GBAGBPoVCReGMO1FrsIeVrPV/N6pSK7H3PLdxm7gmmvnO9
K/LK0OKIT7UL3eus+eh0gt0/Tv/ksq4nSIzXBLPKyPggLmpC6Agf3ydNTpdLQ23J
gWrxykqyLToIiAuL+pvC3Jv8IOPIiVFsY032rOqcwSGdVUyhTsG28+7KnR6744tM
-----END CERTIFICATE-----
`

func hello(val string) string { return "hello " + val + "\n" }

func main() {
	config := tls.Config{
		Time: nil,
	}

	config.Certificates = make([]tls.Certificate, 1)
	var err error
	config.Certificates[0], err = tls.X509KeyPair([]byte(cert), []byte(pkey))
	if err != nil {
		println(err.Error())
		return
	}

	// you must access the server with an HTTPS address, i.e https://localhost:9999/world
	web.Get("/(.*)", hello)
	web.RunTLS("0.0.0.0:9999", &config)
}
