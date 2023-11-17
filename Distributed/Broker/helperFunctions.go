package main

import (
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
)

//General helper function
//Wraps a common call-error pattern into a function. performs a call then handles any errors if necessary
func handleCallAndError(client *rpc.Client, namedFunctionHandler string,
	request *Shared.Request, response *Shared.Response) {
	var namedFunctionHandlerError = client.Call(namedFunctionHandler, request, response)
	Shared.HandleError(namedFunctionHandlerError)
}

func createRequestResponsePair(p Shared.Params, events chan<- Shared.Event) (Shared.Request, *Shared.Response) {

	//Forms the request which contains the [][]byte version of the PGM file
	request := Shared.Request{
		World:       nil,
		Parameters:  p,
		Events:      events,
		CurrentTurn: make(chan int, 1),
		CallAlive:   make(chan int, 1),
		GetAlive:    make(chan int, 1),
		GetTurn:     make(chan int, 1)}
	//There doesn't exist a response, but we will create a new one
	response := new(Shared.Response)

	return request, response
}
