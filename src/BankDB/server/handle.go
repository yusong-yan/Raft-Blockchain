package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strings"
	"unicode"
)

func (rf *Raft) HandleAppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	//CHECK NETWORK
	if rf.Network == Disconnect {
		return errors.New("SERVER " + rf.Me + " DISCONNECT")
	}
	if args.Term >= rf.Term {
		//HeartBeat
		rf.Term = args.Term
		if rf.IsLeader {
			rf.BecomeFollwerFromLeader <- true
		} else {
			rf.ReceiveHB <- true
		}
		rf.setFollwer()
		reply.Success = true
		reply.Term = rf.Term

		if args.Job == Append {
			//APPEND
			entr, find := rf.getLogAtIndexWithoutLock(args.PrevLogIndex)
			if !find { //give the last index
				reply.LastIndex = rf.getLastLogEntryWithoutLock().Index
				reply.Success = false
			} else {
				if entr.Term != args.PrevLogTerm {
					rf.Log = rf.Log[:indexInLog(args.PrevLogIndex)]
					reply.LastIndex = -1
					reply.Success = false
				} else {
					rf.Log = rf.Log[:indexInLog(args.PrevLogIndex+1)]
					rf.Log = append(rf.Log, args.Entries...)
					reply.LastIndex = -1
					reply.Success = true
					rf.PeerCommit = true
				}
			}
			return nil
		} else if args.Job == CommitAndHeartBeat {
			if rf.PeerCommit == true {
				rf.PeerCommit = false
				rf.CommitIndex = min(args.LeaderCommit, rf.getLastLogEntryWithoutLock().Index)
				rf.CommitGetUpdate.Signal()
				rf.CommitGetUpdateDone.Wait()
			}
			if rf.Network == Disconnect {
				return errors.New("SERVER " + rf.Me + " DISCONNECT")
			}
			return nil
		}
		return nil
	}
	//TERM IS BIGGER JUST REPLY TERM
	reply.Term = rf.Term
	reply.Success = false
	return nil
}

func (rf *Raft) HandleRequestVote(args *RequestVoteArgs, reply *RequestVoteReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	//CHECK NETWORK
	if rf.Network == Disconnect {
		return errors.New("SERVER " + rf.Me + " DISCONNECT")
	}

	rfLastIndex := rf.getLastLogEntryWithoutLock().Index
	rfLastTerm := rf.termForLog(rfLastIndex)

	if args.Term > rf.Term {
		rf.Term = args.Term
		if rf.IsLeader {
			rf.BecomeFollwerFromLeader <- true
		} else {
			rf.ReceiveHB <- true
		}
		rf.setFollwer()
		rf.VotedFor = "None"
		//fmt.Println("receieve HB")
	}
	if (rf.VotedFor == "None") && ((rfLastTerm < args.LastLogTerm) || ((rfLastTerm == args.LastLogTerm) && (rfLastIndex <= args.LastLogIndex))) {
		rf.VotedFor = args.PeerId
		//fmt.Println("grand vote")
		reply.VoteGranted = true
	} else {
		//fmt.Println("Not grand vote")
		reply.VoteGranted = false
	}
	reply.Term = rf.Term
	reply.State = rf.State
	return nil
}

func (rf *Raft) HandleClient(args *ClientMessageArgs, reply *ClientMessageReply) error {
	rf.mu.Lock()
	if rf.Network == Disconnect {
		rf.mu.Unlock()
		return errors.New("SERVER " + rf.Me + " DISCONNECT")
	}
	rf.mu.Unlock()
	//
	//search if id exist
	//If id doesn't exist then , rf.Start(args.Message)
	//else if id exist but not commit, StartOnepeer append
	if rf.getState() != Leader {
		fmt.Println("Receive append, but not leader")
		reply.Status = NotLeader
	} else {
		ok := rf.createBlock(args.Message)
		if ok {
			reply.Status = Ok
		} else {
			reply.Status = Fail
		}
	}
	rf.mu.Lock()
	if rf.Network == Disconnect {
		rf.mu.Unlock()
		return errors.New("SERVER " + rf.Me + " DISCONNECT")
	}
	rf.mu.Unlock()
	return nil
}

func (rf *Raft) HandleClientBalance(args *ClientBalanceArgs, reply *ClientBalanceReply) error {
	rf.mu.Lock()
	if rf.Network == Disconnect {
		rf.mu.Unlock()
		return errors.New("SERVER " + rf.Me + " DISCONNECT")
	}
	rf.mu.Unlock()

	if rf.getState() != Leader {
		fmt.Println("Receive checkBalance, but not leader")
		reply.Status = NotLeader
	} else {
		money := rf.Chain.CheckBalance(args.Name)
		reply.Status = Ok
		reply.Money = money
	}
	rf.mu.Lock()
	if rf.Network == Disconnect {
		rf.mu.Unlock()
		return errors.New("SERVER " + rf.Me + " DISCONNECT")
	}
	rf.mu.Unlock()
	return nil
}
func (rf *Raft) call(rpcname string, server string, args interface{}, reply interface{}) bool {
	rf.mu.Lock()
	if rf.Network == Connect {
		rf.mu.Unlock()
		return call(rpcname, server, args, reply)
	} else {
		rf.mu.Unlock()
		return false
	}
}

func call(rpcname string, server string, args interface{}, reply interface{}) bool {
	c, err := rpc.DialHTTP("tcp", ":"+server)
	if err != nil {
		return false
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err != nil {
		//fmt.Println(err)
		return false
	}
	return true
}

func readfile(fileName string) []string {
	file, err := os.Open(fileName)
	defer file.Close()
	if err != nil {
		log.Fatalf("cannot open " + fileName)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", file)
	}
	ff := func(r rune) bool { return !unicode.IsNumber(r) && !unicode.IsLetter(r) }
	words := strings.FieldsFunc(string(content), ff)
	return words
}

func (rf *Raft) setup() {
	//readfile
	rf.Peers = readfile("servers.txt")
	fmt.Println("All peers:  ", rf.Peers)

	//setup my own server
	conn, err := net.Listen("tcp", ":"+rf.Me)
	if err != nil {
		log.Fatal("Listen:", err)
	}
	err = rpc.RegisterName("Raft", rf)
	if err != nil {
		log.Fatal("Raft:", err)
	}
	rpc.HandleHTTP()
	fmt.Println("setupServer")
	go http.Serve(conn, nil)
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
