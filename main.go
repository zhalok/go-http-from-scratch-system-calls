package main

import (
	"fmt"
	"syscall"
)

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
	}

}
