package main

import (
	"fmt"
	"os"

	"../server"
)

func main() {
	arguments := os.Args
	if len(arguments) == 1 {
		fmt.Println("Please provide host:port.")
		return
	}
	port := arguments[1]
	rf := server.MakeRaft(port)
	for {
		fmt.Println("-----------------------------------------")
		fmt.Println(" 1.Connect")
		fmt.Println(" 2.Disconnect ")
		fmt.Println("-----------------------------------------\n")
		fmt.Print(" Select a option ->: ")
		var op string
		fmt.Scan(&op)
		if op == "1" {
			rf.Connect()
		} else if op == "2" {
			rf.Disconnect()
		} else {
			fmt.Println("INCORRECT  Reselect")
			continue
		}
	}
}
