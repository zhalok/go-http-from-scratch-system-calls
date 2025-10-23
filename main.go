package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

var activeConnections []int

type MetaData struct {
	path     string
	method   string
	queryMap map[string]string
}

type Request struct {
	metadata     MetaData
	headers      map[string]string
	body         string
	params       map[string]string
	connectionFd int
}

type Item struct {
	id   string
	name string
}

var handlerMap map[string]map[string]func(Request)

func getItemsHandler(parsedRequest Request) {
	connectionFd := parsedRequest.connectionFd
	writeBackResponse(connectionFd, 200, "Yo")
}

func splitClean(s string, sep string) []string {
	terms := strings.Split(strings.TrimSpace(s), sep)
	cleanTerms := make([]string, 0)

	for _, term := range terms {
		trimmedTerm := strings.TrimSpace(term)
		if len(trimmedTerm) != 0 {
			cleanTerms = append(cleanTerms, trimmedTerm)
		}
	}

	return cleanTerms
}

func getItemHandler(parsedRequest Request) {
	connectionFd := parsedRequest.connectionFd
	id, ok := parsedRequest.params["id"]
	if !ok {
		response := "id not found\n"
		writeBackResponse(connectionFd, 400, response)
		return
	}
	response := fmt.Sprintf("id: %s\n", id)
	writeBackResponse(connectionFd, 200, response)
}

func createItemHandler(parsedRequest Request) {
	connectionFd := parsedRequest.connectionFd
	writeBackResponse(connectionFd, 200, "hello")
}

func comparePaths(requestPath string, handlerPath string) bool {
	requestPathArr := splitClean(requestPath, "/")
	handlerPathArr := splitClean(handlerPath, "/")

	if len(requestPathArr) != len(handlerPathArr) {
		return false
	}

	for idx, _ := range handlerPathArr {
		handlerPathTerm := handlerPathArr[idx]
		requestpathTerm := requestPathArr[idx]
		if !strings.HasPrefix(handlerPathTerm, ":") && handlerPathTerm != requestpathTerm {
			return false
		}
	}

	return true
}

func extractParams(requestPath string, handlerPath string) map[string]string {
	handlerPathTerms := strings.Split(handlerPath, "/")
	requestPathTerms := strings.Split(requestPath, "/")

	paramsMap := make(map[string]string, 0)

	for idx, _ := range handlerPathTerms {
		handlerPathTerm := handlerPathTerms[idx]
		if strings.HasPrefix(handlerPathTerm, ":") {
			param := handlerPathTerm[1:]
			requestPathTerm := requestPathTerms[idx]
			paramsMap[param] = requestPathTerm
		}
	}
	return paramsMap
}

func findAndTriggerHandler(request Request) error {
	requestPath := request.metadata.path
	requestMethod := request.metadata.method
	var paramsMap map[string]string
	handlerPaths := handlerMap[requestMethod]

	for handlerPath := range handlerPaths {
		if comparePaths(requestPath, handlerPath) {
			paramsMap = extractParams(requestPath, handlerPath)
			request.params = paramsMap
			fun, ok := handlerMap[requestMethod][handlerPath]
			if !ok {
				return fmt.Errorf("invalid path, handler not registered")
			}
			fun(request)
			return nil
		}
	}
	return fmt.Errorf("invalid path")
}

func writeBackResponse(connectionFd int, statusCode int, message string) {
	response := fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, message)
	fmt.Printf("sending response back %s\n", message)
	syscall.Write(connectionFd, []byte(response))
}

func closeConnectionWithLog(connectionFd int) {
	fmt.Printf("Closing connection %d\n", connectionFd)
	syscall.Close(connectionFd)
}

func removeConnection(connectionFd int) {

	closeConnectionWithLog(connectionFd)
	newActiveConnection := make([]int, 0)

	for _, activeConnection := range activeConnections {
		if activeConnection != connectionFd {
			newActiveConnection = append(newActiveConnection, activeConnection)
		}

	}
	activeConnections = newActiveConnection

}

func cleanActiveConnections() {
	for _, activeConnection := range activeConnections {
		closeConnectionWithLog(activeConnection)
	}
}

func handleInterrupts() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down gracefully...")

		cleanActiveConnections()

		os.Exit(0)
	}()

	select {}
}

func parseMetaData(urlString string) MetaData {
	terms := strings.Split(urlString, " ")
	method := strings.TrimSpace(terms[0])
	path := strings.TrimSpace(terms[1])
	queryMap := parseQueryString(path)

	return MetaData{
		method:   method,
		path:     strings.TrimSpace(strings.Split(path, "?")[0]),
		queryMap: queryMap,
	}
}

func parseQueryString(pathString string) map[string]string {
	pathAndQueryStrings := strings.Split(pathString, "?")

	if len(pathAndQueryStrings) < 2 {
		return nil
	}

	queryString := pathAndQueryStrings[1]

	keyValueMap := make(map[string]string)

	keyValues := strings.Split(queryString, "&")

	for _, keyValue := range keyValues {
		keyAndValue := strings.Split(keyValue, "=")
		key := keyAndValue[0]
		value := keyAndValue[1]
		keyValueMap[key] = value
	}

	return keyValueMap
}

