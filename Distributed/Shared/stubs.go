package Shared

import (
	"log"
)

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	ServerPort  string
}

var GoLHandler = "GoLOperations.GoLManager"
var InfoHandler = "GoLOperations.TickerManager"
var SuicideHandler = "GoLOperations.KYS"
var PauseHandler = "GoLOperations.PauseManager"
var BackgroundHandler = "GoLOperations.BackgroundManager"

type Response struct {
	World      [][]byte
	AliveCells int
	Turns      int
}

type Request struct {
	World        [][]byte
	Parameters   Params
	Events       chan<- Event
	CurrentTurn  chan int
	CurrentWorld chan [][]byte
	CallAlive    chan int
	GetAlive     chan int
	GetTurn      chan int
}

func HandleError(err error) {
	if err != nil {
		log.Fatal("Error: ", err)
	}
}
