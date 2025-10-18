package main

import (
	"fmt"
	"syscall"
)

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

}
