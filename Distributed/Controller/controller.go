package main

import (
	"flag"
	"fmt"
	"net/rpc"
	"os"
	"runtime"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
)

/*type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}*/

type DistributorChannels struct {
	events    chan<- Shared.Event
	ioCommand chan<- IoCommand
	ioIdle    <-chan bool

	ioFilename chan<- string
	ioOutput   chan<- byte
	ioInput    <-chan byte
}

func aliveCellsReporter(ticker *time.Ticker, c DistributorChannels, client *rpc.Client, request *Shared.Request, response *Shared.Response) {
	c.events <- Shared.AliveCellsCount{CompletedTurns: 0, CellsCount: 0}
	for {
		select {
		//When the ticker triggers, we send an RPC call to return the number of alive cells, and number of turns processed
		case <-ticker.C:
			tickerError := client.Call(Shared.InfoHandler, request, response)
			Shared.HandleError(tickerError)
			c.events <- Shared.AliveCellsCount{response.Turns, response.AliveCells}
			fmt.Println("On turn: ", response.Turns, ", Alive cells: ", response.AliveCells)
		}
	}
}

func determineKeyPress(client *rpc.Client, keyPresses <-chan rune, req *Shared.Request, res *Shared.Response, ticker *time.Ticker, c DistributorChannels) {
	for {
		select {
		case key := <-keyPresses:
			if key == 'k' {
				kError := client.Call(Shared.InfoHandler, req, res)
				Shared.HandleError(kError)

				client.Call(Shared.SuicideHandler, req, res)
				handleGameShutDown(client, res, req.Parameters, c, ticker)
				os.Exit(0)
			} else if key == 's' {
				qError := client.Call(Shared.InfoHandler, req, res)
				Shared.HandleError(qError)
				var filename = strconv.Itoa(req.Parameters.ImageWidth) + "x" + strconv.Itoa(req.Parameters.ImageHeight) + "x" + strconv.
					Itoa(res.Turns)
				writeToFileIO(res.World, req.Parameters, filename, c)
			} else if key == 'p' {
				fmt.Println("Continuing")
				pError := client.Call(Shared.PauseHandler, req, res)
				Shared.HandleError(pError)
			} else if key == 'q' {
				qError := client.Call(Shared.BackgroundHandler, req, res)
				Shared.HandleError(qError)

				kError := client.Call(Shared.InfoHandler, req, res)
				Shared.HandleError(kError)

				ticker.Stop()
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				defer handleCloseClient(client)
				os.Exit(0)
			}
		}
	}
}

func controller(params Shared.Params, channels DistributorChannels, keyPresses <-chan rune) {
	fmt.Println("Server port: ", params.ServerPort)
	client, dialError := rpc.Dial("tcp", params.ServerPort)
	Shared.HandleError(dialError)

	//Create request response pair
	request, response := createRequestResponsePair(params, channels)
	fmt.Println("Actual address: ", &request.CallAlive)

	//Make a ticker for the updates
	ticker := time.NewTicker(2 * time.Second)
	go aliveCellsReporter(ticker, channels, client, &request, response)

	go determineKeyPress(client, keyPresses, &request, response, ticker, channels)

	callError := client.Call(Shared.GoLHandler, &request, response)
	Shared.HandleError(callError)

	channels.events <- Shared.FinalTurnComplete{
		CompletedTurns: params.Turns,
		Alive:          calculateAliveCells(response.World)}

	//Shut down the game safely
	fmt.Println("We here")
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
