package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
)

//------------------GLOBAL VARIABLES AND APPLICABLE STRUCTS-------------------------

//The client struct contains an owner : string in order to be used for debugging purposes.
type clientStruct struct {
	clients [4]*rpc.Client
	owner   string
	lock    sync.Mutex
}

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

type pauseStruct struct {
	pause bool
	lock  sync.Mutex
}

type waitgroupDebug struct {
	waitGroup sync.WaitGroup
	count     int
}

var Clients clientStruct

var paused pauseStruct

var workerChannelList = make([]chan [][]byte, WORKERS)

var stripSizeList []int

var clientsPorts [4]string

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

func changePaused() {
	paused.lock.Lock()
	paused.pause = !paused.pause
	paused.lock.Unlock()
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

func getPaused() bool {
	paused.lock.Lock()
	var temp = paused.pause
	paused.lock.Unlock()
	return temp
}

type BrokerOperations struct{}

//------------------INCOMING RPC CALLS-------------------------

// GoLManager Breaks up the world and sends it to the workers
func (s *BrokerOperations) GoLManager(req Shared.Request, res *Shared.Response) (err error) {
	var waitGroup waitgroupDebug = waitgroupDebug{count: 0}
	//var waitGroup sync.WaitGroup
	var turn int

	turn = initializeWorkerStates(&req, res, res, turn)

	for j := 0; j < WORKERS; j++ {
		var workerChannel = make(chan [][]byte, 2)
		workerChannelList[j] = workerChannel
	}
	stripSizeList = distributeSliceSizes(req.Parameters)
	for i := turn; i < req.Parameters.Turns; i++ {
		//We now do split the input world for each thread accordingly
		for j := 0; j < WORKERS; j++ {

			waitGroup.waitGroup.Add(1)
			waitGroup.count++
			fmt.Println(waitGroup.count)
			//We execute the workers concurrently
			var request, response = createRequestResponsePair(req.Parameters, req.Events)
			request.World = getCurrentWorld()

			go func(workernumber int) {
				executeWorker(request.World, workerChannelList,
					stripSizeList[workernumber], req.Parameters.ImageHeight, req.Parameters.ImageWidth, workernumber,
					&waitGroup, request, response, res)
			}(j)
		}
		waitGroup.waitGroup.Wait()

		var newWorld = mergeWorkerStrips(res.World, workerChannelList, stripSizeList)
		changeCurrentTurn(i + 1)
		changeCurrentWorld(newWorld)

		paused.lock.Lock()
		paused.lock.Unlock()
	}
	res.World = getCurrentWorld()
	return
}

func (s *BrokerOperations) BrokerInfo(req Shared.Request, res *Shared.Response) (err error) {
	currentWorld.lock.Lock()
	currentTurn.lock.Lock()

	res.World = currentWorld.world
	res.AliveCells = getAliveCellsCount(currentWorld.world)
	res.FlippedCells = flipWorldCellsIteration(req.World, currentWorld.world, req.Parameters.ImageHeight, req.Parameters.ImageWidth)
	res.Turns = currentTurn.turn

	currentWorld.lock.Unlock()
	currentTurn.lock.Unlock()
	return
}

// KYS :Handler whenever the user presses "K".
//Called from the local controller to tell the AWS node to kill itself
func (s *BrokerOperations) KYS(request Shared.Request, response *Shared.Response) (err error) {
	for i := 0; i < WORKERS; i++ {
		i := i
		Clients.lock.Lock()
		Clients.owner = "KYS"
		fmt.Println("CLAIMED by", Clients.owner)
		go func() { HandleCallAndError(Clients.clients[i], Shared.SuicideHandler, &request, response, i, response) }()
		Clients.lock.Unlock()
	}
	time.Sleep(1 * time.Second)
	fmt.Println("Terminated Sucessfully")
	os.Exit(0)
	return
}

// PauseManager :Handler whenever the user presses "p". Transmits Pause commands to Workers
//If already paused then unpause, otherwise pause.
func (s *BrokerOperations) PauseManager(request Shared.Request, response *Shared.Response) (err error) {
	for i := 0; i < WORKERS; i++ {
		i := i
		request.Paused = !getPaused()
		Clients.lock.Lock()
		Clients.owner = "Broker Pause"
		fmt.Println("CLAIMED by", Clients.owner)
		go func() { HandleCallAndError(Clients.clients[i], Shared.PauseHandler, &request, response, i, response) }()
		Clients.lock.Unlock()
	}
	changePaused()
	return
}

// BackgroundManager :Handler whenever the user presses "q"
//	When the local controller is killed, then pause the node and then wait until a new local controller is created
// This is a form of fault tolerance.
func (s *BrokerOperations) BackgroundManager(request Shared.Request, response *Shared.Response) (err error) {
	for i := 0; i < WORKERS; i++ {
		i := i
		Clients.lock.Lock()
		Clients.owner = "Broker Background"
		fmt.Println("CLAIMED by", Clients.owner)
		go func() { HandleCallAndError(Clients.clients[i], Shared.PauseHandler, &request, response, i, response) }()
		Clients.lock.Unlock()
	}
	changePaused()
	paused.lock.Lock()
	return
}

//------------------SETUP FUNCTIONS-------------------------

//Sets up the clients for the workers/nodes, called from main
//Hard coded for 4 workers, arbitrary ports
func connectToWorkers() {
	//This should be changed to AWS IPs when implemented beyond local machine
	//clientsPorts = [4]string{"3.87.90.137:8030", "54.196.166.51:8030", "54.90.104.152:8030", "3.91.255.247:8030"}
	clientsPorts = [4]string{"127.0.0.1:8031", "127.0.0.1:8032", "127.0.0.1:8033", "127.0.0.1:8034"}
	var clientsConnections [4]*rpc.Client

	//Initialize our clients
	for i := 0; i < len(clientsPorts); i++ {
		fmt.Println("Attempting to connect to : ", clientsPorts[i])
		clientsConnections[i] = Shared.HandleCreateClientAndError(clientsPorts[i])
	}
	Clients.lock.Lock()
	Clients.owner = "Setup"
	fmt.Println("CLAIMED by", Clients.owner)
	Clients.clients = clientsConnections
	Clients.lock.Unlock()
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
	//controller = Shared.HandleCreateClientAndError("127.0.0.1:8035")

	rpc.Accept(listener)
}
