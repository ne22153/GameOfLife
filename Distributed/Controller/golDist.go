package Controller

import (
	"fmt"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
	//main2 "uk.ac.bris.cs/gameoflife/Distributed/Controller"
)

// Params provides the details of how to run the Game of Life and which image to load.
/*type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}*/

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Shared.Params, events chan<- Shared.Event, keyPresses <-chan rune) {

	//	TODO: Put the missing channels in here.

	p.ServerPort = "127.0.0.1:8030"

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
	fmt.Println("Finished for real")
}
