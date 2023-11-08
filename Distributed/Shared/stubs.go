package Shared

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	ServerPort  string
}

var GoLHandler = "GoLOperations.GoLManager"

type Response struct {
	World [][]byte
}

type Request struct {
	World      [][]byte
	Parameters Params
}
