package server

import (
	"math/rand"
	"sync"
	"time"
)

const (
	Leader = iota + 1
	Cand
	Follwer
)
const (
	DidNotWin = iota + 1
	Win
)
const (
	Connect = iota + 1
	Disconnect
)
const (
	Ok = iota + 1
	Fail
	NotLeader
)

const (
	Append = iota + 1
	CommitAndHeartBeat
	HeartBeat
)

type ApplyMsg struct {
	//CommandValid bool
	Command      *Block
	CommandIndex int
}

type Entry struct {
	Index   int
	Command *Block
	Term    int
}

type ClientMessageArgs struct {
	Message *Transaction
}

type ClientMessageReply struct {
	//Message int
	Status int
}

type ClientBalanceArgs struct {
	Name string
}

type ClientBalanceReply struct {
	Money  int
	Status int
}

type AppendEntriesArgs struct {
	Job          int
	Term         int
	LeaderId     string
	Entries      []Entry
	PrevLogIndex int //index of log entry immediately precedingnew ones
	PrevLogTerm  int //term of PrevLogIndex entry
	LeaderCommit int //leader’s commitIndex
}

type AppendEntriesReply struct {
	LastIndex int
	Term      int
	Success   bool
}

type RequestVoteArgs struct {
	PeerId       string
	Term         int
	LastLogIndex int
	LastLogTerm  int
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
	State       int
}

type Transaction struct {
	Sender   string
	Receiver string
	Amt      int
	Id       string
}

type Block struct {
	Term  int
	Phase string
	Nonce string
	Trans []*Transaction
}

type TransactionWithCond struct {
	Tran *Transaction
	Cond *sync.Cond
}

func generateTime() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	diff := 700 - 350
	return (350 + r.Intn(diff))
}
