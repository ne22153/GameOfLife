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
		//currentWorld <- newWorld
		inputWorld = newWorld
		fmt.Println("Passing new world")
		//This line is causing the whole thing to halt, as nothing is reading from the channel
		//WorkerTracker is supposed to, but it seems the channels aren't the same for some reason
		turn <- i + 1
		fmt.Println("hahah")
	}

	return inputWorld
}

func WorkerTracker(inputWorld [][]byte, p Shared.Params, turn chan int, world chan [][]byte, alive chan int, give chan int, getTurn chan int) {
	var currentTurn = 0
	var currentWorld [][]byte
	fmt.Println("I've been activated")
	for {
		select {
		case t := <-turn:
			currentTurn = t
			fmt.Println("Turn changed")
		case newWorld := <-world:
			fmt.Println("World changed")
			currentWorld = newWorld
		case <-alive:
			fmt.Println("Call received")
			getTurn <- currentTurn
			give <- getAliveCellsCount(currentWorld)
		default:
			time.Sleep(2 * time.Second)
			fmt.Println("Argh, nothing is happening")
		}
	}
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

type GoLOperations struct{}

func (s *GoLOperations) TickerManager(req Shared.Request, res *Shared.Response) (err error) {
	fmt.Println("Sending a signal")
	req.CallAlive <- 1
	fmt.Println("Awaiting response")
	res.Turns = <-req.GetTurn
	res.AliveCells = <-req.GetAlive
	return
}

func (s *GoLOperations) GoLManager(req Shared.Request, res *Shared.Response) (err error) {
	go WorkerTracker(req.World, req.Parameters, req.CurrentTurn, req.CurrentWorld, req.CallAlive, req.GetAlive, req.GetTurn)
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
