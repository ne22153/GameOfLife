package main

import (
	"flag"
	"fmt"
	"runtime"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
	"uk.ac.bris.cs/gameoflife/Distributed/SharedSDL"
	"uk.ac.bris.cs/gameoflife/util"
)

const LIVE = 255

var Channels DistributorChannels

type DistributorChannels struct {
	events    chan<- Shared.Event
	ioCommand chan<- IoCommand
	ioIdle    <-chan bool

	ioFilename chan<- string
	ioOutput   chan<- byte
	ioInput    <-chan byte
}

func flipWorldCellsInitial(world [][]byte, imageHeight, imageWidth, turn int, c DistributorChannels) {
	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			if world[i][j] == LIVE {
				c.events <- Shared.CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: j, Y: i}}
			}
		}
	}
	resourceLock.Lock()
	c.events <- Shared.TurnComplete{CompletedTurns: 0}
	resourceLock.Unlock()
}

//Main logic where we control all of our AWS nodes. Also controls the ticker and keypress logic as well.
func controller(params Shared.Params, channels DistributorChannels, keyPresses <-chan rune) {
	fmt.Println("I have been beckoned")
	var client = Shared.HandleCreateClientAndError(params.ServerPort)
	Channels = channels

	//Create request response pair
	request, response := createRequestResponsePair(params, channels)

	//Make a ticker for the updates
	ticker := time.NewTicker(2 * time.Second)
	//Set up the ticker and keypress processes
	go aliveCellsReporter(ticker, channels, client, request, response)
	go determineKeyPress(client, keyPresses, &request, response, ticker, channels)

	//We set up our broker
	Shared.HandleCallAndError(client, Shared.BrokerHandler, &request, response)

	resourceLock.Lock()
	immuableFinalWorld := CopyWorldImmutable(response.World)
	resourceLock.Unlock()

	channels.events <- Shared.FinalTurnComplete{
		CompletedTurns: params.Turns,
		Alive:          calculateAliveCells(immuableFinalWorld)}
	//Shut down the game safely

	handleGameShutDown(client, response, params, channels, ticker)
}

func main() {
	runtime.LockOSThread()
	var params Shared.Params

	server := flag.String("server", "127.0.0.1:8030", "IP:port string to connect to as server")
	flag.Parse()

	params.ServerPort = *server
	fmt.Println("Server: ", *server)

	flag.IntVar(
		&params.Threads,
		"t",
		8,
		"Specify the number of worker threads to use. Defaults to 8.")

	flag.IntVar(
		&params.ImageWidth,
		"w",
		512,
		"Specify the width of the image. Defaults to 512.")

	flag.IntVar(
		&params.ImageHeight,
		"h",
		512,
		"Specify the height of the image. Defaults to 512.")

	flag.IntVar(
		&params.Turns,
		"turns",
		10000,
		"Specify the number of turns to process. Defaults to 10000.")

	flag.Parse()

	fmt.Println("Threads:", params.Threads)
	fmt.Println("Width:", params.ImageWidth)
	fmt.Println("Height:", params.ImageHeight)

	keyPresses := make(chan rune, 10)
	events := make(chan Shared.Event, 1000)

	go Run(params, events, keyPresses)
	SharedSDL.Run(params, events, keyPresses)
	//rpc.Accept(listener)
}
