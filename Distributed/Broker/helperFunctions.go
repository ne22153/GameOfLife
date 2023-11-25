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
		World:      getCurrentWorld(),
		Parameters: p,
		Events:     events,
		Paused:     !getPaused()}
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

func manager(req Shared.Request, res *Shared.Response, out chan<- [][]byte, clientNum int, brokerRes *Shared.Response) [][]byte {
	Clients.lock.Lock()
	Clients.owner = "manager"
	var errorValue int = HandleCallAndError(Clients.clients[clientNum], Shared.GoLHandler, &req, res, clientNum, brokerRes)
	Clients.lock.Unlock()

	if errorValue != 0 {
		fmt.Println("world:", res.World)
		brokerRes.Resend = true
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
	defer func() {
		(*waitGroup).Done()
		//fmt.Println("Completed the goroutine")
	}()
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

func HandleCreateClientAndError(serverPort string) *rpc.Client {
	client, dialError := rpc.Dial("tcp", serverPort)

	//Busy wait every 500ms
	for {
		if dialError != nil {
			time.Sleep(500 * time.Millisecond)
			client, dialError = rpc.Dial("tcp", serverPort)
			fmt.Println("Bryuh")
		} else {
			//If there is no error, we can break out
			break
		}

	}

	//if dialError != nil {
	//	time.Sleep(1 * time.Second)
	//	client = HandleCreateClientAndError(serverPort)
	//}

	return client
}

func HandleCallAndError(client *rpc.Client, namedFunctionHandler string,
	request *Shared.Request, response *Shared.Response, clientNum int, brokerRes *Shared.Response) int {
	var namedFunctionHandlerError = client.Call(namedFunctionHandler, request, response)
	if namedFunctionHandlerError != nil {
		for i := 0; i < WORKERS; i++ {
			if i != clientNum {
				request.Paused = true
				Clients.lock.Lock()
				Clients.owner = "Call and Error Inner"
				fmt.Println("Going in")
				HandleCallAndError(Clients.clients[i], Shared.PauseHandler, request, response, clientNum, brokerRes)
				Clients.lock.Unlock()
			}
		}
		client := HandleCreateClientAndError(clientsPorts[clientNum])
		Clients.lock.Lock()
		Clients.owner = "Call and Error outer"
		Clients.clients[clientNum] = client
		Clients.lock.Unlock()
		brokerRes.Resend = true
		return 1
	} else {
		brokerRes.Resend = false
	}
	return 0
}

func flipWorldCellsIteration(oldWorld, newWorld [][]byte, imageHeight, imageWidth int) []util.Cell {
	var flippedCells []util.Cell
	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			//If the cell has changed since the last iteration, we need to send an event to say so
			/*if oldWorld[i][j] != newWorld[i][j] {
				flippedCells = append(flippedCells, util.Cell{X: i, Y: j})
			}*/
			if oldWorld[i][j] == LIVE {
				flippedCells = append(flippedCells, util.Cell{X: i, Y: j})
			}
			if newWorld[i][j] == LIVE {
				flippedCells = append(flippedCells, util.Cell{X: i, Y: j})
			}
		}
	}
	return flippedCells
}
