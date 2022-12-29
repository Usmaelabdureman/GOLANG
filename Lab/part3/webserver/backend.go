package main

import (
	"bufio"
	"net"
	"sync"
)

type Server struct {
	net.Conn
	Reader *bufio.Reader
	Writer *bufio.Writer
}

var ServerQueue chan *Server
var requestBytes map[string]int64
var requestLock sync.Mutex
