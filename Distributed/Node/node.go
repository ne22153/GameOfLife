package main

import (
	"flag"
	"math/rand"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
)

//Does the actual working stuff
func GoLWorker(inputWorld [][]byte, p Shared.Params) [][]byte {
	var newWorld [][]byte
	for i := 0; i < p.Turns; i++ {
		newWorld = worker(p.ImageHeight, p.ImageWidth, inputWorld)
	}
	return newWorld
}

type GoLOperations struct{}

func (s *GoLOperations) GoLManager(req Shared.Request, res *Shared.Response) (err error) {
	res.World = GoLWorker(req.World, req.Parameters)
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
