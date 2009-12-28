package web

import (
    "bytes"
    "bufio"
    "encoding/binary"
    "log"
    "net"
    "os"
)

const (
    FcgiBeginRequest = iota + 1
    FcgiAbortRequest
    FcgiEndRequest
    FcgiParams
    FcgiStdin
    FcgiStdout
    FcgiStderr
    FcgiData
    FcgiGetValues
    FcgiGetValuesResult
    FcgiUnknownType
    FcgiMaxType = FcgiUnknownType
)

const (
    FcgiRequestComplete = iota
    FcgiCantMpxConn
    FcgiOverloaded
    FcgiUnknownRole
)

type fcgiHeader struct {
    Version       uint8
    Type          uint8
    RequestId     uint16
    ContentLength uint16
    PaddingLength uint8
    Reserved      uint8
}

func (h fcgiHeader) bytes() []byte {
    order := binary.BigEndian
    buf := make([]byte, 8)
    buf[0] = h.Version
    buf[1] = h.Type
    order.PutUint16(buf[2:4], h.RequestId)
    order.PutUint16(buf[4:6], h.ContentLength)
    buf[6] = h.PaddingLength
    buf[7] = h.Reserved
    return buf
}

type fcgiEndRequest struct {
    appStatus      uint32
    protocolStatus uint8
    reserved       [3]uint8
}

func (er fcgiEndRequest) bytes() []byte {
    buf := make([]byte, 8)
    binary.BigEndian.PutUint32(buf, er.appStatus)
    buf[4] = er.protocolStatus
    return buf
}

type fcgiConn struct {
    requestId uint16
    fd        net.Conn
}

func (conn *fcgiConn) WriteString(s string) os.Error {
    content := bytes.NewBufferString(s).Bytes()
    l := len(content)
    // round to the nearest 8
    padding := make([]byte, uint8(-l&7))
    hdr := fcgiHeader{
        Version: 1,
        Type: FcgiStdout,
        RequestId: conn.requestId,
        ContentLength: uint16(l),
        PaddingLength: uint8(len(padding)),
    }

    conn.fd.Write(hdr.bytes())
    conn.fd.Write(content)
    conn.fd.Write(padding)

    return nil
}

func (conn *fcgiConn) StartResponse(status int) os.Error {
    return nil
}

func (conn *fcgiConn) Close() os.Error {
    content := fcgiEndRequest{appStatus: 200, protocolStatus: FcgiRequestComplete}.bytes()
    l := len(content)

    hdr := fcgiHeader{
        Version: 1,
        Type: FcgiEndRequest,
        RequestId: uint16(conn.requestId),
        ContentLength: uint16(l),
        PaddingLength: 0,
    }

    conn.fd.Write(hdr.bytes())
    conn.fd.Write(content)

    conn.fd.Close()

    return nil
}

func readFcgiParams(data []byte) {
    var params = make(map[string]string)

    for idx := 0; len(data) > idx; {
        var keySize int = int(data[idx])
        if keySize>>7 == 0 {
            idx += 1
        } else {
            binary.Read(bytes.NewBuffer(data[idx:idx+4]), binary.BigEndian, &keySize)
            idx += 4
        }

        var valSize int = int(data[idx])
        if valSize>>7 == 0 {
            idx += 1
        } else {
            binary.Read(bytes.NewBuffer(data[idx:idx+4]), binary.BigEndian, &valSize)
            idx += 4
        }

        key := data[idx : idx+keySize]
        idx += keySize
        val := data[idx : idx+valSize]
        idx += valSize

        println(string(key), ":", string(val))
        params[string(key)] = string(val)
    }
}

func handleFcgiRequest(fd net.Conn) {

    br := bufio.NewReader(fd)
    for {
        var h fcgiHeader
        err := binary.Read(br, binary.BigEndian, &h)
        if err != nil {
            log.Stderrf(err.String())
        }
        content := make([]byte, h.ContentLength)
        n, err := br.Read(content)
        //read padding
        if h.PaddingLength > 0 {
            padding := make([]byte, h.PaddingLength)
            n, err = br.Read(padding)
            println("read padding", n)
        }

        switch h.Type {
        case FcgiBeginRequest:
            println("begin request!")
        case FcgiParams:
            readFcgiParams(content)
        case FcgiStdin:
            println("stdin")
        case FcgiData:
            println("data!")
        case FcgiAbortRequest:
            println("abort!")
        }

    }
}

func listenAndServeFcgi(addr string) {
    println("listening and serving scgi!")
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
        go handleFcgiRequest(fd)

    }
}
