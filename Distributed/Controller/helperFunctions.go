package Controller

import (
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
	"uk.ac.bris.cs/gameoflife/util"
)

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
//Handles necessary logic for closing the client neatly and sucessfully
func handleCloseClient(client *rpc.Client) {
	closeError := client.Close()
	Shared.HandleError(closeError)
}

//Helper function to controller
//Handles logic in creating a request and a response pair
func createRequestResponsePair(p Shared.Params, c DistributorChannels) (Shared.Request, *Shared.Response) {

	//Forms the request which contains the [][]byte version of the PGM file
	request := Shared.Request{
		World:      WriteFromFileIO(p.ImageHeight, p.ImageWidth, c),
		Parameters: p,
		Events:     c.events}
	//There doesn't exist a response but we will create a new one
	response := new(Shared.Response)

	return request, response
}