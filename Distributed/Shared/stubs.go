package Shared

import (
	"log"
	"uk.ac.bris.cs/gameoflife/util"
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

// PauseManager :Handler whenever the user presses "p"
//If already paused then unpause, otherwise pause.
var PauseHandler = "GoLOperations.PauseManager"
var BackgroundHandler = "GoLOperations.BackgroundManager"

var BrokerHandler = "BrokerOperations.GoLManager"
var BrokerInfo = "BrokerOperations.BrokerInfo"
var BrokerKill = "BrokerOperations.KYS"
var BrokerPause = "BrokerOperations.PauseManager"
var BrokerBackground = "BrokerOperations.BackgroundManager"

type Response struct {
	World        [][]byte
	AliveCells   int
	Turns        int
	Resend       bool
	FlippedCells []util.Cell
}

type Request struct {
	World      [][]byte
	Parameters Params
	Events     chan<- Event
	Turn       int
	Paused     bool
}

func HandleError(err error) {
	if err != nil {
		log.Fatal("Error: ", err)
	}
}
