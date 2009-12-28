package web

import (
    "bytes"
    "bufio"
    "encoding/binary"
    "log"
    "net"
)

const (
    FcgiRequestComplete = 0
    FcgiBeginRequest    = 1
    FcgiAbortRequest    = 2
    FcgiEndRequest      = 3
    FcgiParams          = 4
    FcgiStdin           = 5
    FcgiStdout          = 6
    FcgiStderr          = 7
    FcgiData            = 8
    FcgiGetValues       = 9
    FcgiGetValuesResult = 10
    FcgiUnknownType     = 11
    FcgiMaxType         = FcgiUnknownType
)

type FcgiHeader struct {
    Version       uint8
    Type          uint8
    RequestId     uint16
    ContentLength uint16
    PaddingLength uint8
    Reserved      uint8
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
        var h FcgiHeader
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
        //case FcgiBeginRequest:
        //  println("begin request!");
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
