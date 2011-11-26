// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import "net/http"

var statusText = map[int]string{
    http.StatusContinue:           "Continue",
    http.StatusSwitchingProtocols: "Switching Protocols",

    http.StatusOK:                   "OK",
    http.StatusCreated:              "Created",
    http.StatusAccepted:             "Accepted",
    http.StatusNonAuthoritativeInfo: "Non-Authoritative Information",
    http.StatusNoContent:            "No Content",
    http.StatusResetContent:         "Reset Content",
    http.StatusPartialContent:       "Partial Content",

    http.StatusMultipleChoices:   "Multiple Choices",
    http.StatusMovedPermanently:  "Moved Permanently",
    http.StatusFound:             "Found",
    http.StatusSeeOther:          "See Other",
    http.StatusNotModified:       "Not Modified",
    http.StatusUseProxy:          "Use Proxy",
    http.StatusTemporaryRedirect: "Temporary Redirect",

    http.StatusBadRequest:                   "Bad Request",
    http.StatusUnauthorized:                 "Unauthorized",
    http.StatusPaymentRequired:              "Payment Required",
    http.StatusForbidden:                    "Forbidden",
    http.StatusNotFound:                     "Not Found",
    http.StatusMethodNotAllowed:             "Method Not Allowed",
    http.StatusNotAcceptable:                "Not Acceptable",
    http.StatusProxyAuthRequired:            "Proxy Authentication Required",
    http.StatusRequestTimeout:               "Request Timeout",
    http.StatusConflict:                     "Conflict",
    http.StatusGone:                         "Gone",
    http.StatusLengthRequired:               "Length Required",
    http.StatusPreconditionFailed:           "Precondition Failed",
    http.StatusRequestEntityTooLarge:        "Request Entity Too Large",
    http.StatusRequestURITooLong:            "Request URI Too Long",
    http.StatusUnsupportedMediaType:         "Unsupported Media Type",
    http.StatusRequestedRangeNotSatisfiable: "Requested Range Not Satisfiable",
    http.StatusExpectationFailed:            "Expectation Failed",

    http.StatusInternalServerError:     "Internal Server Error",
    http.StatusNotImplemented:          "Not Implemented",
    http.StatusBadGateway:              "Bad Gateway",
    http.StatusServiceUnavailable:      "Service Unavailable",
    http.StatusGatewayTimeout:          "Gateway Timeout",
    http.StatusHTTPVersionNotSupported: "HTTP Version Not Supported",
}
