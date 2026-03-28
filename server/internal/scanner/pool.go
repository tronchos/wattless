package scanner

import (
	"context"
	"sync"
	"sync/atomic"
)

type BrowserPool struct {
	sem    chan struct{}
	active atomic.Int64
	wg     sync.WaitGroup
}

func NewBrowserPool(limit int) *BrowserPool {
	if limit <= 0 {
		limit = 1
	}

	return &BrowserPool{
		sem: make(chan struct{}, limit),
	}
}

func (p *BrowserPool) Limit() int {
	return cap(p.sem)
}

func (p *BrowserPool) ActiveCount() int {
	return int(p.active.Load())
}

func (p *BrowserPool) Acquire(ctx context.Context) error {
	select {
	case p.sem <- struct{}{}:
		p.active.Add(1)
		p.wg.Add(1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *BrowserPool) TryAcquire() bool {
	select {
	case p.sem <- struct{}{}:
		p.active.Add(1)
		p.wg.Add(1)
		return true
	default:
		return false
	}
}

func (p *BrowserPool) Release() {
	select {
	case <-p.sem:
		p.active.Add(-1)
		p.wg.Done()
	default:
	}
}

func (p *BrowserPool) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
