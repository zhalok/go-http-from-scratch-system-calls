package main

import (
	"fmt"
	"strings"
	"syscall"
)

func readFromConnectionSocket(connectionFd int) {
	defer syscall.Close(connectionFd)

	buf := make([]byte, 1024)
	message := ""
	payaload := ""
	for {
		readBytes, err := syscall.Read(connectionFd, buf)
		if err != nil {
			fmt.Printf("Error reading from connection %d\n", connectionFd)
			syscall.Close(connectionFd)
			return
		}
		if readBytes == 0{
			fmt.Printf("Client has closed connection: %d\n",connectionFd)
			fmt.Printf("Closing connection %d\n",connectionFd)
			syscall.Close(connectionFd)
			return
		}

		message += string(buf[:readBytes])
		if strings.Contains(message, "\n") {
			splited := strings.Split(message, "\n")
			payaload = splited[0]
			message = splited[1]
		}
		fmt.Printf("Recieved Message from connection %d: %s\n",connectionFd, payaload)

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

	for {
		connectionFd, connectionAddress, err := syscall.Accept(listeningSocketFd)
		if err != nil {
			fmt.Printf("there was a problem accepting the connection from the listening socket %+v\n", err)
			continue
		}
		addressAndPort := connectionAddress.(*syscall.SockaddrInet4)
		fmt.Printf("recieved connection from %s, connection fd: %d\n", extractAddressAndPort(*addressAndPort), connectionFd)
		readFromConnectionSocket(connectionFd)
	}

}
