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

type MetaData struct{
	path string
	method string
	queryMap map[string]string
}

type Request struct{
	metadata MetaData
	headers map[string]string
	body string
	params map[string]string
	connectionFd int
}

type Item struct{
	id string
	name string
}

var handlerMap map[string]func(Request)

func getItemsHandler(parsedRequest Request){
	connectionFd := parsedRequest.connectionFd
	writeBackResponse(connectionFd,200,"Yo")
}

func comparePaths(requestPath string,handlerPath string) bool{
	requestPathArr := strings.Split(strings.TrimSpace(requestPath),"/")
	handlerPathArr := strings.Split(strings.TrimSpace(handlerPath),"/")

	if len(requestPathArr) != len(handlerPathArr){
		return false
	} 

	for idx , _ := range handlerPathArr{
		handlerPathTerm := handlerPathArr[idx]
		requestpathTerm := requestPathArr[idx]
		if !strings.HasPrefix(handlerPathTerm,":") && handlerPathTerm != requestpathTerm{
			return false
		}
	}

	return true
}

func extractParams(requestPath string, handlerPath string) map[string]string{
	handlerPathTerms := strings.Split(handlerPath,"/")
	requestPathTerms := strings.Split(requestPath,"/")

	paramsMap := make(map[string]string,0)

	for idx, _ := range(handlerPathTerms){
		handlerPathTerm := handlerPathTerms[idx]
		if strings.HasPrefix(handlerPathTerm,":"){
			param := handlerPathTerm[1:]
			requestPathTerm := requestPathTerms[idx]
			paramsMap[param] = requestPathTerm
		}
	}
	return paramsMap
}

func findAndTriggerHandler(request Request) error{
	requestPath := request.metadata.path
	var paramsMap map[string]string
	for handlerPath := range handlerMap{
		if(comparePaths(requestPath,handlerPath)){
			paramsMap = extractParams(requestPath,handlerPath)
			request.params = paramsMap
			fun,ok := handlerMap[requestPath]
			if !ok{
				return fmt.Errorf("invalid path")
			}
			fun(request)
			return nil
		}
	}
	return fmt.Errorf("invalid path")
}

func writeBackResponse(connectionFd int, statusCode int, message string){
	// HTTP/1.1 200 OK
	response := fmt.Sprintf("HTTP/1.1 %d %s\r\n",statusCode, message)
	fmt.Printf("sending response back %s\n",message)
	syscall.Write(connectionFd,[]byte(response))
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
	terms := strings.Split(urlString," ")
	method := strings.TrimSpace(terms[0])
	path := strings.TrimSpace(terms[1])
	queryMap:= parseQueryString(path)

	return MetaData{
		method: method,
		path: strings.TrimSpace(strings.Split(path,"?")[0]),
		queryMap: queryMap,
	}
}

func parseQueryString(pathString string) map[string]string{
	pathAndQueryStrings := strings.Split(pathString, "?")

	if len(pathAndQueryStrings) < 2{
		return nil
	}

	queryString := pathAndQueryStrings[1]

	keyValueMap := make(map[string]string)
	
	keyValues := strings.Split(queryString,"&")

	for _, keyValue := range keyValues{
		keyAndValue := strings.Split(keyValue,"=")
		key := keyAndValue[0]
		value := keyAndValue[1]
		keyValueMap[key] = value
	}

	return keyValueMap
}

func parseHeaders(hearderStrings []string) map[string]string {
	headersMap := make(map[string]string)
    for _, headerString := range hearderStrings{

		headerString = strings.TrimSpace(headerString)

		if headerString == ""{
			continue
		}

		keyValue := strings.Split(headerString,":")
	
		key := strings.TrimSpace(strings.ToLower( keyValue[0]))
		value := strings.TrimSpace(strings.ToLower(keyValue[1]))
		headersMap[key] = value
	}

	return headersMap
}

func parseHttpRequest(rawRequest string) Request {
	lines := strings.Split(rawRequest,"\n")
	metaData := parseMetaData(lines[0])

	headersMap := parseHeaders(lines[1:])

	return Request{
		metadata: metaData,
		headers: headersMap,
	}

} 

func readFromConnectionSocket(connectionFd int) error {
	defer removeConnection(connectionFd)

	buf := make([]byte, 1024)
	message := ""
	payaload := ""
	for {
		readBytes, err := syscall.Read(connectionFd, buf)
		if err != nil {
			return fmt.Errorf("Error reading from connection %d: %w\n", connectionFd, err)
		}
		if readBytes == 0 {
			fmt.Printf("Client has closed connection: %d\n", connectionFd)
			fmt.Printf("Closing connection %d\n", connectionFd)
			return nil
		}
		message += string(buf[:readBytes])
		lines := strings.Split(message, "\n")

		for _, line := range lines[0 : len(lines)-1] {
			if line != "" {
				payaload += fmt.Sprintf("%s\n", line)
			}
		}

		message = lines[len(lines)-1]
		
		request := parseHttpRequest(payaload)
		headers := request.headers
		request.connectionFd = connectionFd
	
		contentLength, err := strconv.Atoi(headers["content-length"])
		if err != nil{
			fmt.Printf("error while getting content length string %v\n",err)
			panic(err)
		}

		fmt.Printf("Content length %d\n",contentLength)
		if len(message) >= contentLength{
			request.body = strings.TrimSpace(message)
		}

		fmt.Printf("Request: %+v\n",request)

		err = findAndTriggerHandler(request)

		if err != nil{
			strings.Contains(err.Error(),"invalid")
			writeBackResponse(connectionFd,400,"invalid path")
			return nil
		}
		
		return nil
	}
}

func extractAddressAndPort(sa syscall.SockaddrInet4) string {
	return fmt.Sprintf("%d.%d.%d.%d:%d", int(sa.Addr[0]), int(sa.Addr[1]), int(sa.Addr[2]), int(sa.Addr[3]), sa.Port)
}

func main() {

	listeningSocketFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	if err != nil {
		fmt.Printf("there was an error while creating the listening socket %+v\n", err)
		panic(err)
	}
	defer syscall.Close(listeningSocketFd)

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
	activeConnections = make([]int, 0)

	go handleInterrupts()

	handlerMap = make(map[string]func(Request))
	handlerMap["/items"] = getItemsHandler

	for {
		connectionFd, connectionAddress, err := syscall.Accept(listeningSocketFd)
		if err != nil {
			fmt.Printf("there was a problem accepting the connection from the listening socket %+v\n", err)
			continue
		}
		activeConnections = append(activeConnections, connectionFd)
		addressAndPort := connectionAddress.(*syscall.SockaddrInet4)
		fmt.Printf("recieved connection from %s, connection fd: %d\n", extractAddressAndPort(*addressAndPort), connectionFd)
		go readFromConnectionSocket(connectionFd)
	}

}
