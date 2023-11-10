package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
)

//Does the actual working stuff
func GoLWorker(inputWorld [][]byte, p Shared.Params) [][]byte {
	var newWorld [][]byte
	fmt.Println(p.Turns)
	if p.Turns == 0 {
		if p.ImageHeight == 16 {
			fmt.Println("Auto done: ", inputWorld)
		}
		return inputWorld
	}
	for i := 0; i < p.Turns-1; i++ {
		if p.ImageWidth == 16 && p.Turns < 10 {
			fmt.Println("Turn number: ", i-1, " World: ", inputWorld)
		}
		newWorld = worker(p.ImageHeight, p.ImageWidth, inputWorld)
		inputWorld = newWorld
		if p.ImageWidth == 16 && p.Turns < 10 {
			fmt.Println("Turn number: ", i, " World: ", inputWorld)
		}
	}

	return inputWorld
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
