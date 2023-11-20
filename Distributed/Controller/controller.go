package main

import (
	"flag"
	"fmt"
	"runtime"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
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

type ControllerOperations struct{}

func (s *ControllerOperations) CellsReporter(req Shared.Request, res *Shared.Response) (err error) {
	for i := 0; i < req.Parameters.ImageHeight; i++ {
		for j := 0; j < req.Parameters.ImageWidth; j++ {
			//If the cell has changed since the last iteration, we need to send an event to say so
			if req.OldWorld[i][j] != req.World[i][j] {
				Channels.events <- Shared.CellFlipped{CompletedTurns: req.Turn, Cell: util.Cell{X: j, Y: i}}
			}
		}
	}
	Channels.events <- Shared.TurnComplete{req.Turn}
	return
}

func flipWorldCellsInitial(world [][]byte, imageHeight, imageWidth, turn int, c DistributorChannels) {
	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			if world[i][j] == LIVE {
				fmt.Println("Cell flipped")
				c.events <- Shared.CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: j, Y: i}}
			}
		}
	}
}

//Main logic where we control all of our AWS nodes. Also controls the ticker and keypress logic as well.
func controller(params Shared.Params, channels DistributorChannels, keyPresses <-chan rune) {
	fmt.Println("Server port: ", params.ServerPort)
	var client = Shared.HandleCreateClientAndError(params.ServerPort)
	fmt.Println(client)
	Channels = channels

	//Create request response pair
	request, response := createRequestResponsePair(params, channels)
	fmt.Println("Actual address: ", &request.CallAlive)

	//Make a ticker for the updates
	ticker := time.NewTicker(2 * time.Second)
	//Set up the ticker and keypress processes
	go aliveCellsReporter(ticker, channels, client, &request, response)
	go determineKeyPress(client, keyPresses, &request, response, ticker, channels)

	//We set up our broker
	fmt.Println("Sending a call")
	flipWorldCellsInitial(request.World, request.Parameters.ImageHeight, request.Parameters.ImageWidth, 0, channels)
	//channels.events <- Shared.TurnComplete{}
	Shared.HandleCallAndError(client, Shared.BrokerHandler, &request, response)
	channels.events <- Shared.FinalTurnComplete{
		CompletedTurns: params.Turns,
		Alive:          calculateAliveCells(response.World)}
	fmt.Println("Shutting down")
	//Shut down the game safely
	defer handleGameShutDown(client, response, params, channels, ticker)
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

	/*listener, _ := net.Listen("tcp", ":8035")
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(listener)*/

	go Run(params, events, keyPresses)
	Shared.Run(params, events, keyPresses)
	//rpc.Accept(listener)
}
