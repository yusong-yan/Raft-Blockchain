package server

import (
	"fmt"
	"sync"
)

type Chain struct {
	mu     sync.Mutex
	Blocks []string
}

func MakeChain(ApplyCh chan ApplyMsg) *Chain {
	c := &Chain{}
	c.Blocks = []string{}
	go c.AppendBlocks(ApplyCh)
	return c
}

func (c *Chain) AppendBlocks(ApplyCh chan ApplyMsg) {
	for {
		msg := <-ApplyCh
		c.mu.Lock()
		c.Blocks = append(c.Blocks, msg.Command)
		c.mu.Unlock()
	}
}

func (c *Chain) PrintBlocks() {
	c.mu.Lock()
	fmt.Println(c.Blocks)
	c.mu.Unlock()
}
