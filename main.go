package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var activeConnections []int

func closeConnectionWithLog(connectionFd int) {
	fmt.Printf("Closing connection %d\n", connectionFd)
	syscall.Close(connectionFd)
}

func acknowledgeClient(connectionFd int) {
	_, err := syscall.Write(connectionFd, []byte("Connection Accepted\n"))
	if err != nil {
		fmt.Printf("Can not accept connection: %d\n", connectionFd)
	}
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

func readFromConnectionSocket(connectionFd int) {
	defer removeConnection(connectionFd)

	buf := make([]byte, 1024)
	message := ""
	payaload := ""
	for {
		readBytes, err := syscall.Read(connectionFd, buf)
		if err != nil {
			fmt.Printf("Error reading from connection %d\n", connectionFd)
			return
		}
		if readBytes == 0 {
			fmt.Printf("Client has closed connection: %d\n", connectionFd)
			fmt.Printf("Closing connection %d\n", connectionFd)
			return
		}

		message += string(buf[:readBytes])
		if strings.Contains(message, "\n") {
			splited := strings.Split(message, "\n")
			payaload = splited[0]
			message = splited[1]
		}
		fmt.Printf("Recieved Message from connection %d: %s\n", connectionFd, payaload)

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

	for {
		connectionFd, connectionAddress, err := syscall.Accept(listeningSocketFd)
		if err != nil {
			fmt.Printf("there was a problem accepting the connection from the listening socket %+v\n", err)
			continue
		}
		activeConnections = append(activeConnections, connectionFd)
		acknowledgeClient(connectionFd)
		addressAndPort := connectionAddress.(*syscall.SockaddrInet4)
		fmt.Printf("recieved connection from %s, connection fd: %d\n", extractAddressAndPort(*addressAndPort), connectionFd)
		readFromConnectionSocket(connectionFd)
	}

}
