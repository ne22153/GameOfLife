package main

import (
	"fmt"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
	"uk.ac.bris.cs/gameoflife/util"
)

type pauseStruct struct {
	pause bool
	lock  sync.Mutex
}

var paused pauseStruct

func changePaused() {
	paused.lock.Lock()
	paused.pause = !paused.pause
	paused.lock.Unlock()
}

func getPaused() bool {
	paused.lock.Lock()
	var temp = paused.pause
	paused.lock.Unlock()
	return temp
}

//Helper function of controller
//Performs necessary logic in order to handle the ticker
func aliveCellsReporter(ticker *time.Ticker, c DistributorChannels,
	client *rpc.Client, request *Shared.Request, response *Shared.Response) {
	c.events <- Shared.AliveCellsCount{CompletedTurns: 0, CellsCount: 0}
	flipWorldCellsInitial(request.World, request.Parameters.ImageHeight, request.Parameters.ImageWidth, 0, c)
	currentWorld := request.World
	for {
		select {
		//When the ticker triggers,
		//we send an RPC call to return the number of alive cells, and number of turns processed
		case <-ticker.C:
			request.World = currentWorld
			Shared.HandleCallAndError(client, Shared.BrokerInfo, request, response)
			c.events <- Shared.AliveCellsCount{
				CompletedTurns: response.Turns,
				CellsCount:     response.AliveCells}
			if !getPaused() {
				for i := 0; i < len(response.FlippedCells); i++ {
					c.events <- Shared.CellFlipped{CompletedTurns: response.Turns, Cell: response.FlippedCells[i]}
				}
				c.events <- Shared.TurnComplete{CompletedTurns: response.Turns}
				currentWorld = response.World
			}
		}
	}
}

//Helper function to controller
//Simply calculate the alive cells such that it can be used as a parameter for the final turn event
func calculateAliveCells(world [][]byte) []util.Cell {
	var coordinates []util.Cell
	for index, row := range world {
		for index2 := range row {
			if world[index][index2] > 0 {
				coordinates = append(coordinates, util.Cell{X: index2, Y: index})
			}
		}
	}
	return coordinates
}

//Helper function to controller
//Handles logic in creating a request and a response pair
func createRequestResponsePair(p Shared.Params, c DistributorChannels) (Shared.Request, *Shared.Response) {

	//Forms the request which contains the [][]byte version of the PGM file
	request := Shared.Request{
		World:      WriteFromFileIO(p.ImageHeight, p.ImageWidth, c),
		Parameters: p,
		Events:     c.events}
	//There doesn't exist a response, but we will create a new one
	response := new(Shared.Response)

	return request, response
}

//Helper function to controller
//Handles necessary logic for closing the client neatly and successfully
func handleCloseClient(client *rpc.Client) {
	closeError := client.Close()
	Shared.HandleError(closeError)
}

//General helper function
//Set the io to idle and ticker to stop and close the client
func shutDownIOTickerClient(c DistributorChannels, ticker *time.Ticker, client *rpc.Client) {
	ticker.Stop()
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	defer handleCloseClient(client)
}

//Helper function of controller
//Performs necessary logic to end the game neatly
func handleGameShutDown(client *rpc.Client, response *Shared.Response,
	p Shared.Params, c DistributorChannels, ticker *time.Ticker) {
	var filename = strconv.Itoa(p.ImageWidth) + "x" +
		strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(p.Turns)
	writeToFileIO(response.World, p, filename, c)
	shutDownIOTickerClient(c, ticker, client)
	close(c.events)
	//os.Exit(0)
}

//Helper function of controller
//Performs necessary logic for key presses by the user
func determineKeyPress(client *rpc.Client, keyPresses <-chan rune,
	req *Shared.Request, res *Shared.Response,
	ticker *time.Ticker, c DistributorChannels) {
	//We make sure this runs forever while the controller is alive
	for {
		select {
		case key := <-keyPresses:
			if key == 'k' {
				Shared.HandleCallAndError(client, Shared.BrokerInfo, req, res)
				Shared.HandleCallAndError(client, Shared.BrokerKill, req, res)
				handleGameShutDown(client, res, req.Parameters, c, ticker)
				os.Exit(0)
			} else if key == 's' {
				Shared.HandleCallAndError(client, Shared.BrokerInfo, req, res)
				var filename = strconv.Itoa(req.Parameters.ImageWidth) + "x" +
					strconv.Itoa(req.Parameters.ImageHeight) + "x" +
					strconv.Itoa(res.Turns)
				writeToFileIO(res.World, req.Parameters, filename, c)
			} else if key == 'p' {
				fmt.Println("Continuing")
				Shared.HandleCallAndError(client, Shared.BrokerPause, req, res)
				changePaused()
			} else if key == 'q' {
				Shared.HandleCallAndError(client, Shared.BrokerBackground, req, res)
				Shared.HandleCallAndError(client, Shared.BrokerInfo, req, res)
				shutDownIOTickerClient(c, ticker, client)
				os.Exit(0)
			}
		}
	}
}
