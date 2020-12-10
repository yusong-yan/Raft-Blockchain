package server

import (
	"math/rand"
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
)

const (
	Append = iota + 1
	CommitAndHeartBeat
	HeartBeat
)

type ApplyMsg struct {
	//CommandValid bool
	Command      string
	CommandIndex int
}

type Entry struct {
	Index   int
	Command string
	Term    int
	//Id      int
}

type ClientMessageArgs struct {
	Message Transaction
}

type ClientMessageReply struct {
	Message int
	Status  int
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

func generateTime() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	diff := 700 - 350
	return (350 + r.Intn(diff)) * 10
}
