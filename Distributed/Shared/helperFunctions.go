package Shared

import "net/rpc"

// HandleCallAndError General helper function
//Wraps a common call-error pattern into a function. performs a call then handles any errors if necessary
func HandleCallAndError(client *rpc.Client, namedFunctionHandler string,
	request *Request, response *Response) {
	var namedFunctionHandlerError = client.Call(namedFunctionHandler, request, response)
	HandleError(namedFunctionHandlerError)
}

// HandleCreateClientAndError General Helper function
//Wraps a dial-error pattern into a functions. Creates a client. handles any errors if necessary.
func HandleCreateClientAndError(serverport string) *rpc.Client {
	client, dialError := rpc.Dial("tcp", serverport)
	HandleError(dialError)

	return client
}
