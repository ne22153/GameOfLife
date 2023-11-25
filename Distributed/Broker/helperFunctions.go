package main

import (
	"fmt"
	"math"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
	"uk.ac.bris.cs/gameoflife/util"
)

func createRequestResponsePair(p Shared.Params, events chan<- Shared.Event) (Shared.Request, *Shared.Response) {

	//Forms the request which contains the [][]byte version of the PGM file
	request := Shared.Request{
		World:       getCurrentWorld(),
		Parameters:  p,
		Events:      events,
		CurrentTurn: make(chan int, 1),
		CallAlive:   make(chan int, 1),
		GetAlive:    make(chan int, 1),
		GetTurn:     make(chan int, 1),
		Paused:      !getPaused()}
	//There doesn't exist a response, but we will create a new one
	response := new(Shared.Response)

	return request, response
}

//Helper function of distributor
//We merge worker strips into one world [][]byte (we also remove buffers from each worker as well)
func mergeWorkerStrips(newWorld [][]byte, workerChannelList []chan [][]byte, stripSizeList []int) [][]byte {
	for i := 0; i < len(workerChannelList); i++ {
		//worldSection is just a game slice from a specific worker
		worldSection := <-(workerChannelList[i])
		endBufferIndex := stripSizeList[i] + 1
		//We don't add the top and end buffers (that's what the inner loop's doing)
		newWorld = append(newWorld, worldSection[1:endBufferIndex]...)
	}

	return newWorld
}

//Determine how big the slice of the GoL board that the worker will work on.
//Return list of slice sizes
func distributeSliceSizes(p Shared.Params) []int {

	var stripSize = int(math.Ceil(float64(p.ImageHeight / WORKERS)))
	stripSizeList := make([]int, WORKERS) //Each index is the strip size for the specific worker

	if (stripSize*WORKERS)-p.ImageHeight == stripSize {
		stripSize--
	}

	sum := 0
	for i := range stripSizeList {
		stripSizeList[i] = stripSize
		sum += stripSize
	}

	//We adjust the final worker's slice size to fit to the pixels
	if sum > p.ImageHeight { //if sum is more than height
		difference := sum - p.ImageHeight
		stripSizeList[len(stripSizeList)-1] -= difference
	} else if p.ImageHeight > sum { //if sum is less the same as height
		difference := p.ImageHeight - sum
		stripSizeList[len(stripSizeList)-1] += difference
	}

	return stripSizeList
}

// creates the strip that the worker will operate on
// currentHeight is pass by reference so that it will update for the next worker
func createStrip(world [][]byte, stripSize int, workerNumber, imageHeight int) [][]byte {
	//fmt.Println(stripSizeList)
	//fmt.Println(workerNumber)
	var topBuffer int
	var endBuffer int
	var startIndex int

	//We initialize the strip
	var strip [][]byte

	//We exploit the fact that every strip size but the last one is the same, so we can just precalculate the currentY
	//coordinate locally
	currentY := (stripSize) * workerNumber

	if workerNumber == 0 { //starting worker
		topBuffer = imageHeight - 1
		startIndex = 0
		endBuffer = stripSize //first worker

		strip = append(strip, world[topBuffer])
		strip = append(strip, world[startIndex:endBuffer+1]...)
	} else if workerNumber == WORKERS-1 { //final worker
		topBuffer = currentY - 1
		startIndex = currentY
		endBuffer = 0

		strip = append(strip, world[topBuffer:imageHeight]...)
		strip = append(strip, world[0])
	} else { //middle workers
		topBuffer = currentY - 1
		startIndex = currentY
		endBuffer = currentY + stripSize

		strip = append(strip, world[topBuffer:endBuffer+1]...)
	}

	return strip
}

func getAliveCellsCount(inputWorld [][]byte) int {
	aliveCells := 0

	for _, row := range inputWorld {
		for _, tile := range row {
			if tile == LIVE {
				aliveCells++
			}
		}
	}

	return aliveCells
}

