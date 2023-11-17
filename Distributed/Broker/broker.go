package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
)

//------------------GLOBAL VARIABLES AND APPLICABLE STRUCTS-------------------------

var Clients [4]*rpc.Client

type currentTurnStruct struct {
	turn int
	lock sync.Mutex
}

var currentTurn currentTurnStruct

type currentWorldStruct struct {
	world [][]byte
	lock  sync.Mutex
}

var currentWorld currentWorldStruct

//------------------CONSTANTS-------------------------

// WORKERS - Number of clients being used to run GoL
const WORKERS = 4
const BUFFER = 2
const LIVE = 255

//------------------GETTERS AND SETTERS FOR LOCKED GLOBALS-------------------------

func changeCurrentTurn(input int) {
	currentTurn.lock.Lock()
	currentTurn.turn = input
	currentTurn.lock.Unlock()
}

func changeCurrentWorld(input [][]byte) {
	currentWorld.lock.Lock()
	currentWorld.world = input
	currentWorld.lock.Unlock()
}

func getCurrentWorld() [][]byte {
	currentWorld.lock.Lock()
	var temp = currentWorld.world
	currentWorld.lock.Unlock()
	return temp
}

func getCurrentTurn() int {
	currentTurn.lock.Lock()
	var temp = currentTurn.turn
	currentTurn.lock.Unlock()
	return temp
}

type BrokerOperations struct{}

//------------------HELPER FUNCTIONS-------------------------

func mergeWorkerStrips(newWorld [][]byte, workerChannelList []chan [][]byte, stripSizeList []int, turns int) [][]byte {
	for i := 0; i < len(workerChannelList); i++ {
		//worldSection is just a game slice from a specific worker
		worldSection := <-(workerChannelList[i])
		endBufferIndex := stripSizeList[i] + 1
		//We don't add the top and end buffers (that's what the inner loop's doing)
		newWorld = append(newWorld, worldSection[1:endBufferIndex]...)
	}

	return newWorld
}

func distributeSliceSizes(p Shared.Params) []int {

	var stripSize = int(math.Ceil(float64(p.ImageHeight / WORKERS)))
	//Each index is the strip size for the specific worker
	stripSizeList := make([]int, WORKERS)

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

func createStrip(world [][]byte, stripSize int, workerNumber, imageHeight int) [][]byte {
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

func manager(req Shared.Request, res *Shared.Response, out chan<- [][]byte, clientNum int) [][]byte {
	Shared.HandleCallAndError(Clients[clientNum], Shared.GoLHandler, &req, res)

	return res.World
}

func executeWorker(inputWorld [][]byte, workerChannelList []chan [][]byte, stripSize int, imageWidth,
	imageHeight,
	workerNumber int, waitGroup *sync.WaitGroup, req Shared.Request, res *Shared.Response) {
	req.World = createStrip(inputWorld, stripSize,
		workerNumber, imageHeight)
	req.Parameters.ImageHeight = (stripSize) + BUFFER
	workerChannelList[workerNumber] <- manager(req, res,
		workerChannelList[workerNumber], workerNumber)
	defer (*waitGroup).Done()
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

//------------------INCOMING RPC CALLS-------------------------

// GoLManager Breaks up the world and sends it to the workers
func (s *BrokerOperations) GoLManager(req Shared.Request, res *Shared.Response) (err error) {
	var waitGroup sync.WaitGroup
	var workerChannelList = make([]chan [][]byte, WORKERS)
	for j := 0; j < WORKERS; j++ {
		var workerChannel = make(chan [][]byte, 2)
		workerChannelList[j] = workerChannel
	}
	//time.Sleep(2 * time.Second)
	var stripSizeList = distributeSliceSizes(req.Parameters)
	changeCurrentWorld(req.World)
	for i := 0; i < req.Parameters.Turns; i++ {
		//We now do split the input world for each thread accordingly
		for j := 0; j < WORKERS; j++ {
			waitGroup.Add(1)
			//We execute the workers concurrently
			var request, response = createRequestResponsePair(req.Parameters, req.Events)
			request.World = getCurrentWorld()
			go executeWorker(request.World, workerChannelList,
				stripSizeList[j], req.Parameters.ImageHeight, req.Parameters.ImageWidth, j,
				&waitGroup, request, response)
		}
		waitGroup.Wait()
		changeCurrentWorld(mergeWorkerStrips(res.World, workerChannelList, stripSizeList, req.Parameters.Turns))
		changeCurrentTurn(i + 1)
	}
	res.World = getCurrentWorld()
	return
}

func (s *BrokerOperations) BrokerInfo(req Shared.Request, res *Shared.Response) (err error) {
	currentWorld.lock.Lock()
	res.World = currentWorld.world
	res.AliveCells = getAliveCellsCount(currentWorld.world)
	currentWorld.lock.Unlock()

	res.Turns = getCurrentTurn()
	return
}

func (s *BrokerOperations) KYS(request Shared.Request, response *Shared.Response) (err error) {
	for i := 0; i < WORKERS; i++ {
		fmt.Println("Killing it", i)
		go Shared.HandleCallAndError(Clients[i], Shared.SuicideHandler, &request, response)
		fmt.Println("Killed it", i)
	}
	//defer os.Exit(0)
	return
}

func (s *BrokerOperations) PauseManager(request Shared.Request, response *Shared.Response) (err error) {
	for i := 0; i < WORKERS; i++ {
		fmt.Println("pausing it:", i)
		Shared.HandleCallAndError(Clients[i], Shared.PauseHandler, &request, response)
		fmt.Println("paused it:", i)
	}
	fmt.Println()
	return
}

func (s *BrokerOperations) BackgroundManager(request Shared.Request, response *Shared.Response) (err error) {
	for i := 0; i < WORKERS; i++ {
		Shared.HandleCallAndError(Clients[i], Shared.PauseHandler, &request, response)
	}
	return
}

//------------------SETUP FUNCTIONS-------------------------

//Sets up the clients for the workers/nodes, called from main
//Hard coded for 4 workers, arbitrary ports
func connectToWorkers() {
	//This should be changed to AWS IPs when implemented beyond local machine
	var clientsPorts = [4]string{"127.0.0.1:8031", "127.0.0.1:8032", "127.0.0.1:8033", "127.0.0.1:8034"}
	var clientsConnections = [4]*rpc.Client{}

	for i := 0; i < 4; i++ {
		fmt.Println("Attempting to connect to : ", clientsPorts[i])
		clientsConnections[i] = Shared.HandleCreateClientAndError(clientsPorts[i])
	}

	Clients = clientsConnections
}

//Main sets up a listener to listen for controller
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	Shared.HandleRegisterAndError(&BrokerOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(listener)
	go connectToWorkers()
	rpc.Accept(listener)
}
