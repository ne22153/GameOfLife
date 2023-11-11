package Shared

import (
	"fmt"
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
var TickersHandler = "GoLOperations.TickerManager"

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
}

func HandleError(err error) {
	if err != nil {
		fmt.Println("broken here")
		log.Fatal(err)
	}
}
