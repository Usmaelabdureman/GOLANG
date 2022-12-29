package main

import (
	"bufio"
	format "fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
)

func main() {
	format.Printf("Tamirat")
	ln, err := net.Listen("tcp", ":8080")

	if err != nil {
		log.Fatalf("Failed to listen: %s", err)
	}

	for {
		if conn, err := ln.Accept(); err == nil {
			go handleConnection(conn)
		}
	}

}

func handleConnection(conn net.Conn)  {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("Failed lala to read request: %s", err)
				
			}
			return 

		}
		if be, err := net.Dial("tcp", "127.0.0.1:8081"); err == nil {
			be_reader := bufio.NewReader(be)

			if err := req.Write(be); err == nil {

				// 6. read the respoonse from the backend.

				if resp, err := http.ReadResponse(be_reader, req); err == nil {
					// 7. Send the reponse to the client
					// resp.Close = true
					bytes := updateStats(req, resp)

					resp.Header.Set("X-Bytes", strconv.FormatInt(bytes, 10))

					if err := resp.Write(conn); err == nil {

						format.Printf("what is hapsdwspening\n")
						log.Printf("%s: %d", req.URL.Path, resp.StatusCode)

					}

					conn.Close()
				}
			}
		}

	}
	
}

var requestBytes map[string]int64
var requestLock sync.Mutex

func init()  {
	requestBytes = make(map[string]int64)
}


func updateStats(req *http.Request, resp *http.Response) int64 {
	requestLock.Lock()
	defer requestLock.Unlock()

	bytes := requestBytes[req.URL.Path] + resp.ContentLength
	requestBytes[req.URL.Path] = bytes
	
	return bytes
	
}