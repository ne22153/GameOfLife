package Controller

import (
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
	//main2 "uk.ac.bris.cs/gameoflife/Distributed/Controller"
)

const HARD_CODED_SERVER_PORT = "127.0.0.1:8030"

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Shared.Params, events chan<- Shared.Event, keyPresses <-chan rune) {

	//	TODO: Put the missing channels in here.

	p.ServerPort = HARD_CODED_SERVER_PORT

	ioCommand := make(chan IoCommand)
	ioIdle := make(chan bool)
	ioFilename := make(chan string, 1)
	ioOutput := make(chan uint8)
	ioInput := make(chan uint8)

	ioChannels := IoChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioOutput,
		input:    ioInput,
	}
	go startIo(p, ioChannels)

	distributorChannels := DistributorChannels{
		events:     events,
		ioCommand:  ioCommand,
		ioIdle:     ioIdle,
		ioFilename: ioFilename,
		ioOutput:   ioOutput,
		ioInput:    ioInput,
	}
	controller(p, distributorChannels, keyPresses)
}
