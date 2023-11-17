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
