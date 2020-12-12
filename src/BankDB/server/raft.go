package server

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

type Raft struct {
	//setup
	Me    string
	mu    sync.Mutex
	Peers []string
	//
	Log                     []Entry
	IsLeader                bool
	State                   int
	Term                    int
	VotedFor                string
	ReceiveHB               chan bool
	BecomeFollwerFromLeader chan bool
	Network                 int
	NextIndex               map[string]int
	MatchIndex              map[string]int
	PeerAlive               map[string]bool
	OpenCommit              map[string]bool
	CommitIndex             int
	Chain                   *Chain
	ApplyCh                 chan ApplyMsg
	CommitGetUpdate         *sync.Cond
	CommitGetUpdateDone     *sync.Cond
	LastApply               int

	RecevieTransactionWithCond chan *TransactionWithCond
}

func MakeRaft(me string) {
	rf := &Raft{}
	rf.State = Follwer
	rf.Network = Connect
	rf.Peers = []string{}
	rf.Log = []Entry{}
	rf.VotedFor = "None"
	rf.IsLeader = false
	rf.Me = me
	rf.Term = 0
	rf.ReceiveHB = make(chan bool, 1)
	rf.BecomeFollwerFromLeader = make(chan bool, 1)
	rf.NextIndex = map[string]int{}
	rf.MatchIndex = map[string]int{}
	rf.PeerAlive = map[string]bool{}
	//rf.PeerCommit = false
	rf.OpenCommit = map[string]bool{}
	rf.CommitIndex = 0
	rf.LastApply = 0

	rf.RecevieTransactionWithCond = make(chan *TransactionWithCond, 1)
	for i := 0; i < len(rf.Peers); i++ {
		server := rf.Peers[i]
		rf.NextIndex[server] = rf.getLastLogEntryWithoutLock().Index + 1
		rf.MatchIndex[server] = rf.NextIndex[server] - 1
		rf.PeerAlive[server] = true
	}
	rf.setup()
	//For state Machine
	rf.ApplyCh = make(chan ApplyMsg, 1)
	rf.Chain = MakeChain(rf.ApplyCh)
	rf.CommitGetUpdate = sync.NewCond(&rf.mu)
	rf.CommitGetUpdateDone = sync.NewCond(&rf.mu)
	go rf.listenApply()
	//Start Raft
	//fmt.Println("Become Follwer with Term", rf.Term)
	go rf.startElection()

	rf.startUserInterface()
}

//ELECTION TIMER
func (rf *Raft) startElection() {
	for {
		ticker := time.NewTicker(time.Duration(generateTime()) * time.Millisecond)
		electionResult := make(chan int, 1)
	Loop:
		for {
			select {
			case <-ticker.C:
				interval := generateTime()
				ticker = time.NewTicker(time.Duration(interval) * time.Millisecond)
				go func() {
					electionResult <- rf.startAsCand(interval)
				}()
			case <-rf.ReceiveHB:
				ticker = time.NewTicker(time.Duration(generateTime()) * time.Millisecond)
			case a := <-electionResult:
				if a == Win {
					break Loop
				}
			default:
			}
		}
		ticker.Stop()

		rf.mu.Lock()
		rf.setLeader()
		rf.mu.Unlock()

		go rf.startAsLeader()
		<-rf.BecomeFollwerFromLeader
		fmt.Println("Become Follwer from Leader with Term", rf.getTerm())
	}
}

