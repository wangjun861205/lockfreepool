package lockfreepool

import (
	"context"
	"fmt"
	"testing"
)

type S struct {
	S string
}

func (s *S) Clean() {}

func (s *S) Check() bool {
	return true
}

func NewS() Element {
	return &S{"hello"}
}

func TestPool(t *testing.T) {
	pool := NewPool(20, context.Background(), NewS)
	fmt.Println(pool.list)
	for i := 0; i < 100; i++ {
		go func() {
			e := pool.Pop()
			if e != nil {
				s := e.(*S)
				fmt.Println(s)
			} else {
				fmt.Println("nil")
				return
			}
			pool.Push(&e)
		}()
	}
	// pool.ExtendN(20)
	// pool.SubtractN(15)
	pool.SubtractToHalf()
	// time.Sleep(3 * time.Second)
	pool.Close()
	<-pool.Done
	fmt.Println(pool.list)
	fmt.Println("success")
}
