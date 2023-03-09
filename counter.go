package limhook

import (
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
	head int
}

func (c *counter) add() (<-chan struct{}, bool) {
	if c.flying < c.limit {
		c.flying++
		return nil, true
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
