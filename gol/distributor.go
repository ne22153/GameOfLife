package gol

import (
	"math"
	"os"
	"strconv"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/util"
)

// LIVE We make constants as stand-ins for alive or dead
const LIVE = 255
const DEAD = 0

// BUFFER We make a const for a buffer size stand in
const BUFFER = 2

var worldLock sync.Mutex
var turnLock sync.Mutex

type distributorChannels struct {
	events    chan<- Event
	ioCommand chan<- ioCommand
	ioIdle    <-chan bool

	ioFilename chan<- string
	ioOutput   chan<- byte
	ioInput    <-chan byte
}

//Helper function to distributor to find the number of alive cells adjacent to the tile
func calculateAliveCells(world [][]byte) []util.Cell {
	var coordinates []util.Cell
	worldLock.Lock()
	rows := world

	for index, row := range rows {
		for index2 := range row {
			if world[index][index2] > 0 {
				coordinates = append(coordinates, util.Cell{X: index2, Y: index})
			}
		}
	}
	worldLock.Unlock()
	return coordinates
}

//Determine how big the slice of the GoL board that the worker will work on.
//Return list of slice sizes
func distributeSliceSizes(p Params) []int {

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

// creates the strip that the worker will operate on
// currentHeight is pass by reference so that it will update for the next worker
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

func manager(imageHeight int, imageWidth int, inputWorld [][]byte, out chan<- [][]byte) {
	gameSlice := worker(imageHeight, imageWidth, inputWorld)
	out <- gameSlice
}

//Helper function of distributor
//Creates a strip for the worker and then the worker will perform GoL algorithm on such strip
func executeWorker(inputWorld [][]byte, workerChannelList []chan [][]byte, stripSizeList []int, imageWidth,
	imageHeight,
	threads,
	workerNumber int, waitGroup *sync.WaitGroup) {
	var strip = createStrip(inputWorld, stripSizeList,
		workerNumber, imageHeight, threads)
	var workerStripSize = (stripSizeList[workerNumber]) + BUFFER
	manager(workerStripSize, imageWidth, strip,
		workerChannelList[workerNumber])
	defer (*waitGroup).Done()
}

func getAliveCellsCount(inputWorld [][]byte) int {
	aliveCells := 0

	worldLock.Lock()
	turnLock.Lock()
	rows := inputWorld

	for _, row := range rows {
		for _, tile := range row {
			if tile == LIVE {
				aliveCells++
			}
		}
	}
	turnLock.Unlock()
	worldLock.Unlock()
	return aliveCells
}

//Manages the key press interrupts
func goPressTrack(inputWorld [][]byte, keyPresses <-chan rune, c distributorChannels, p Params, turn chan int,
	aliveCellsTicker *time.Ticker, pauseChannel chan bool) {
	var turns = 0
	var paused = false
	for {
		select {
		case key := <-keyPresses:
			if key == 's' {
				//When s is pressed, we need to generate a PGM file with the current state of the board

				var filename = strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.
					Itoa(turns)
				writeToFileIO(inputWorld, p, filename, c)
			} else if key == 'p' {
				//When p is pressed, pause the processing and print the current turn that is being processed
				//If p is pressed again resume the processing
				if paused {
					c.events <- StateChange{turns, Executing}
					paused = !paused
					pauseChannel <- true
				} else {
					c.events <- StateChange{turns, Paused}
					paused = !paused
				}

			} else if key == 'q' {
				//When q is pressed, generate a PGM file with the current state of the board then terminate
				handleGameShutDown(inputWorld, p, turns, c, aliveCellsTicker)
				//Exit the program
				os.Exit(0)
			}
		//When turn is incremented, we're informed of the change
		case t := <-turn:
			turns = t
			if !paused {
				pauseChannel <- true
			}
		}
	}
}

func aliveCellsReporter(turn, aliveCells *int, ticker *time.Ticker, c distributorChannels) {
	for {
		select {
		case <-ticker.C:
			worldLock.Lock()
			turnLock.Lock()
			c.events <- AliveCellsCount{*turn, *aliveCells}
			turnLock.Unlock()
			worldLock.Unlock()
		}
	}
}

//Helper function of distributor
//We use this to change color of the cells in SDL GUI (this flip initializes drawing)
func flipWorldCellsInitial(world [][]byte, imageHeight, imageWidth, turn int, c distributorChannels) {
	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			if world[i][j] == LIVE {
				c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: j, Y: i}}
			}
		}
	}
}