//ELECTION
func (rf *Raft) startAsCand(interval int) int {
	//setup timer for cand
	//fmt.Println("start election")
	cond := sync.NewCond(&rf.mu)
	var needReturn bool
	needReturn = false
	go func(needReturn *bool, cond *sync.Cond) {
		time.Sleep(time.Duration(interval-20) * time.Millisecond)
		rf.mu.Lock()
		*needReturn = true
		rf.mu.Unlock()
		cond.Signal()
	}(&needReturn, cond)

	//setup args and rf
	hearedBack := 1
	hearedBackSuccess := 1
	votes := 1
	args := RequestVoteArgs{}
	rf.mu.Lock()
	//rf.PeerCommit = false
	rf.State = Cand
	rf.Term = rf.Term + 1
	fmt.Println("Become Candidate with Term", rf.Term)
	rf.VotedFor = rf.Me
	args.Term = rf.Term
	args.PeerId = rf.Me
	args.LastLogIndex = rf.getLastLogEntryWithoutLock().Index
	args.LastLogTerm = rf.termForLog(args.LastLogIndex)
	rf.mu.Unlock()
	// fmt.Println(rf.Me, "start election with lastIndex", args.LastLogIndex, "and lastlongTerm", args.LastLogTerm)
	for s := 0; s < len(rf.Peers); s++ {
		server := rf.Peers[s]
		if server == rf.Me {
			continue
		}
		reply := RequestVoteReply{}

		go func() {
			ok := rf.call("Raft.HandleRequestVote", server, &args, &reply)
			//Handle Reply
			if !ok || needReturn {
				rf.mu.Lock()
				hearedBack++
				rf.mu.Unlock()
				cond.Signal()
				return
			}
			rf.mu.Lock()
			hearedBack++
			hearedBackSuccess++
			if reply.Term > rf.Term && rf.State == Cand {
				rf.Term = reply.Term
				rf.ReceiveHB <- true
				rf.mu.Unlock()
				cond.Signal()
				return
			}

			if reply.VoteGranted == true && rf.State == Cand {
				votes++
			}
			rf.mu.Unlock()
			cond.Signal()
		}()
	}
	//wait
	rf.mu.Lock()
	for hearedBack != len(rf.Peers) && votes <= len(rf.Peers)/2 && needReturn == false && rf.State == Cand {
		cond.Wait()
	}
	//decide
	if votes > len(rf.Peers)/2 && rf.State == Cand && needReturn == false {
		rf.mu.Unlock()
		return Win
	} else {
		//fmt.Println("Lose becuase of Vote", votes)
		rf.mu.Unlock()
		return DidNotWin
	}
}

//Become leader, and setup leader
func (rf *Raft) startAsLeader() {
	go rf.blockGenerator()
	//setupleader
	rf.mu.Lock()
	for i := 0; i < len(rf.Peers); i++ {
		server := rf.Peers[i]
		rf.NextIndex[server] = rf.getLastLogEntryWithoutLock().Index + 1
		rf.MatchIndex[server] = rf.NextIndex[server] - 1
		rf.PeerAlive[server] = true
		rf.OpenCommit[server] = false
	}
	//rf.PeerCommit = false
	rf.mu.Unlock()
	rf.start(nil)
	for {
		go rf.sendHeartBeat()
		if rf.getState() != Leader {
			return
		}
		time.Sleep(time.Duration(120) * time.Millisecond)
	}
}

//every 150 milisecond, send heartbeat also commit entry
func (rf *Raft) sendHeartBeat() {
	if rf.getState() == Leader {
		hearedBack := 1
		hearedBackSuccess := 1
		for s := 0; s < len(rf.Peers); s++ {
			server := rf.Peers[s]
			if server == rf.Me {
				continue
			}
			args := AppendEntriesArgs{}
			args.LeaderId = rf.Me
			args.Entries = []Entry{}
			args.Job = CommitAndHeartBeat
			rf.mu.Lock()
			args.LeaderCommit = rf.CommitIndex
			args.Term = rf.Term
			args.Job = HeartBeat
			if rf.OpenCommit[server] {
				args.Job = CommitAndHeartBeat
			}
			rf.mu.Unlock()
			reply := AppendEntriesReply{}
			go func() {
				ok := rf.call("Raft.HandleAppendEntries", server, &args, &reply)
				//Handle Reply
				if !ok {
					//send Fail maybe crash or disconnet
					//fmt.Println("HB to " + server + " lost")
					rf.mu.Lock()
					hearedBack++
					rf.PeerAlive[server] = false
					rf.OpenCommit[server] = false
					rf.mu.Unlock()
					return
				}
				//fmt.Println("HB to " + server + " send")
				rf.mu.Lock()
				hearedBack++
				hearedBackSuccess++
				if reply.Term > rf.Term && rf.State == Leader {
					// fmt.Println(rf.Me+" become follwer from Term ", rf.Term, " to ", reply.Term)
					rf.Term = reply.Term
					rf.BecomeFollwerFromLeader <- true
					rf.setFollwer()
					rf.mu.Unlock()
					return
				}
				if !rf.PeerAlive[server] && rf.State == Leader {
					rf.PeerAlive[server] = true
					go func() {
						rf.startOnePeerAppend(server)
						rf.mu.Lock()
						if rf.updateCommitForLeader() && rf.IsLeader {
							rf.CommitGetUpdate.Signal()
							rf.CommitGetUpdateDone.Wait()
						}
						rf.mu.Unlock()
					}()
				}
				rf.mu.Unlock()
			}()
		}
	}
}

