package lockfreepool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

var ErrOverRange error = errors.New("The number to subtract is more greater than pool capicity")

//The elements you want to store into pool, must implement Element interface
type Element interface {
	//Check() will report the element status, if fine return true, else return false. pool will call Check() when you push element
	//back to pool, if it return false, the pool will call newFunc() to generate a new element and push the new into itself
	Check() bool
	//Clean() will perform clean working. Pool will call it when the element not fine
	Clean()
}

type Pool struct {
	length   uint64
	count    uint64
	list     []*Element
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	pause    *chan struct{}
	subtract *chan struct{}
	//After pool successfully closed, Done will be closed to notify outer.
	Done    chan struct{}
	newFunc func() Element
}

// NewPool return a *Pool. It receive 3 parameters. length is the pool initial capicity. ctx is a context.Context, it must not be
// nil, you can close a pool by calling the cancel() pairs of the ctx. newFunc is a func which return Element, some elements corrupt it
// will be called by pool to generate a new element.
func NewPool(length uint64, ctx context.Context, newFunc func() Element) *Pool {
	newCtx, cancel := context.WithCancel(ctx)
	list := make([]*Element, length)
	for i := 0; i < int(length); i++ {
		e := newFunc()
		list[i] = &e
	}
	pause := make(chan struct{})
	subtract := make(chan struct{})
	done := make(chan struct{})
	pool := &Pool{
		length:   length,
		count:    0,
		list:     list,
		ctx:      newCtx,
		cancel:   cancel,
		Done:     done,
		pause:    &pause,
		subtract: &subtract,
		newFunc:  newFunc,
	}
	return pool
}

//GetLength returns the length of pool
func (p *Pool) GetLength() uint64 {
	return p.length
}

//Pop returns Element, if the pool has been closed, it will return nil.
func (p *Pool) Pop() Element {
	p.wg.Add(1)
	count := atomic.AddUint64(&(p.count), 1)
	index := count % p.length
	pointer := (*unsafe.Pointer)(unsafe.Pointer(&(p.list[index])))
	for {
		select {
		case <-p.ctx.Done():
			p.wg.Done()
			return nil
		case <-*p.pause:
			continue
		case <-*p.subtract:
			index = count % p.length
			pointer = (*unsafe.Pointer)(unsafe.Pointer(&(p.list[index])))
			continue
		default:
			elem := atomic.SwapPointer(pointer, nil)
			if elem == nil {
				continue
			}
			ep := (*Element)(elem)
			return *ep
		}
	}
}

//Push receive a *Element and store it.
//Note: never push a nil *Element, it will lead to panic
func (p *Pool) Push(elem *Element) {
	var count uint64
	if !(*elem).Check() {
		(*elem).Clean()
		ne := p.newFunc()
		elem = &ne
	}
	elemPointer := *(*unsafe.Pointer)(unsafe.Pointer(&elem))
	for {
		select {
		case <-p.ctx.Done():
			(*elem).Clean()
			p.wg.Done()
			return
		case <-*p.pause:
			continue
		case <-*p.subtract:
			(*elem).Clean()
			p.wg.Done()
			return
		default:
			index := count % p.length
			oldPointer := (*unsafe.Pointer)(unsafe.Pointer(&(p.list[index])))
			ok := atomic.CompareAndSwapPointer(oldPointer, unsafe.Pointer(nil), elemPointer)
			if !ok {
				count += 1
				continue
			}
			p.wg.Done()
			return
		}
	}
}

//ExtendN makes the capicity of pool growing to (p.length + n)
func (p *Pool) ExtendN(n uint64) {
	newList := make([]*Element, p.length+n)
	newPause := make(chan struct{})
	newPausePointer := unsafe.Pointer(&newPause)
	oldPausePPointer := (*unsafe.Pointer)(unsafe.Pointer(&p.pause))
	close(*p.pause)
	copy(newList, p.list)
	for i := p.length; i < p.length+n; i++ {
		e := p.newFunc()
		newList[i] = &e
	}
	p.list = newList
	p.length += n
	atomic.StorePointer(oldPausePPointer, newPausePointer)
}

//Extend2 makes the capicity of pool growing to twice
func (p *Pool) Extend2() {
	newList := make([]*Element, p.length*2)
	newPause := make(chan struct{})
	newPausePointer := unsafe.Pointer(&newPause)
	oldPausePPointer := (*unsafe.Pointer)(unsafe.Pointer(&p.pause))
	close(*p.pause)
	copy(newList, p.list)
	for i := p.length; i < p.length*2; i++ {
		e := p.newFunc()
		newList[i] = &e
	}
	p.list = newList
	p.length *= 2
	atomic.StorePointer(oldPausePPointer, newPausePointer)
}

//Subtract makes the capicity of pool subtract to (p.length - n). 0 < n < p.length
func (p *Pool) SubtractN(n uint64) error {
	if n >= p.length {
		return ErrOverRange
	}
	close(*p.subtract)
	newList := p.list[:p.length-n]
	newSubtract := make(chan struct{})
	for i, elem := range newList {
		if elem == nil {
			newElem := p.newFunc()
			newList[i] = &newElem
		}
	}
	newLength := p.length - n
	p.list = newList
	atomic.StoreUint64(&(p.length), newLength)
	time.Sleep(500 * time.Millisecond)
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&p.subtract)), unsafe.Pointer(&newSubtract))
	return nil
}

//SubtractToHalf makes the capicity of pool to subtract to (p.length / 2)
func (p *Pool) SubtractToHalf() {
	newLength := p.length / 2
	close(*p.subtract)
	newList := p.list[:newLength]
	newSubtract := make(chan struct{})
	for i, elem := range newList {
		if elem == nil {
			newElem := p.newFunc()
			newList[i] = &newElem
		}
	}
	p.list = newList
	atomic.StoreUint64(&(p.length), newLength)
	time.Sleep(500 * time.Millisecond)
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&p.subtract)), unsafe.Pointer(&newSubtract))
}

//Close pool
func (p *Pool) Close() {
	p.cancel()
	p.wg.Wait()
	close(p.Done)
	fmt.Println("close success")
}
