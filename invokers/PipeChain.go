package invokers

import (
	"bytes"
	"io"
	"log"
	"os/exec"
)

type PipeChain struct {
	stopChan chan int
	stopFlag bool
}

func (p *PipeChain) Run(ib *bytes.Buffer, ob *bytes.Buffer, eb *bytes.Buffer, chain ...*exec.Cmd) (err error) {
	pipes := make([]*io.PipeWriter, len(chain)-1)
	i := 0
	chain[i].Stdin = ib
	for ; i < len(chain)-1; i++ {
		ip, op := io.Pipe()
		pipes[i] = op
		chain[i].Stdout = op
		chain[i].Stderr = eb
		chain[i+1].Stdin = ip
	}
	chain[i].Stdout = ob
	chain[i].Stderr = eb

	p.stopChan = make(chan int)
	p.stopFlag = false

	defer func() {
		if p.stopChan != nil {
			close(p.stopChan)
		}
	}()

	go func() {
		sign := <- p.stopChan
		if sign != 0 {
			p.stopFlag = true
			for idx, cmd := range chain {
				if cmd != nil && cmd.Process != nil {
					if cmd.ProcessState == nil {
						log.Printf("Pipe[%d] - Process[%d] is running, kill it now\n", idx, cmd.Process.Pid)
						procErr := cmd.Process.Kill()
						if procErr != nil {
							log.Printf("Pipe[%d] - Process[%d]: Kill() failed %s\n", idx, cmd.Process.Pid, procErr.Error())
						}
					} else {
						log.Printf("Pipe[%d] - Process[%d] has been finished\n", idx, cmd.Process.Pid)
					}
				} else {
					log.Printf("Pipe[%d] - Process has not been started yet\n", idx)
				}
			}
		}
	}()

	if err = p.next(chain, pipes); err != nil {
		// log or do something with this error
	}
	return err
}

func (p *PipeChain) Stop() {
	if p.stopChan != nil {
		p.stopChan <- 1
	}
}

func (p *PipeChain) next(chain []*exec.Cmd, pipes []*io.PipeWriter) (err error) {
	if chain[0].Process == nil {
		if err = chain[0].Start(); err != nil {
			return err
		}
	}
	if len(chain) > 1 {
		if err = chain[1].Start(); err != nil {
			return err
		}
		defer func() {
			if err == nil {
				pipes[0].Close()
				if !p.stopFlag {
					err = p.next(chain[1:], pipes[1:])
				}
			}
		}()
	}
	return chain[0].Wait()
}
