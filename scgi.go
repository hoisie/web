package web

import (
    "bytes"
    "http"
    "io/ioutil"
    "log"
    "net"
    "strconv"
)

func readScgiRequest(buf *bytes.Buffer) Request {
    headers := make(map[string]string)

    content := buf.Bytes()
    colon := bytes.IndexByte(content, ':')
    content = content[colon+1:]
    fields := bytes.Split(content, []byte{0}, 0)
    for i := 0; i < len(fields)-1; i += 2 {
        key := string(fields[i])
        value := string(fields[i+1])
        headers[key] = value
    }
    var body bytes.Buffer
    body.Write(fields[len(fields)-1][1:])

    var httpheader = make(map[string]string)

    method, _ := headers["REQUEST_METHOD"]
    ctype, _ := headers["CONTENT_TYPE"]
    clength, _ := headers["CONTENT_LENGTH"]
    host, _ := headers["HTTP_HOST"]
    path, _ := headers["REQUEST_URI"]
    port, _ := headers["SERVER_PORT"]
    proto, _ := headers["SERVER_PROTOCOL"]
    rawurl := "http://" + host + ":" + port + path
    url, _ := http.ParseURL(rawurl)
    useragent, _ := headers["USER_AGENT"]

    httpheader["Content-Length"] = clength
    httpheader["Content-Type"] = ctype

    req := Request{Method: method,
        RawURL: rawurl,
        URL: url,
        Proto: proto,
        Host: host,
        UserAgent: useragent,
        Body: &body,
        Header: httpheader,
    }

    return req
}

func handleScgiRequest(fd net.Conn) {
    var buf bytes.Buffer
    var tmp [1024]byte
    n, err := fd.Read(&tmp)
    if err != nil || n == 0 {
        return
    }

    colonPos := bytes.IndexByte(tmp[0:], ':')

    read := n
    length, _ := strconv.Atoi(string(tmp[0:colonPos]))
    buf.Write(tmp[0:n])

    for read < length {
        n, err := fd.Read(&tmp)
        if err != nil || n == 0 {
            break
        }
        buf.Write(tmp[0:n])
        read += n
    }

    req := readScgiRequest(&buf)
    perr := req.ParseForm()
    if perr != nil {
        log.Stderrf("Failed to parse form data %q", req.Body)
    }
    resp := routeHandler(&req)

    var scgiResp bytes.Buffer
    scgiResp.WriteString("Status: " + resp.Status + "\r\n")
    scgiResp.WriteString("Content-Type: text/html\r\n\r\n")
    fd.Write(scgiResp.Bytes())
    if resp.Body != nil {
        body, _ := ioutil.ReadAll(resp.Body)
        fd.Write(body)
    }
    fd.Close()
}

func listenAndServeScgi(addr string) {
    l, err := net.Listen("tcp", addr)
    if err != nil {
        log.Stderrf(err.String())
        return
    }

    for {
        fd, err := l.Accept()
        if err != nil {
            log.Stderrf(err.String())
            break
        }
        go handleScgiRequest(fd)

    }
}
