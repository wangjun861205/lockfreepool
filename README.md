# lockfreepool
This is a general concurrency safe pool

##Example
```
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
```
##Usage
1. Define a type which implement Element interface. The Element interface contains 2 methods, `Clean()` will perform
clean when the `Check()` of element return false.
2. Define a function which returns Element, it will be called when the `Check()` of element return false, and the return value
will be put into pool, the old one will be clean and discard
3. Create a new pool by calling `NewPool()`
4. Get element from pool by calling `Pop()`, it will return a Element
5. Do your stuff.
6. Put element back into pool by calling `Push()`. Note: you should pass a *Element to this method
7. If you want to adjust the capicity of pool, you can use `ExtendN()`, `Extend2()`, `SubtractN()`, `SubtractToHalf()`
  
