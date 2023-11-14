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
	world [][]byte
	lock  sync.Mutex
}

type currentTurnStruct struct {
	turn int
	lock sync.Mutex
}

type pausedStruct struct {
	pause bool
	lock  sync.Mutex
}

//Globals we made for general management of mutexes and variables
var currentWorld currentWorldStruct
var currentTurn currentTurnStruct
var paused pausedStruct
var condition sync.WaitGroup

//General helper function for the global variables
//Locks current world's lock, changes the world value to the input, then Unlocks it
func changeCurrentWorld(input [][]byte) {
	currentWorld.lock.Lock()
	currentWorld.world = input
	currentWorld.lock.Unlock()
}

//General helper function for the global variables
//Locks current turn's lock, changes the turn value to the input, then Unlocks it
func changeCurrentTurn(input int) {
	currentTurn.lock.Lock()
	currentTurn.turn = input
	currentTurn.lock.Unlock()
}

//Does the actual working stuff
func GoLWorker(inputWorld [][]byte, p Shared.Params, turn chan<- int, currentWorldChannel chan [][]byte) [][]byte {
	var newWorld [][]byte
	fmt.Println(p.Turns)
	if p.Turns == 0 {
		if p.ImageHeight == 16 {
			fmt.Println("Auto done: ", inputWorld)
		}
		return inputWorld
	}
	for i := 0; i < p.Turns; i++ {
		newWorld = worker(p.ImageHeight, p.ImageWidth, inputWorld)
		//currentWorld <- newWorld
		inputWorld = newWorld
		//turn <- i + 1
		changeCurrentWorld(inputWorld)
		changeCurrentTurn(i + 1)

		paused.lock.Lock()
		paused.lock.Unlock()
		fmt.Println("Done", i+1)
	}
	//Once all turns have been processed, free the condition variable
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

// GoLOperations :Handles GoL operations and all of its methods
type GoLOperations struct{}

// TickerManager :Handler for the ticker. Extracts the alive cells count and the turns of the current running state
//into the response.
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

// GoLManager :Handler for the actual GoL algorithm
func (s *GoLOperations) GoLManager(req *Shared.Request, res *Shared.Response) (err error) {
	//There already existed a GoL instance running (due to keypress Q, do this)
	if paused.pause {
		//Restarts the GoL instance running
		paused.lock.Unlock()
		paused.pause = !paused.pause

		condition.Wait()
		currentWorld.lock.Lock()
		res.World = currentWorld.world
		currentWorld.lock.Unlock()
	} else { //If the node is fresh and no previous GoL instance was running in the past
		condition.Add(1)
		res.World = GoLWorker(req.World, req.Parameters, req.CurrentTurn, req.CurrentWorld)
	}

	return
}

// KYS :Handler whenever the user presses "K".
//Called from the local controller to tell the AWS node to kill itself
func (s *GoLOperations) KYS(req *Shared.Request, res *Shared.Response) (err error) {
	fmt.Println("Terminated sucessfully")

	defer os.Exit(0)
	return

}

// PauseManager :Handler whenever the user presses "p"
//If aready pasued then unpause, otherwise pause.
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

// BackgroundManager :Handler whenever the user presses "q"
//	WHen the local controller is killed, then pause the node and then wait until a new local controller is created
// This is a form of fault tolerance.
func (s *GoLOperations) BackgroundManager(req *Shared.Request, res *Shared.Response) (err error) {
	fmt.Println("Stopped - waiting for a controller to reconnect to it.")
	paused.pause = true
	paused.lock.Lock()
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	var registerError error = rpc.Register(&GoLOperations{})
	Shared.HandleError(registerError)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
