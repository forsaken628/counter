package limhook

import (
	"context"
	stdErr "errors"
	"sync"
)

var ErrTooManyRequest = stdErr.New("too many request")

type Config struct {
	Pool   string
	Limit  int
	Buffer int
}

func New(cfg ...Config) *Hook {
	h := Hook{
		m: make(map[string]*counter),
	}
	for _, v := range cfg {
		h.m[v.Pool] = &counter{
			limit: v.Limit,
			ring:  make([]chan struct{}, v.Buffer),
		}
	}
	return &h
}

type Hook struct {
	m  map[string]*counter
	mx sync.Mutex
}

type counter struct {
	limit  int
	flying int

	ring []chan struct{}
	head int // 下一个可用
}

func (c *counter) add() (<-chan struct{}, bool) {
	if c.flying < c.limit {
		c.flying++
		return nil, true
	}

	if len(c.ring) == 0 {
		return nil, false
	}

	ch := c.ring[c.head]
	if ch != nil {
		return nil, false
	}

	ch = make(chan struct{})
	c.ring[c.head] = ch
	c.head++
	if c.head >= len(c.ring) {
		c.head = 0
	}
	return ch, true
}

func (c *counter) done() {
	if c.flying <= 0 {
		panic("neg counter")
	}
	if c.flying < c.limit {
		c.flying--
		return
	}

	n := len(c.ring)
	for i := 0; i < n; i++ {
		j := i + c.head
		if j >= n {
			j -= n
		}

		if c.ring[j] == nil {
			continue
		}

		close(c.ring[j])
		c.ring[j] = nil
		return
	}
	c.flying--
}

func (c *counter) remove(ch <-chan struct{}) {
	n := len(c.ring)
	found := false
	for i := 0; i < n; i++ {
		j := i + c.head
		if j >= n {
			j -= n
		}

		if !found {
			if c.ring[j] == ch {
				c.ring[j] = nil
				found = true
			}
			continue
		}

		if j == 0 {
			c.ring[n-1] = c.ring[0]
		} else {
			c.ring[j-1] = c.ring[j]
		}
		if c.ring[j] == nil {
			break
		}
		if i == n-1 {
			c.ring[j] = nil
		}
	}
	if !found {
		panic("not found")
	}
	c.head--
	if c.head < 0 {
		c.head += n
	}
}

func NewCounter(limit, buffer int) *Counter {
	return &Counter{
		counter: counter{
			limit: limit,
			ring:  make([]chan struct{}, buffer),
		},
	}
}

type Counter struct {
	counter
	mx sync.Mutex
}

func (c *Counter) Run(ctx context.Context, fn func() error) error {
	if c.limit == 0 {
		return fn()
	}
	c.mx.Lock()
	ch, ok := c.add()
	c.mx.Unlock()
	if !ok {
		return ErrTooManyRequest
	}
	defer func() {
		c.mx.Lock()
		c.done()
		c.mx.Unlock()
	}()

	if ch == nil {
		return fn()
	}

	select {
	case <-ctx.Done():
		c.mx.Lock()
		c.remove(ch)
		c.mx.Unlock()
		return ctx.Err()
	case <-ch:
		return fn()
	}
}
