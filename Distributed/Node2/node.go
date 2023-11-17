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
/*func changeCurrentTurn(input int) {
	currentTurn.lock.Lock()
	currentTurn.turn = input
	currentTurn.lock.Unlock()
}*/

// GoLWorker does the actual working stuff
func GoLWorker(inputWorld [][]byte, p Shared.Params) [][]byte {
	var newWorld [][]byte
	//fmt.Println(p.Turns)
	if p.Turns == 0 {
		if p.ImageHeight == 16 {
			fmt.Println("Auto done: ", inputWorld)
		}
		return inputWorld
	}
	newWorld = worker(p.ImageHeight, p.ImageWidth, inputWorld)
	//currentWorld <- newWorld
	inputWorld = newWorld
	//turn <- i + 1
	changeCurrentWorld(inputWorld)

	paused.lock.Lock()
	paused.lock.Unlock()
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
/*func (s *GoLOperations) TickerManager(req Shared.Request, res *Shared.Response) (err error) {
	currentWorld.lock.Lock()
	res.World = currentWorld.world
	res.AliveCells = getAliveCellsCount(currentWorld.world)
	currentWorld.lock.Unlock()

	currentTurn.lock.Lock()
	res.Turns = currentTurn.turn
	currentTurn.lock.Unlock()
	return
}*/

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
		res.World = GoLWorker(req.World, req.Parameters)
		if req.Parameters.ImageWidth == 16 && req.Parameters.Turns == 1 {
			fmt.Println(res.World)
		}
	}

	return
}

// KYS :Handler whenever the user presses "K".
//Called from the local controller to tell the AWS node to kill itself
func (s *GoLOperations) KYS(*Shared.Request, *Shared.Response) (err error) {
	fmt.Println("Terminated sucessfully")

	defer os.Exit(0)
	return

}

// PauseManager :Handler whenever the user presses "p"
//If already paused then unpause, otherwise pause.
func (s *GoLOperations) PauseManager(*Shared.Request, *Shared.Response) (err error) {
	if paused.pause {
		fmt.Println("Unlocking")
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
func (s *GoLOperations) BackgroundManager(*Shared.Request, *Shared.Response) (err error) {
	fmt.Println("Stopped - waiting for a controller to reconnect to it.")
	paused.pause = true
	paused.lock.Lock()
	return
}

func main() {
	pAddr := flag.String("port", "8032", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	Shared.HandleRegisterAndError(&GoLOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			panic(err)
		}
	}(listener)
	rpc.Accept(listener)
}