//Helper function of distributor
//We use this to change color of the cells in SDL GUI (update renderer after an iteration has been computed)
func flipWorldCellsIteration(oldWorld, newWorld [][]byte, turn, imageHeight, imageWidth int, c distributorChannels) {
	for i := 0; i < imageHeight; i++ {
		for j := 0; j < imageWidth; j++ {
			//If the cell has changed since the last iteration, we need to send an event to say so
			if oldWorld[i][j] != newWorld[i][j] {
				c.events <- CellFlipped{CompletedTurns: turn, Cell: util.Cell{X: j, Y: i}}
			}
		}
	}
	c.events <- TurnComplete{turn}
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

//Helper function of distributor
//Performs necessary logic to end the game neatly
func handleGameShutDown(world [][]byte, p Params, turns int, c distributorChannels,
	ticker *time.Ticker) {
	var filename = strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(turns)

	writeToFileIO(world, p, filename, c)

	//Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turns, Quitting}
	ticker.Stop()
	//Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {

	var turn = 0
	var aliveCells = 0
	var inputWorld = writeFromFileIO(p.ImageHeight, p.ImageWidth, c)

	//We need to find the strip sized passed to each worker
	var stripSizeList = distributeSliceSizes(p)

	aliveCells = getAliveCellsCount(inputWorld)
	//We create a ticker
	aliveCellsTicker := time.NewTicker(2 * time.Second)

	//We report the alive cells every two secs
	go aliveCellsReporter(&turn, &aliveCells, aliveCellsTicker, c)

	var turnChannel = make(chan int)
	var pauseChannel = make(chan bool)
	//Keep track of any key presses by the user
	go goPressTrack(inputWorld, keyPresses, c, p, turnChannel, aliveCellsTicker, pauseChannel)

	//We flip the cells
	flipWorldCellsInitial(inputWorld, p.ImageHeight, p.ImageWidth, turn, c)

	//Run the GoL algorithm for specified number of turns
	for i := 0; i < p.Turns; i++ {
		var newWorld [][]byte
		if p.Threads == 1 {
			newWorld = worker(p.ImageHeight, p.ImageWidth, inputWorld)
		} else {
			//	We need to make a wait group and communication channels for each strip
			var waitGroup sync.WaitGroup
			var workerChannelList = make([]chan [][]byte, p.Threads)
			for j := 0; j < p.Threads; j++ {
				var workerChannel = make(chan [][]byte, 2)
				workerChannelList[j] = workerChannel
			}
			//We now do split the input world for each thread accordingly
			for j := 0; j < p.Threads; j++ {
				waitGroup.Add(1)
				//We execute the workers concurrently
				go executeWorker(inputWorld, workerChannelList,
					stripSizeList, p.ImageHeight, p.ImageWidth, p.Threads, j,
					&waitGroup)
			}
			waitGroup.Wait()

			worldLock.Lock()
			newWorld = mergeWorkerStrips(newWorld, workerChannelList, stripSizeList)
			worldLock.Unlock()
		}
		aliveCells = getAliveCellsCount(newWorld)
		turnLock.Lock()
		turn++
		turnLock.Unlock()
		turnChannel <- turn

		//Update alive cells
		<-pauseChannel

		flipWorldCellsIteration(inputWorld, newWorld, turn, p.ImageHeight, p.ImageHeight, c)
		worldLock.Lock()
		inputWorld = newWorld
		worldLock.Unlock()
	}

	c.events <- FinalTurnComplete{turn, calculateAliveCells(inputWorld)}
	handleGameShutDown(inputWorld, p, p.Turns, c, aliveCellsTicker)
}
