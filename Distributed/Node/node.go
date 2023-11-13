package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
)

type currentWorldStruct struct {
	lock  sync.Mutex
	world [][]byte
}

var currentWorld currentWorldStruct

type currentTurnStruct struct {
	lock sync.Mutex
	turn int
}

var currentTurn currentTurnStruct

type pausedStruct struct {
	lock  sync.Mutex
	pause bool
}

var paused pausedStruct

var condition sync.WaitGroup

//Does the actual working stuff
func GoLWorker(inputWorld [][]byte, p Shared.Params, turn chan<- int, currentWorldChannel chan [][]byte) [][]byte {
	var newWorld [][]byte
	fmt.Println(p.Turns)
	if p.Turns == 0 {
		if p.ImageHeight == 16 {
			fmt.Println("Auto done: ", inputWorld)
		}
		//currentWorld <- inputWorld

		return inputWorld
	}
	for i := 0; i < p.Turns; i++ {
		newWorld = worker(p.ImageHeight, p.ImageWidth, inputWorld)
		//currentWorld <- newWorld
		inputWorld = newWorld
		//turn <- i + 1
		currentWorld.lock.Lock()
		currentWorld.world = inputWorld
		currentWorld.lock.Unlock()

		currentTurn.lock.Lock()
		currentTurn.turn = i + 1
		currentTurn.lock.Unlock()

		paused.lock.Lock()
		paused.lock.Unlock()
		fmt.Println("Done", i+1)
	}
	condition.Done()
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

type GoLOperations struct{}

func (s *GoLOperations) TickerManager(req Shared.Request, res *Shared.Response) (err error) {
	currentWorld.lock.Lock()
	res.World = currentWorld.world
	res.AliveCells = getAliveCellsCount(currentWorld.world)
	currentWorld.lock.Unlock()

	currentTurn.lock.Lock()
	res.Turns = currentTurn.turn
	currentTurn.lock.Unlock()
	return
}

func (s *GoLOperations) GoLManager(req *Shared.Request, res *Shared.Response) (err error) {
	//go WorkerTracker(req.World, req.Parameters, req.CurrentTurn, req.CurrentWorld, req.CallAlive, req.GetAlive, req.GetTurn)
	//fmt.Println("GoL gets: ", &req.CallAlive)
	if paused.pause {
		paused.lock.Unlock()
		paused.pause = !paused.pause

		condition.Wait()
		currentWorld.lock.Lock()
		res.World = currentWorld.world
		currentWorld.lock.Unlock()

	} else {
		condition.Add(1)
		res.World = GoLWorker(req.World, req.Parameters, req.CurrentTurn, req.CurrentWorld)
	}
	//paused.lock.Unlock()
	//res.World = GoLWorker(req.World, req.Parameters, req.CurrentTurn, req.CurrentWorld)
	return
}

func (s *GoLOperations) KYS(req *Shared.Request, res *Shared.Response) (err error) {
	fmt.Println("Did the shit")
	//paused.lock.Lock()
	os.Exit(0)
	return
}

func (s *GoLOperations) PauseManager(req *Shared.Request, res *Shared.Response) (err error) {
	if paused.pause {
		paused.lock.Unlock()
	} else {
		fmt.Println("Locking")
		paused.lock.Lock()
	}
	paused.pause = !paused.pause
	return
}

func (s *GoLOperations) BackgroundManager(req *Shared.Request, res *Shared.Response) (err error) {
	fmt.Println("Stopped for now")
	paused.pause = true
	paused.lock.Lock()
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
