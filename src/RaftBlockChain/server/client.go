package server

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Client struct {
	ClientPeers []string
	RaftPeers   []string
	Me          string
	Counter     int
}

func MakeClient(name string) {
	cl := &Client{}
	cl.RaftPeers = readfile("servers.txt")
	cl.ClientPeers = readfile("clients.txt")
	cl.Me = name
	cl.Counter = 0
	cl.readPersist()
	cl.startUserInterface()
}

func (cl *Client) getBalance() int {
	arg := ClientBalanceArgs{}
	arg.Name = cl.Me
	for i := 0; i < 10; i++ {
		for _, v := range cl.RaftPeers {
			reply := ClientBalanceReply{}
			reply.Status = Fail
			ok := call("Raft.HandleClientBalance", v, &arg, &reply)
			if !ok {
				fmt.Println("Some server crash")
				time.Sleep(700 * time.Millisecond)
				continue
			}
			if reply.Status == NotLeader {
				continue
			} else if reply.Status == Ok {
				return reply.Money
			} else {
				fmt.Println("Not Enough Server for raft! Stopped trying!")
				return -1
			}
		}
	}
	return -1
}

func (cl *Client) send(sender string, receiver string, money int) bool {
	tran := &Transaction{}
	tran.Amt = money
	tran.Receiver = receiver
	tran.Sender = sender
	tran.Id = cl.Me + strconv.Itoa(cl.Counter)
	cl.Counter++
	cl.persist()
	for {
		if cl.trySend(tran) {
			return true
		} else {
			fmt.Println("Enter for Retry")
			var op string
			fmt.Scan(&op)
		}
	}
}

func (cl *Client) trySend(tran *Transaction) bool {
	arg := ClientMessageArgs{}
	arg.Message = tran
	for i := 0; i < 5; i++ {
		for _, v := range cl.RaftPeers {
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
			} else if reply.Status == NotLeader {
				continue
			} else {
				fmt.Println("Not Enough Server for raft! Stopped trying!")
				return false
			}
			//Else it might be not leader or is leader but cannot operat since not enough server runing
		}
	}
	return false
}

func (cl *Client) persist() {
	fileName := cl.Me + ".yys"
	ofile, _ := os.Create(fileName)
	e := json.NewEncoder(ofile)
	e.Encode(cl.Counter)
}

func (cl *Client) readPersist() {
	fileName := cl.Me + ".yys"
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println(fileName, " file doesn't exist")
		return
	}
	d := json.NewDecoder(file)
	var Counter int
	d.Decode(&Counter)
	cl.Counter = Counter
	fmt.Println("\nREAD FROM DISK--------------")
	fmt.Println("Counter: ", cl.Counter)
	fmt.Println("----------------------------")
}

func (cl *Client) startUserInterface() {
	for {
		fmt.Println("-----------------------------------------")
		fmt.Println(" 1.Balance transaction  ")
		fmt.Println(" 2.Transfer transaction ")
		fmt.Print("-----------------------------------------\n\n")
		fmt.Print(" Select a option ->: ")
		var op string
		fmt.Scan(&op)
		if op == "1" {
			money := cl.getBalance()
			if money != -1 {
				fmt.Println("SUCCESS  ", cl.Me, " have money: ", money)
			} else {
				fmt.Println("Server crash or recovering, try again")
			}
		} else if op == "2" {
			fmt.Print(" Select a option from ")
			arr := []string{}
			//fmt.Println(peers)
			for _, k := range cl.ClientPeers {
				if k != cl.Me {
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
			if cl.send(cl.Me, receiverS, money) {
				fmt.Println("SUCCESS  ", cl.Me, " send money ", money, "$ to ", receiverS)
			} else {
				fmt.Println("SENDING FAIL")
			}
		} else {
			fmt.Println("INCORRECT  Reselect")
			continue
		}
	}
}

func contains(arr []string, e string) bool {
	for _, v := range arr {
		if e == v {
			return true
		}
	}
	return false
}