func parseHeaders(hearderStrings []string) map[string]string {
	headersMap := make(map[string]string)
	for _, headerString := range hearderStrings {

		headerString = strings.TrimSpace(headerString)

		if headerString == "" {
			continue
		}

		keyValue := strings.Split(headerString, ":")

		key := strings.TrimSpace(strings.ToLower(keyValue[0]))
		value := strings.TrimSpace(strings.ToLower(keyValue[1]))
		headersMap[key] = value
	}

	return headersMap
}

func parseHttpRequest(rawRequest string) Request {
	lines := strings.Split(rawRequest, "\n")
	metaData := parseMetaData(lines[0])

	headersMap := parseHeaders(lines[1:])

	return Request{
		metadata: metaData,
		headers:  headersMap,
	}

}

func readHeaders(connectionFd int) (string, string, error){
	message := ""
	buf := make([]byte, 1)
	for !strings.Contains(message, "\r\n\r\n") {
		readBytes, err := syscall.Read(connectionFd, buf)
		if readBytes == 0 {
			return "", "", fmt.Errorf("no bytes to read")

		}
		if err != nil {
			return "", "", fmt.Errorf("there was a problem while reading from the connection %w", err)
		}

		message += string(buf[:readBytes])
	}

	lines := splitClean(message,"\r\n\r\n")
	headerString := lines[0]
	restRequest := ""

	if len(lines) > 1{
		restRequest = lines[1]
	}

	return headerString, restRequest, nil
}

func readBody(connectionFd int, alreadyReadString string, contentLength int) (string,error){
	message := alreadyReadString
	buf := make([]byte,1)
	for len(message) < contentLength{
		readBytes, err := syscall.Read(connectionFd,buf)
		if readBytes == 0{
			return "", fmt.Errorf("there was no byte to be read from the connection")
		}
		if err != nil {
			return "", fmt.Errorf("there was a problem while reading from the connection %w",err)
		}
		message += string(buf[:readBytes])
	}
	return message, nil
}

func readFromConnectionSocket(connectionFd int) error {
	defer removeConnection(connectionFd)

	headerString, restRequest, err := readHeaders(connectionFd)
	if err != nil{
		return fmt.Errorf("there was a problem while reading headers %w", err)
	}

	lines := strings.Split(headerString,"\n")
	parsedMetaData := parseMetaData(lines[0])
	parsedHeaders := parseHeaders(lines[1:])

	contentLength, ok := parsedHeaders["content-length"]
	if !ok {
		contentLength = "0"
	}

	contentLengthInt, err := strconv.Atoi(contentLength)

	if err != nil{
		return fmt.Errorf("there was a problem converting content length to integer %w",err)
	}

	body, err := readBody(connectionFd, restRequest, contentLengthInt)

	if err != nil{
		return fmt.Errorf("there was a problem while reading body from the connection %w",err)
	}

	request := Request{
		metadata: parsedMetaData,
		headers: parsedHeaders,
		body: body,
	}

	fmt.Printf("request %+v\n",request)

	return nil
}

func extractAddressAndPort(sa syscall.SockaddrInet4) string {
	return fmt.Sprintf("%d.%d.%d.%d:%d", int(sa.Addr[0]), int(sa.Addr[1]), int(sa.Addr[2]), int(sa.Addr[3]), sa.Port)
}

func main() {
	activeConnections = make([]int, 0)

	listeningSocketFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		fmt.Printf("there was an error while creating the listening socket %+v\n", err)
		panic(err)
	}
	defer syscall.Close(listeningSocketFd)

	syscall.SetsockoptInt(listeningSocketFd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)

	fmt.Printf("listening socket file descriptor %d\n", listeningSocketFd)

	activeConnections = append(activeConnections, listeningSocketFd)
	fmt.Printf("active connections %v\n", activeConnections)

	addr := [4]byte{127, 0, 0, 1}
	sourceAddressAndPort := syscall.SockaddrInet4{
		Addr: addr,
		Port: 8080,
	}

	err = syscall.Bind(listeningSocketFd, &sourceAddressAndPort)

	if err != nil {
		fmt.Printf("there was a problem binding the socket with source ip and port %+v\n", err)
		panic(err)
	}

	err = syscall.Listen(listeningSocketFd, 10)

	if err != nil {
		fmt.Printf("there was problem listening on the socket %+v\n", err)
		panic(err)
	}

	fmt.Println("Listening on 127.0.0.1:8080")

	go handleInterrupts()

	handlerMap = make(map[string]map[string]func(Request))
	handlerMap["GET"] = make(map[string]func(Request))
	handlerMap["POST"] = make(map[string]func(Request))

	handlerMap["GET"]["/items"] = getItemsHandler
	handlerMap["GET"]["/items/:id"] = getItemHandler
	handlerMap["POST"]["/items"] = createItemHandler

	for {
		connectionFd, connectionAddress, err := syscall.Accept(listeningSocketFd)
		if err != nil {
			fmt.Printf("there was a problem accepting the connection from the listening socket %+v\n", err)
			break
		}
		activeConnections = append(activeConnections, connectionFd)
		addressAndPort := connectionAddress.(*syscall.SockaddrInet4)
		fmt.Printf("recieved connection from %s, connection fd: %d\n", extractAddressAndPort(*addressAndPort), connectionFd)
		go readFromConnectionSocket(connectionFd)
	}

}
