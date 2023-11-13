package Controller

import (
	"flag"
	"fmt"
	"net/rpc"
	"runtime"
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

func aliveCellsReporter(ticker *time.Ticker, c DistributorChannels, client *rpc.Client, request Shared.Request, response *Shared.Response) {
	c.events <- Shared.AliveCellsCount{CompletedTurns: 0, CellsCount: 0}
	for {
		select {
		//When the ticker triggers, we send an RPC call to return the number of alive cells, and number of turns processed
		case <-ticker.C:
			fmt.Println("Sending call")
			tickerError := client.Call(Shared.TickersHandler, request, response)
			Shared.HandleError(tickerError)
			c.events <- Shared.AliveCellsCount{response.Turns, response.AliveCells}
		}
	}
}

func controller(params Shared.Params, channels DistributorChannels, keyPresses <-chan rune) {
	fmt.Println("Serverport: ", params.ServerPort)
	client, dialError := rpc.Dial("tcp", params.ServerPort)
	Shared.HandleError(dialError)

	//Create request response pair
	request, response := createRequestResponsePair(params, channels)

	//Make a ticker for the updates
	ticker := time.NewTicker(2 * time.Second)
	go aliveCellsReporter(ticker, channels, client, request, response)

	callError := client.Call(Shared.GoLHandler, request, response)
	Shared.HandleError(callError)

	channels.events <- Shared.FinalTurnComplete{
		CompletedTurns: params.Turns,
		Alive:          calculateAliveCells(response.World)}

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
		10000000000,
		"Specify the number of turns to process. Defaults to 10000000000.")

	flag.Parse()

	fmt.Println("Threads:", params.Threads)
	fmt.Println("Width:", params.ImageWidth)
	fmt.Println("Height:", params.ImageHeight)

	keyPresses := make(chan rune, 10)
	events := make(chan Shared.Event, 1000)

	Run(params, events, keyPresses)

}
