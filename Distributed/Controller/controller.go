package Controller

import (
	"flag"
	"fmt"
	"net/rpc"
	"runtime"
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

func controller(params Shared.Params, channels DistributorChannels, keyPresses <-chan rune) {
	fmt.Println(params.ServerPort)

	fmt.Println("Serverport", params.ServerPort)
	client, dialError := rpc.Dial("tcp", params.ServerPort)
	Shared.HandleError(dialError)
	defer func(client *rpc.Client) {
		closeError := client.Close()
		Shared.HandleError(closeError)

	}(client)

	//Forms the request which contains the [][]byte version of the PGM file
	request := Shared.Request{World: WriteFromFileIO(params.ImageHeight, params.ImageWidth, channels, params.Turns), Parameters: params}
	response := new(Shared.Response)
	callError := client.Call(Shared.GoLHandler, request, response)
	Shared.HandleError(callError)

	fmt.Println("Responded: ", response.World)
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
