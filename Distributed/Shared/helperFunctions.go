package Shared

import (
	"net/rpc"
)

// HandleCallAndError General helper function
//Wraps a common call-error pattern into a function. performs a call then handles any errors if necessary
func HandleCallAndError(client *rpc.Client, namedFunctionHandler string,
	request *Request, response *Response) {
	//fmt.Println(namedFunctionHandler)
	var namedFunctionHandlerError = client.Call(namedFunctionHandler, request, response)
	HandleError(namedFunctionHandlerError)
}

// HandleCreateClientAndError General Helper function
//Wraps a dial-error pattern into a functions. Creates a client. handles any errors if necessary.
func HandleCreateClientAndError(serverPort string) *rpc.Client {
	client, dialError := rpc.Dial("tcp", serverPort)

	HandleError(dialError)

	return client
}

// HandleRegisterAndError General Helper function
// Wraps a register-error pattern into a function. Registers all the handler methods. Handles any errors if necessary.
// handlerAndMethods is the struct and all its methods required to be passed in.
func HandleRegisterAndError(handlerAndMethods interface{}) {
	var registerError = rpc.Register(handlerAndMethods)
	HandleError(registerError)
}
