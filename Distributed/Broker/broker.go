package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/rpc"
	"os"
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

//Number of clients being used to run GoL
const THREADS = 4
const BUFFER = 2
const LIVE = 255

//------------------SETTERS FOR LOCKED GLOBALS-------------------------

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

//The purpose of the broker is to receive an RPC call from the controller
// and receive / start the necessary data processing in the node

//It should hold all the connection information for the nodes, and the
// controller should only contain the connection information for the broker

//It should break up the inputWorld into smaller bits based on the number of
// connected workers and send the different chunks to each worker

//It should receive the processed strips from each worker and reassemble them
// to send back to the controller

type BrokerOperations struct{}

//------------------HELPER FUNCTIONS-------------------------

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

func distributeSliceSizes(p Shared.Params) []int {

	var stripSize = int(math.Ceil(float64(p.ImageHeight / p.Threads)))
	stripSizeList := make([]int, p.Threads) //Each index is the strip size for the specific worker

	if (stripSize*p.Threads)-p.ImageHeight == stripSize {
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

func createStrip(world [][]byte, stripSizeList []int, workerNumber, imageHeight, threads int) [][]byte {
	var topBuffer int
	var endBuffer int
	var startIndex int

	//We initialize the strip
	var strip [][]byte

	//We exploit the fact that every strip size but the last one is the same, so we can just precalculate the currentY
	//coordinate locally
	var normalStripSize = stripSizeList[0]
	currentY := (normalStripSize) * workerNumber

	if workerNumber == 0 { //starting worker
		topBuffer = imageHeight - 1
		startIndex = 0
		endBuffer = stripSizeList[0] //first worker

		strip = append(strip, world[topBuffer])
		strip = append(strip, world[startIndex:endBuffer+1]...)
	} else if workerNumber == threads-1 { //final worker

		topBuffer = currentY - 1
		startIndex = currentY
		endBuffer = 0

		strip = append(strip, world[topBuffer:imageHeight]...)
		strip = append(strip, world[0])
	} else { //middle workers
		topBuffer = currentY - 1
		startIndex = currentY
		endBuffer = currentY + normalStripSize

		strip = append(strip, world[topBuffer:endBuffer+1]...)
	}

	return strip
}

func manager(req Shared.Request, res *Shared.Response, out chan<- [][]byte, clientNum int) {
	//fmt.Println("Chosen client : ", Clients[clientNum])
	var sliceError = Clients[clientNum].Call(Shared.GoLHandler, req, res)
	Shared.HandleError(sliceError)
	out <- res.World
}

func executeWorker(inputWorld [][]byte, workerChannelList []chan [][]byte, stripSizeList []int, imageWidth,
	imageHeight,
	threads,
	workerNumber int, waitGroup *sync.WaitGroup, clientNum int, req Shared.Request, res *Shared.Response) {
	req.World = createStrip(inputWorld, stripSizeList,
		workerNumber, imageHeight, threads)
	req.Parameters.ImageHeight = (stripSizeList[workerNumber]) + BUFFER
	manager(req, res,
		workerChannelList[workerNumber], clientNum)
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

//Breaks up the world and sends it to the workers
func (s *BrokerOperations) GoLManager(req Shared.Request, res *Shared.Response) (err error) {
	var waitGroup sync.WaitGroup
	var workerChannelList = make([]chan [][]byte, THREADS)
	for j := 0; j < THREADS; j++ {
		var workerChannel = make(chan [][]byte, 2)
		workerChannelList[j] = workerChannel
	}
	//time.Sleep(2 * time.Second)
	var stripSizeList = distributeSliceSizes(req.Parameters)

	for i := 0; i < req.Parameters.Turns; i++ {
		//We now do split the input world for each thread accordingly
		for j := 0; j < THREADS; j++ {
			waitGroup.Add(1)
			//We execute the workers concurrently
			go executeWorker(req.World, workerChannelList,
				stripSizeList, req.Parameters.ImageHeight, req.Parameters.ImageWidth, req.Parameters.Threads, j,
				&waitGroup, j, req, res)
		}
		waitGroup.Wait()
		changeCurrentWorld(mergeWorkerStrips(res.World, workerChannelList, stripSizeList))
		changeCurrentTurn(i + 1)
	}
	return
}

func (s *BrokerOperations) BrokerInfo(req Shared.Request, res *Shared.Response) (err error) {
	currentWorld.lock.Lock()
	res.World = currentWorld.world
	res.AliveCells = getAliveCellsCount(currentWorld.world)
	currentWorld.lock.Unlock()

	currentTurn.lock.Lock()
	res.Turns = currentTurn.turn
	currentTurn.lock.Unlock()
	return
}

func (s *BrokerOperations) KYS(request Shared.Request, response *Shared.Response) (err error) {
	for i := 0; i < THREADS; i++ {
		err := Clients[i].Call(Shared.SuicideHandler, request, response)
		if err != nil {
			panic(err)
		}
	}
	defer os.Exit(0)
	return
}

func (s *BrokerOperations) pauseManager(request Shared.Request, response *Shared.Response) (err error) {
	for i := 0; i < THREADS; i++ {
		err := Clients[i].Call(Shared.PauseHandler, request, response)
		if err != nil {
			panic(err)
		}
	}
	return
}

func (s *BrokerOperations) BackgroundManager(request Shared.Request, response *Shared.Response) (err error) {
	for i := 0; i < THREADS; i++ {
		err := Clients[i].Call(Shared.BackgroundHandler, request, response)
		if err != nil {
			panic(err)
		}
	}
	return
}

//------------------SETUP FUNCTIONS-------------------------

//Sets up the clients for the workers/nodes, called from main
//Hard coded for 4 workers, arbitrary ports
func connectToWorkers() {
	fmt.Println("Made it")
	//This should be changed to AWS IPs when implemented beyond local machine
	var clientsPorts = [4]string{"127.0.0.1:8031", "127.0.0.1:8032", "127.0.0.1:8033", "127.0.0.1:8034"}
	var clientsConnections = [4]*rpc.Client{}
	var err error
	for i := 0; i < 4; i++ {
		fmt.Println("Attempting to connect to : ", clientsPorts[i])
		clientsConnections[i], err = rpc.Dial("tcp", clientsPorts[i])
		if err != nil {
			panic(err)
		}
	}
	Clients = clientsConnections
}

//Main sets up a listener to listen for controller
func main() {
	fmt.Println("Doing the stuff")
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	fmt.Println("1")
	rand.Seed(time.Now().UnixNano())
	var registerError = rpc.Register(&BrokerOperations{})
	Shared.HandleError(registerError)
	fmt.Println("2")
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	fmt.Println("3")
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			panic(err)
		}
	}(listener)
	fmt.Println("4")
	go connectToWorkers()
	rpc.Accept(listener)
}
