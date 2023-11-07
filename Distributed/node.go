package Distributed

import (
	"flag"
	"math/rand"
	"net"
	"net/rpc"
	"time"
)

//Does the actual working stuff
func GoLWorker(s [][]byte, i int) [][]byte {
	return make([][]byte, 0)
}

type GoLOperations struct{}

func (s *GoLOperations) GoLManager(req Request, res *Response) (err error) {
	res.World = GoLWorker(req.World, 2)
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&GoLOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
