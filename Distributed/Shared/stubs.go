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
var SuicideHandler = "GoLOperations.KYS"
var PauseHandler = "GoLOperations.PauseManager"
var BackgroundHandler = "GoLOperations.BackgroundManager"

var BrokerHandler = "BrokerOperations.GoLManager"
var BrokerInfo = "BrokerOperations.BrokerInfo"
var BrokerKill = "BrokerOperations.KYS"
var BrokerPause = "BrokerOperations.PauseManager"
var BrokerBackground = "BrokerOperations.BackgroundManager"

var ControllerHandler = "ControllerOperations.CellReporter"

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
	OldWorld     [][]byte
	Turn         int
}

func HandleError(err error) {
	if err != nil {
		log.Fatal("Error: ", err)
	}
}
