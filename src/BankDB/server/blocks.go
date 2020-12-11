package server

import (
	"fmt"
	"sync"
)

type Chain struct {
	mu           sync.Mutex
	Blocks       []*Block
	Id           map[string]bool
	BalanceTable map[string]int
}

func MakeChain(ApplyCh chan ApplyMsg) *Chain {
	c := &Chain{}
	c.Blocks = []*Block{}
	c.Id = map[string]bool{}
	c.BalanceTable = map[string]int{}
	go c.AppendBlocks(ApplyCh)
	return c
}

func (c *Chain) AppendBlocks(ApplyCh chan ApplyMsg) {
	for {
		msg := <-ApplyCh
		c.mu.Lock()
		for _, vs := range msg.Command.Trans {
			c.Id[vs.Id] = true
			c.setBalance(vs)
		}
		c.Blocks = append(c.Blocks, msg.Command)
		c.mu.Unlock()
	}
}

func (c *Chain) ExistId(id string) bool {
	c.mu.Lock()
	_, exist := c.Id[id]
	c.mu.Unlock()
	if exist {
		return true
	} else {
		return false
	}
}
func (c *Chain) PrintBlocks() {
	c.mu.Lock()
	fmt.Println("-----------------------------------------")
	fmt.Println("State Machine:")
	for _, vs := range c.Blocks {
		fmt.Print("Block <{")
		for i, v := range vs.Trans {
			fmt.Print(" TRANSACTION", i+1, ": [")
			fmt.Print(v.Sender)
			fmt.Print(" send to ")
			fmt.Print(v.Receiver)
			fmt.Print(" money:")
			fmt.Print(v.Amt)
			fmt.Print("]")
		}
		fmt.Println(" }>")
	}
	fmt.Println("-----------------------------------------\n")
	fmt.Print("Balance Table : ")
	fmt.Println(c.BalanceTable)
	c.mu.Unlock()
}

func (c *Chain) LastBlock() *Block {
	c.mu.Lock()
	size := len(c.Blocks)
	var b *Block
	if size > 0 {
		b = c.Blocks[size-1]
	} else {
		b = &Block{}
	}
	c.mu.Unlock()
	return b
}

func (c *Chain) CheckBalance(name string) int {
	money, exist := c.BalanceTable[name]
	if !exist {
		return 10
	} else {
		return money
	}
}

func (c *Chain) setBalance(tran *Transaction) {
	c.BalanceTable[tran.Sender] = c.CheckBalance(tran.Sender) - tran.Amt
	c.BalanceTable[tran.Receiver] = c.CheckBalance(tran.Receiver) + tran.Amt
}
