package main

import (
	"flag"
	"fmt"
	"net/rpc"
	"os"
	"runtime"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
)

type DistributorChannels struct {
	events    chan<- Shared.Event
	ioCommand chan<- IoCommand
	ioIdle    <-chan bool

	ioFilename chan<- string
	ioOutput   chan<- byte
	ioInput    <-chan byte
}

//Main logic where we control all of our AWS nodes. Also controls the ticker and keypress logic as well.
func controller(params Shared.Params, channels DistributorChannels, keyPresses <-chan rune) {
	fmt.Println("Server port: ", params.ServerPort)
	client, dialError := rpc.Dial("tcp", params.ServerPort)
	Shared.HandleError(dialError)

	//Create request response pair
	request, response := createRequestResponsePair(params, channels)
	fmt.Println("Actual address: ", &request.CallAlive)

	//Make a ticker for the updates
	ticker := time.NewTicker(2 * time.Second)
	//Set up the ticker and keypress processes
	go aliveCellsReporter(ticker, channels, client, &request, response)
	go determineKeyPress(client, keyPresses, &request, response, ticker, channels)

	handleCallAndError(client, Shared.GoLHandler, &request, response)
	channels.events <- Shared.FinalTurnComplete{
		CompletedTurns: params.Turns,
		Alive:          calculateAliveCells(response.World)}

	//Shut down the game safely
	defer handleGameShutDown(client, response, params, channels, ticker)
	os.Exit(0)
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
	Shared.Run(params, events, keyPresses)
}