//append block to log and send to everyone
func (rf *Raft) start(Command *Block) (int, int, bool) {
	fmt.Println("Recieve message , Appending")
	Index := -1
	Term := -1
	IsLeader := rf.getState() == Leader
	//check if ID exist
	if IsLeader {
		hearedBack := 1
		hearedBackSuccess := 1
		cond := sync.NewCond(&rf.mu)
		rf.mu.Lock()
		Term = rf.Term
		newE := Entry{}
		if Command != nil {
			newE.Command = Command
			newE.Index = rf.getLastLogEntryWithoutLock().Index + 1
			newE.Term = rf.Term
			rf.Log = append(rf.Log, newE)
		} else {
			if (len(rf.Log)) > 0 {
				rf.Log[len(rf.Log)-1].Term = rf.Term
			}
		}
		Index = rf.getLastLogEntryWithoutLock().Index
		rf.mu.Unlock()
		for i := 0; i < len(rf.Peers); i++ {
			server := rf.Peers[i]
			if server == rf.Me {
				continue
			}
			go func() {
				ok := rf.startOnePeerAppend(server)
				rf.mu.Lock()
				hearedBack++
				if ok {
					hearedBackSuccess++
				}
				rf.mu.Unlock()
				cond.Signal()
			}()
		}

		//wait
		rf.mu.Lock()
		for hearedBack != len(rf.Peers) && hearedBackSuccess <= len(rf.Peers)/2 && rf.IsLeader {
			cond.Wait()
		}
		//decide
		if hearedBackSuccess <= len(rf.Peers)/2 && rf.IsLeader {
			//rf.HeartBeatJob = CommitAndHeartBeat
			// rf.BecomeFollwerFromLeader <- true
			// rf.setFollwer()
			rf.mu.Unlock()
			return -1, -1, false
		} else {
			if rf.updateCommitForLeader() && rf.IsLeader {
				rf.CommitGetUpdate.Signal()
				rf.CommitGetUpdateDone.Wait()
				rf.mu.Unlock()
				return Index, Term, IsLeader
			} else {
				rf.mu.Unlock()
				return -1, -1, false
			}
		}
	}
	return -1, -1, false
}

//update one server's log
func (rf *Raft) startOnePeerAppend(server string) bool {
	result := false
	if rf.getState() == Leader {
		//set up sending log
		entries := []Entry{}
		args := AppendEntriesArgs{}
		rf.mu.Lock()
		for i := rf.MatchIndex[server] + 1; i <= rf.getLastLogEntryWithoutLock().Index; i++ {
			entry, find := rf.getLogAtIndexWithoutLock(i)
			if !find {
				entries = []Entry{}
				break
			}
			entries = append(entries, entry)
		}
		args.LeaderId = rf.Me
		args.Term = rf.Term
		args.PrevLogIndex = rf.MatchIndex[server]
		args.PrevLogTerm = rf.termForLog(args.PrevLogIndex)
		args.Entries = entries
		args.LeaderCommit = rf.CommitIndex
		args.Job = Append
		rf.mu.Unlock()
		for rf.getState() == Leader {
			reply := AppendEntriesReply{}
			rf.mu.Lock()
			if rf.PeerAlive[server] && rf.IsLeader {
				rf.mu.Unlock()
				ok := rf.call("Raft.HandleAppendEntries", server, &args, &reply)
				if !ok {
					rf.mu.Lock()
					rf.PeerAlive[server] = false
					rf.OpenCommit[server] = false
					rf.mu.Unlock()
					result = false
					break
				}
			} else {
				rf.mu.Unlock()
				result = false
				break
			}

			if reply.Success {
				//update
				rf.mu.Lock()
				rf.MatchIndex[server] = len(args.Entries) + args.PrevLogIndex
				rf.NextIndex[server] = rf.MatchIndex[server] + 1
				rf.OpenCommit[server] = true
				rf.PeerAlive[server] = true
				rf.mu.Unlock()
				result = true
				break
			} else {
				//resend
				rf.mu.Lock()
				args.Term = rf.Term
				args.LeaderCommit = rf.CommitIndex
				if reply.LastIndex != -1 {
					//if server's log size bigger than rflog size
					args.PrevLogIndex = reply.LastIndex
				} else {
					args.PrevLogIndex = args.PrevLogIndex - 1
				}
				args.PrevLogTerm = rf.termForLog(args.PrevLogIndex)
				args.Entries = rf.Log[indexInLog(args.PrevLogIndex+1):]
				rf.mu.Unlock()
			}
		}
	}
	return result
}

