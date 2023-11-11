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
func GoLWorker(inputWorld [][]byte, p Shared.Params, turn chan<- int, currentWorld chan [][]byte) [][]byte {
	var newWorld [][]byte
	fmt.Println(p.Turns)
	if p.Turns == 0 {
		if p.ImageHeight == 16 {
			fmt.Println("Auto done: ", inputWorld)
		}
		currentWorld <- inputWorld
		return inputWorld
	}
	for i := 0; i < p.Turns; i++ {
		newWorld = worker(p.ImageHeight, p.ImageWidth, inputWorld)
		inputWorld = newWorld
		currentWorld <- inputWorld
		turn <- i + 1
	}

	return inputWorld
}

func getAliveCellsCount(inputWorld [][]byte) int {
	aliveCells := 0

	for _, row := range inputWorld {
		for _, tile := range row {
			if tile == LIVE {
				aliveCells++
			}
		}
	}

	return aliveCells
}

func aliveCellsReporter(turn, aliveCells *int, ticker *time.Ticker, c chan<- Shared.Event) {
	for {
		select {
		case <-ticker.C:
			c <- Shared.AliveCellsCount{CompletedTurns: *turn, CellsCount: *aliveCells}
			fmt.Println("Sent")
		}
	}
}

type GoLOperations struct{}

func (s *GoLOperations) TickerManager(req Shared.Request, res *Shared.Response) (err error) {
	fmt.Println("entered")
	res.AliveCells = getAliveCellsCount(req.World)
	return
}

func (s *GoLOperations) GoLManager(req Shared.Request, res *Shared.Response) (err error) {
	res.World = GoLWorker(req.World, req.Parameters, req.CurrentTurn, req.CurrentWorld)
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
