package main

import (
	"fmt"
	"math"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
	"uk.ac.bris.cs/gameoflife/util"
)

// Helper function of GoLManager
// initializeWorkerStates returns a (int, bool) pair of the turn and a flag determining if a goto back to
//start of the function is needed
// bool = true if there already exists a game state
// bool = false if there is no game state -> make a fresh game state
func initializeWorkerStates(request *Shared.Request, response *Shared.Response, brokerResponse *Shared.Response,
	turn int) (int, bool) {
	var restartFlag bool = false

	//If paused.pause is true, this implies that there was a pre-existing game state either due to pausing or
	//disconnection from an AWS node. In this case, simply restart whatever state the game was previously in.
	if paused.pause {
		paused.pause = !paused.pause
		paused.lock.Unlock()
		for i := 0; i < WORKERS; i++ {
			request.Paused = false
			Clients.lock.Lock()
			Clients.owner = "Broker manager"
			fmt.Println("CLAIMED by", Clients.owner)
			j := HandleCallAndError(Clients.clients[i], Shared.PauseHandler, request, response, i, brokerResponse)
			Clients.lock.Unlock()
			if j != 0 {
				restartFlag = true
			}
		}

		turn = getCurrentTurn()

		//Otherwise, start the game anew.
	} else {
		turn = 0
		changeCurrentWorld(request.World)
	}

	return turn, restartFlag

}

// Helper function for GoLManager
// will check if broker response is true, then it will set the restart flag to true
func checkForResend(response *Shared.Response, turnNum int) bool {

	var restartFlag bool = false
	if !response.Resend {
		var newWorld = mergeWorkerStrips(response.World, workerChannelList, stripSizeList)
		changeCurrentTurn(turnNum + 1)
		changeCurrentWorld(newWorld)
	} else {
		changePaused()
		paused.lock.Lock()
		restartFlag = true
	}

	return restartFlag
}

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
	temp := Clients.clients[clientNum]
	Clients.lock.Unlock()
	/*Clients.owner = "manager"
	fmt.Println("CLAIMED by", Clients.owner, clientNum+1)*/
	var errorValue int = HandleCallAndError(temp, Shared.GoLHandler, &req, res, clientNum, brokerRes)

	if errorValue != 0 {
		fmt.Println("world:", res.World)
		fmt.Println("Detected reconnection.")
	} else {
		fmt.Println("Released by manager ", clientNum+1)
	}

	return res.World
}

//Helper function of distributor
//Creates a strip for the worker and then the worker will perform GoL algorithm on such strip
func executeWorker(inputWorld [][]byte, workerChannelList []chan [][]byte, stripSize int, imageWidth,
	imageHeight,
	workerNumber int, waitGroup *waitgroupDebug, req Shared.Request, res *Shared.Response,
	brokerRes *Shared.Response) {

	req.World = createStrip(inputWorld, stripSize,
		workerNumber, imageHeight)
	req.Parameters.ImageHeight = (stripSize) + BUFFER
	workerChannelList[workerNumber] <- manager(req, res,
		workerChannelList[workerNumber], workerNumber, brokerRes)

	defer func() {
		(*waitGroup).waitGroup.Done()
		(*waitGroup).count--

		if brokerRes.Resend {
			fmt.Println("waitgroup after done: ", (*waitGroup).count)
			fmt.Println("Completed the goroutine")
			fmt.Println("will resend!")
		}

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
			fmt.Println("ping: waiting for node to reconnect")

			client, dialError = rpc.Dial("tcp", serverPort)
		} else {
			//If there is no error, we can break out
			break
		}

	}

	return client
}

func HandleCallAndError(client *rpc.Client, namedFunctionHandler string,
	request *Shared.Request, response *Shared.Response, clientNum int, brokerRes *Shared.Response) int {
	if namedFunctionHandler == Shared.GoLHandler {
		fmt.Println("Sending to worker ", clientNum+1)
	}
	var namedFunctionHandlerError error

	//This will essentially busy wait until a client is reconnected. Instead of handling the error, simply run it
	//continously until there is no error
	for {

		namedFunctionHandlerError = client.Call(namedFunctionHandler, request, response)
		fmt.Println(namedFunctionHandlerError)
		//Escape hatch!
		if namedFunctionHandlerError == nil {
			break
		}

		client = HandleCreateClientAndError(clientsPorts[clientNum])

		Clients.lock.Lock()
		Clients.owner = "Call and Error outer"
		fmt.Println("CLAIMED by", Clients.owner)
		Clients.clients[clientNum] = client
		Clients.lock.Unlock()
	}

	//Since we know the error is definitely nil (otherwise it won't break the for we don't need to resend anything
	//anymore
	brokerRes.Resend = false
	fmt.Println("Finishing worker ", clientNum+1)

	return 0
}

func flipWorldCellsIteration(oldWorld, newWorld [][]byte, imageHeight, imageWidth int) []util.Cell {
	var flippedCells []util.Cell
	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			if oldWorld[i][j] == LIVE {
				flippedCells = append(flippedCells, util.Cell{X: j, Y: i})
			}
			if newWorld[i][j] == LIVE {
				flippedCells = append(flippedCells, util.Cell{X: j, Y: i})
			}
		}
	}
	return flippedCells
}