//If ther is new commit, push it to State machine
func (rf *Raft) listenApply() {
	for {
		rf.mu.Lock()
		rf.CommitGetUpdate.Wait()
		for rf.CommitIndex > rf.LastApply {
			rf.LastApply = rf.LastApply + 1
			am := ApplyMsg{}
			am.Command = rf.Log[indexInLog(rf.LastApply)].Command
			am.CommandIndex = rf.LastApply
			rf.ApplyCh <- am
		}
		rf.mu.Unlock()
		rf.CommitGetUpdateDone.Signal()
	}
}

func (rf *Raft) getState() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.State
}

func (rf *Raft) getTerm() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.Term
}

func (rf *Raft) connect() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.Network = Connect
}

func (rf *Raft) disconnect() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.Network = Disconnect
}

func (rf *Raft) setLeader() {
	rf.IsLeader = true
	rf.State = Leader
	fmt.Println("Become Leader with Term", rf.Term)
}

func (rf *Raft) setFollwer() {
	rf.State = Follwer
	rf.IsLeader = false
	//fmt.Println("Become Follwer with Term", rf.Term)
}

//Update leader's commitIndex
func (rf *Raft) updateCommitForLeader() bool {
	beginIndex := rf.CommitIndex + 1
	lastCommittedIndex := -1
	updated := false
	for ; beginIndex <= rf.getLastLogEntryWithoutLock().Index; beginIndex++ {
		granted := 1

		for Server, ServerMatchIndex := range rf.MatchIndex {
			if Server == rf.Me || !rf.PeerAlive[Server] {
				continue
			}
			if ServerMatchIndex >= beginIndex {
				granted++
			}
		}

		if granted >= len(rf.Peers)/2+1 {
			lastCommittedIndex = beginIndex
		}
	}
	if lastCommittedIndex > rf.CommitIndex && rf.IsLeader {
		rf.CommitIndex = lastCommittedIndex
		updated = true
	}
	return updated
}

func (rf *Raft) printLog() {
	rf.mu.Lock()
	var state string
	if rf.State == Leader {
		state = "Leader"
	} else if rf.State == Cand {
		state = "Candidate"
	} else {
		state = "Follwer"
	}
	fmt.Println("-----------------------------------------")
	fmt.Println(state, " with ", rf.Term, " have LOG:")
	for _, vs := range rf.Log {
		for _, v := range vs.Command.Trans {
			fmt.Print(v.Sender)
			fmt.Print(" send to ")
			fmt.Print(v.Receiver)
			fmt.Print(" money:")
			fmt.Print(v.Amt)
			fmt.Println()
		}
	}
	fmt.Print("-----------------------------------------\n\n")
	rf.mu.Unlock()
}
func (rf *Raft) getLogAtIndexWithoutLock(index int) (Entry, bool) {
	if index == 0 {
		return Entry{}, true
	} else if len(rf.Log) == 0 {
		return Entry{}, false
	} else if (index < -1) || (index > rf.getLastLogEntryWithoutLock().Index) {
		return Entry{}, false
	} else {
		localIndex := index - rf.Log[0].Index
		return rf.Log[localIndex], true
	}
}

func (rf *Raft) getLogAtIndex(index int) (Entry, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.getLogAtIndexWithoutLock(index)
}

func (rf *Raft) getLastLogEntryWithoutLock() Entry {
	entry := Entry{}
	if len(rf.Log) == 0 {
		entry.Term = 0
		entry.Index = 0
	} else {
		entry = rf.Log[len(rf.Log)-1]
	}
	return entry
}

