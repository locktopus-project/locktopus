package internal

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewChainMutex_SequentialOrder_LockBeforeChain(t *testing.T) {
	order := make([]int, 0)
	wg := sync.WaitGroup{}
	wg.Add(4)

	var cm1, cm2, cm3, cm4 *ChainMutex

	ch1 := make(chan string)
	ch2 := make(chan string)
	ch3 := make(chan string)
	ch4 := make(chan string)

	go func() {
		ch1 <- "start"
		cm1 = NewChainMutex(LockTypeWrite)

		cm1.Lock()
		cm2 = cm1.Chain(LockTypeWrite)
		ch1 <- "end"

		order = append(order, 1)
		cm1.Unlock()

		wg.Done()
	}()

	go func() {
		ch2 <- "start"

		cm2.Lock()
		cm3 = cm2.Chain(LockTypeRead)

		ch2 <- "end"

		order = append(order, 2)
		cm2.Unlock()

		wg.Done()
	}()

	go func() {
		ch3 <- "start"

		cm3.Lock()
		cm4 = cm3.Chain(LockTypeWrite)

		ch3 <- "end"

		order = append(order, 3)
		fmt.Println("cm3 unlock")
		cm3.Unlock()

		wg.Done()
	}()

	go func() {
		ch4 <- "start"

		cm4.Lock()

		order = append(order, 4)
		fmt.Println("cm4 unlock")
		cm4.Unlock()

		wg.Done()
	}()

	<-ch1
	<-ch1
	<-ch2
	<-ch2
	<-ch3
	<-ch3
	<-ch4

	wg.Wait()

	if fmt.Sprintf("%v", order) != "[1 2 3 4]" {
		t.Errorf("Expected order to be [1 2 3 4], got %v", order)
	}
}

func TestNewChainMutex_SequentialOrder_ChainBeforeLock(t *testing.T) {
	order := make([]int, 0)
	wg := sync.WaitGroup{}
	wg.Add(4)

	var cm1, cm2, cm3, cm4 *ChainMutex

	cm1 = NewChainMutex(LockTypeWrite)
	cm2 = cm1.Chain(LockTypeWrite)
	cm3 = cm2.Chain(LockTypeRead)
	cm4 = cm3.Chain(LockTypeWrite)

	go func() {
		cm4.Lock()
		order = append(order, 4)
		cm4.Unlock()

		wg.Done()
	}()

	go func() {
		cm3.Lock()
		order = append(order, 3)
		cm3.Unlock()

		wg.Done()
	}()

	go func() {
		cm2.Lock()
		order = append(order, 2)
		cm2.Unlock()

		wg.Done()
	}()

	go func() {
		cm1.Lock()
		order = append(order, 1)
		cm1.Unlock()

		wg.Done()
	}()

	wg.Wait()

	if fmt.Sprintf("%v", order) != "[1 2 3 4]" {
		t.Errorf("Expected order to be [1 2 3 4], got %v", order)
	}
}

func TestNewChainMutex_ReadsMustBeParallel(t *testing.T) {
	order := make([]int, 0)
	wg := sync.WaitGroup{}
	wg.Add(3)

	var cm1, cm2, cm3, cm4, cm5 *ChainMutex

	cm1 = NewChainMutex(LockTypeWrite)
	cm2 = cm1.Chain(LockTypeRead)
	cm3 = cm2.Chain(LockTypeRead)
	cm4 = cm3.Chain(LockTypeRead)
	cm5 = cm4.Chain(LockTypeWrite)

	go func() {
		cm5.Lock()
		order = append(order, 5)
		cm5.Unlock()

		wg.Done()
	}()

	go func() {
		cm3.Lock()
		order = append(order, 3)
		cm3.Unlock()

		time.Sleep(time.Millisecond * 10)

		cm4.Lock()
		order = append(order, 4)
		cm4.Unlock()

		time.Sleep(time.Millisecond * 10)

		cm2.Lock()
		order = append(order, 2)
		cm2.Unlock()

		wg.Done()
	}()

	go func() {
		cm1.Lock()
		order = append(order, 1)
		cm1.Unlock()

		wg.Done()
	}()

	wg.Wait()

	if fmt.Sprintf("%v", order) != "[1 3 4 2 5]" {
		t.Errorf("Expected order to be [1 3 4 2 5], got %v", order)
	}
}
