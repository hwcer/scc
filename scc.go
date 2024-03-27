package scc

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

type handle func(context.Context)

func catch(err error) {
	fmt.Printf("%v", err)
}

func New(ctx context.Context) *SCC {
	if ctx == nil {
		ctx = context.Background()
	}
	s := &SCC{Catch: catch, WaitGroup: sync.WaitGroup{}}
	s.Context, s.cancel = context.WithCancel(ctx)
	s.WaitGroup.Add(1)
	return s
}

// SCC 协程控制器
type SCC struct {
	sync.WaitGroup
	context.Context
	stop   int32
	cancel context.CancelFunc
	Catch  func(error) //异常捕获,默认控制台打印
}

// GO 普通的GO
func (s *SCC) GO(f func()) {
	go func() {
		s.WaitGroup.Add(1)
		defer s.WaitGroup.Done()
		f()
	}()
}

// CGO 带有取消通道的协程
func (s *SCC) CGO(f handle) {
	go func() {
		s.WaitGroup.Add(1)
		defer s.WaitGroup.Done()
		f(context.WithoutCancel(s.Context))
	}()
}

// SGO 使用recover保护主进程
func (s *SCC) SGO(f handle) {
	go func() {
		s.Try(f)
	}()
}

func (s *SCC) Try(f handle) {
	defer func() {
		if e := recover(); e != nil {
			s.Catch(fmt.Errorf("%v\n%v", e, string(debug.Stack())))
		}
	}()
	s.WaitGroup.Add(1)
	defer s.WaitGroup.Done()
	f(context.WithoutCancel(s.Context))
}

// Wait 阻塞模式等待所有协程结束
// 只子主协程中使用
// 请不要在SCC创建的协程中使用，负责会无限等待
func (s *SCC) Wait(timeout time.Duration) (err error) {
	if timeout == 0 {
		s.WaitGroup.Wait()
	} else {
		err = s.Timeout(timeout, func() error {
			s.WaitGroup.Wait()
			return nil
		})
	}
	return
}

// Cancel 关闭所有协程
func (s *SCC) Cancel() bool {
	if !atomic.CompareAndSwapInt32(&s.stop, 0, 1) {
		return false
	}
	s.WaitGroup.Done()
	s.cancel()
	return true
}

// Stopped 判断是否已经关闭
func (s *SCC) Stopped() bool {
	return s.stop > 0
}