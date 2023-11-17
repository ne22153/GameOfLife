package main

import (
	"uk.ac.bris.cs/gameoflife/Distributed/Shared"
)

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
