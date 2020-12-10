package server

import (
	"fmt"
	"strconv"
	"time"
)

type Client struct {
	clientPeers []string
	raftPeers   []string
	me          string
	counter     int
}

func MakeClient(name string) {
	cl := &Client{}
	cl.raftPeers = readfile("servers.txt")
	cl.clientPeers = readfile("clients.txt")
	cl.me = name
	cl.counter = 0
	for {
		fmt.Println("-----------------------------------------")
		fmt.Println(" 1.Balance transaction  ")
		fmt.Println(" 2.Transfer transaction ")
		fmt.Println("-----------------------------------------\n")
		fmt.Print(" Select a option ->: ")
		var op string
		fmt.Scan(&op)
		if op == "1" {
			money := 20
			fmt.Println("SUCCESS  ", cl.me, " have money: ", money)
		} else if op == "2" {
			fmt.Print(" Select a option from ")
			arr := []string{}
			//fmt.Println(peers)
			for _, k := range cl.clientPeers {
				if k != cl.me {
					arr = append(arr, k)
				}
			}
			fmt.Print(arr, " -> ")
			var receiverS string
			fmt.Scan(&receiverS)
			if !contains(arr, receiverS) {
				fmt.Println("INCORRECT  No such receiver")
				continue
			}
			fmt.Print(" Select amount of money ->")
			var moneyS string
			fmt.Scan(&moneyS)
			money, err := strconv.Atoi(moneyS)
			if err != nil {
				panic(err)
			}
			if cl.Send(cl.me, receiverS, money) {
				fmt.Println("SUCCESS  ", cl.me, " send money ", money, "$ to ", receiverS)
			} else {
				fmt.Println("SENDING FAIL")
			}
		} else {
			fmt.Println("INCORRECT  Reselect")
			continue
		}
	}
}

func (cl *Client) Send(sender string, receiver string, money int) bool {
	tran := Transaction{}
	tran.Amt = money
	tran.Receiver = receiver
	tran.Sender = sender
	tran.Id = cl.me + strconv.Itoa(cl.counter)
	cl.counter++
	for {
		if cl.TrySend(tran) {
			return true
		} else {
			fmt.Println("Enter for Retry")
			var op string
			fmt.Scan(&op)
		}
	}
}

func (cl *Client) TrySend(tran Transaction) bool {
	arg := ClientMessageArgs{}
	arg.Message = tran
	for i := 0; i < 5; i++ {
		for _, v := range cl.raftPeers {
			reply := ClientMessageReply{}
			reply.Status = Fail
			ok := call("Raft.HandleClient", v, &arg, &reply)
			if !ok {
				//Crash or Fail
				fmt.Println("Some server crash")
				time.Sleep(700 * time.Millisecond)
				continue
			}
			if reply.Status == Ok {
				//Case one too many server crash or disconnect
				//Case two in leader election
				return true
			} else {
				fmt.Println("Not Enough Server for raft! Stopped trying!")
				return false
			}
			//Else it might be not leader or is leader but cannot operat since not enough server runing
		}
	}
	return false
}
func contains(arr []string, e string) bool {
	for _, v := range arr {
		if e == v {
			return true
		}
	}
	return false
}