func manager(req Shared.Request, res *Shared.Response, out chan<- [][]byte, clientNum int, brokerRes *Shared.Response) [][]byte {
	var j int = HandleCallAndError(Clients[clientNum], Shared.GoLHandler, &req, res, clientNum, brokerRes)

	if j == 1 {
		fmt.Println("We messed up in the manager - IDK Whtat to do fr fr")
	}
	//For some reason the response differs from within the call and out of the call
	//The difference seems to be random every call, so perhaps issues with response access?

	if req.Parameters.ImageWidth == 16 && req.Parameters.Turns == 1 {
		fmt.Println("\n", clientNum+1, res.World)
	}
	return res.World
}

//Helper function of distributor
//Creates a strip for the worker and then the worker will perform GoL algorithm on such strip
func executeWorker(inputWorld [][]byte, workerChannelList []chan [][]byte, stripSize int, imageWidth,
	imageHeight,
	workerNumber int, waitGroup *sync.WaitGroup, req Shared.Request, res *Shared.Response, brokerRes *Shared.Response) {
	req.World = createStrip(inputWorld, stripSize,
		workerNumber, imageHeight)
	req.Parameters.ImageHeight = (stripSize) + BUFFER
	workerChannelList[workerNumber] <- manager(req, res,
		workerChannelList[workerNumber], workerNumber, brokerRes)
	defer (*waitGroup).Done()
}

func reportToController(p Shared.Params, events chan<- Shared.Event, oldWorld [][]byte, newWorld [][]byte) {
	request, response := createRequestResponsePair(p, events)
	request.OldWorld = oldWorld
	request.World = newWorld
	request.Turn = getCurrentTurn()
	Shared.HandleCallAndError(controller, Shared.ControllerHandler, &request, response)
}

func HandleCreateClientAndError(serverPort string) *rpc.Client {
	//Initial connection attempt
	client, dialError := rpc.Dial("tcp", serverPort)

	fmt.Println("dialerror: ", dialError)

pingLoop:
	//Iterative solution
	for {
		//Busy waiting with 250ms ping
		time.Sleep(250 * time.Millisecond)
		fmt.Println("250ms ping")

		//Reattempt
		client, dialError = rpc.Dial("tcp", serverPort)
		if dialError == nil {
			break pingLoop
		}
	}
	//if dialError != nil {
	//
	//	time.Sleep(250 * time.Millisecond)
	//	fmt.Println("Ping")
	//	client = HandleCreateClientAndError(serverPort)
	//}
	fmt.Println("Client created: ", client)
	return client
}

func HandleCallAndError(client *rpc.Client, namedFunctionHandler string,
	request *Shared.Request, response *Shared.Response,
	clientNum int, brokerRes *Shared.Response) int {
	var namedFunctionHandlerError error = client.Call(namedFunctionHandler, request, response)

	//fmt.Println("error: ", namedFunctionHandlerError)
	if namedFunctionHandlerError != nil {

		//Handle all other threads
		for i := 0; i < WORKERS; i++ {
			if i != clientNum {
				request.Paused = true
				HandleCallAndError(Clients[i], Shared.PauseHandler, request, response, clientNum, brokerRes)
			}
		}
		client := HandleCreateClientAndError(clientsPorts[clientNum])

		Clients[clientNum] = client
		brokerRes.Resend = true
		response.Resend = true

		fmt.Println("client being weird? :", client)
		fmt.Println("Clients as list: ", Clients)
		fmt.Println("broker response resend value: ", brokerRes.Resend)

		time.Sleep(500 * time.Millisecond)

		return 1

		//If nothing needs to be resent (no disconnections)
	} else {
		brokerRes.Resend = false
	}
	return 0
}

func flipWorldCellsIteration(oldWorld, newWorld [][]byte, turn, imageHeight, imageWidth int) []util.Cell {
	var flippedCells []util.Cell
	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			//If the cell has changed since the last iteration, we need to send an event to say so
			if oldWorld[i][j] != newWorld[i][j] {
				flippedCells = append(flippedCells, util.Cell{X: i, Y: j})
			}
		}
	}
	return flippedCells
}