func (rf *Raft) getLastLogEntry() Entry {
	entry := Entry{}
	rf.mu.Lock()
	entry = rf.getLastLogEntryWithoutLock()
	rf.mu.Unlock()
	return entry
}

func (rf *Raft) termForLog(index int) int {
	entry, ok := rf.getLogAtIndexWithoutLock(index)
	if ok {
		return entry.Term
	} else {
		return -1
	}
}

func indexInLog(index int) int {
	if index > 0 {
		return index - 1
	} else {
		println("ERROR")
		return -1
	}
}

//ALl rest of the function  is for Creating block
func (rf *Raft) createBlock(tran *Transaction) bool {
	fmt.Println("Recieve message , deciding leader")
	if rf.Chain.ExistId(tran.Id) {
		fmt.Println("Receive append, but exist")
		return true
	}
	//give it to generator and wait for response
	fmt.Println("Starting create block")
	cond := sync.NewCond(&rf.mu)
	TWC := &TransactionWithCond{}
	TWC.Cond = cond
	TWC.Tran = tran
	rf.RecevieTransactionWithCond <- TWC
	rf.mu.Lock()
	cond.Wait()
	rf.mu.Unlock()
	time.Sleep(200 * time.Millisecond)
	if rf.Chain.ExistId(tran.Id) {
		fmt.Println("Receive append, exist")
		return true
	}
	//_, _, isLeader := rf.Start(Block)
	return false
}

//So far only one block can be generated, later I will update the function, so more minner can create block at same time
func (rf *Raft) blockGenerator() {
	counter := 0
	var responses []*sync.Cond
	var b *Block
	for rf.isLeaderAndConnect() {
		if counter < 3 {
			select {
			case TWC := <-rf.RecevieTransactionWithCond:
				fmt.Println("Receive on Transaction in blockGenerator")
				if counter == 0 {
					responses = []*sync.Cond{}
					b = &Block{}
					b.Trans = []*Transaction{}
				}
				b.Trans = append(b.Trans, TWC.Tran)
				responses = append(responses, TWC.Cond)
				counter++
			default:
			}
		}
		if counter != 0 {
			rf.mu.Lock()
			b.Term = rf.Term
			b.Phase = rf.asSha256((rf.Chain.LastBlock()))
			rf.mu.Unlock()
			b.Nonce = strconv.Itoa(rand.Int())
			if rf.validNonce(rf.asSha256(b)) {
				rf.start(b)
				for _, v := range responses {
					v.Signal()
				}
				counter = 0
			}
			time.Sleep(100 * time.Millisecond)
			fmt.Println("try nonce fail")
		}
	}
	for _, v := range responses {
		v.Signal()
	}
}

func (rf *Raft) validNonce(s string) bool {
	lastChar := s[len(s)-1]
	fmt.Println(lastChar)
	if 0 <= lastChar && lastChar <= 2 {
		return true
	}
	return false
}

func (rf *Raft) asSha256(o interface{}) string {
	s := fmt.Sprintf("%v", o)
	hash := sha256.Sum256([]byte(s))
	return string(hash[:])
}

func (rf *Raft) printBlocks() {
	rf.Chain.PrintBlocks()
}

func (rf *Raft) startUserInterface() {
	for {
		//fmt.Println("-----------------------------------------")
		fmt.Println("\n a.PrintLog")
		fmt.Println(" b.Connect ")
		fmt.Println(" c.Disconnect")
		fmt.Println(" d.PrintStateMachine")
		//fmt.Println("-----------------------------------------\n")
		fmt.Print(" Enter: ")
		var op string
		fmt.Scan(&op)
		if op == "b" {
			rf.connect()
		} else if op == "c" {
			rf.disconnect()
		} else if op == "a" {
			rf.printLog()
		} else if op == "d" {
			rf.printBlocks()
		} else {
			fmt.Println("INCORRECT  Reselect")
			continue
		}
	}
}

func (rf *Raft) isLeaderAndConnect() bool {
	network := false
	isLeader := false
	rf.mu.Lock()
	if rf.State == Leader {
		isLeader = true
	}
	if rf.Network == Connect {
		network = true
	}
	rf.mu.Unlock()
	if isLeader == true && network == true {
		return true
	}
	return false
}
