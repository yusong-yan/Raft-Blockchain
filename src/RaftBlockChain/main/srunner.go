package main

import (
	"fmt"
	"os"

	"github.com/yusong-yan/Raft-Blockchain/RaftBlockChain/src/server"
)

func main() {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide host:port.")
		return
	}
	port := arguments[1]
	server.MakeRaft(port)
}
