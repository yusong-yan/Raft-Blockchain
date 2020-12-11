package main

import (
	"fmt"
	"os"

	"../server"
)

func main() {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide name")
		return
	}
	name := arguments[1]
	server.MakeClient(name)
}
