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

type pausedStruct struct {
	pause bool
	lock  sync.Mutex
}

//Globals we made for general management of mutexes and variables
var currentWorld currentWorldStruct
var paused pausedStruct
var condition sync.WaitGroup

//General helper function for the global variables
//Locks current world's lock, changes the world value to the input, then Unlocks it
func changeCurrentWorld(input [][]byte) {
	currentWorld.lock.Lock()
	currentWorld.world = input
	currentWorld.lock.Unlock()
}

// GoLWorker does the actual working stuff
func GoLWorker(inputWorld [][]byte, p Shared.Params) [][]byte {
	var newWorld [][]byte
	if p.Turns == 0 {
		return inputWorld
	}

	newWorld = worker(p.ImageHeight, p.ImageWidth, inputWorld)
	inputWorld = newWorld
	changeCurrentWorld(inputWorld)

	paused.lock.Lock()
	paused.lock.Unlock()
	//Once all turns have been processed, free the condition variable
	condition.Done()
	return inputWorld
}

// GoLOperations :Handles GoL operations and all of its methods
type GoLOperations struct{}

// GoLManager :Handler for the actual GoL algorithm
func (s *GoLOperations) GoLManager(req *Shared.Request, res *Shared.Response) (err error) {
	condition.Add(1)
	res.World = GoLWorker(req.World, req.Parameters)
	return
}

// KYS :Handler whenever the user presses "K".
//Called from the local controller to tell the AWS node to kill itself
func (s *GoLOperations) KYS(*Shared.Request, *Shared.Response) (err error) {
	fmt.Println("Terminated successfully")

	defer os.Exit(0)
	return

}

// PauseManager :Handler whenever the user presses "p"
//If already paused then unpause, otherwise pause.
func (s *GoLOperations) PauseManager(req *Shared.Request, res *Shared.Response) (err error) {
	if !req.Paused && paused.pause {
		fmt.Println("Unlocking")
		paused.lock.Unlock()
	} else if req.Paused {
		fmt.Println("Locking")
		paused.lock.Lock()
	}
	paused.pause = req.Paused
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
	pAddr := flag.String("port", "8031", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	Shared.HandleRegisterAndError(&GoLOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	fmt.Println("Waiting")
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			panic(err)
		}
	}(listener)
	rpc.Accept(listener)
}
