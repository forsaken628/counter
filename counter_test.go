package limhook

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_counter(t *testing.T) {
	c := counter{
		limit: 2,
		ring:  make([]chan struct{}, 2),
	}

	ch, ok := c.add()
	assert.Nil(t, ch)
	assert.True(t, ok)
	assert.Equal(t, 1, c.flying)

	ch, ok = c.add()
	assert.Nil(t, ch)
	assert.True(t, ok)
	assert.Equal(t, 2, c.flying)

	ch0, ok := c.add()
	assert.NotNil(t, ch0)
	assert.True(t, ok)
	assert.Equal(t, 2, c.flying)

	ch1, ok := c.add()
	assert.NotNil(t, ch1)
	assert.True(t, ok)
	assert.Equal(t, 2, c.flying)

	ch, ok = c.add()
	assert.Nil(t, ch)
	assert.False(t, ok)
	assert.Equal(t, 2, c.flying)

	c.done()
	assert.Equal(t, 2, c.flying)
	select {
	case <-ch0:
	case <-time.After(time.Millisecond):
		assert.Fail(t, "should release")
	}

	ch2, ok := c.add()
	assert.NotNil(t, ch1)
	assert.True(t, ok)
	assert.Equal(t, 2, c.flying)

	c.done()
	assert.Equal(t, 2, c.flying)
	select {
	case <-ch1:
	case <-time.After(time.Millisecond):
		assert.Fail(t, "should release")
	}

	c.done()
	assert.Equal(t, 2, c.flying)
	select {
	case <-ch2:
	case <-time.After(time.Millisecond):
		assert.Fail(t, "should release")
	}

	c.done()
	assert.Equal(t, 1, c.flying)

	c.done()
	assert.Equal(t, 0, c.flying)
}

func Test_counter_remove(t *testing.T) {
	ch0 := make(chan struct{})
	ch1 := make(chan struct{})
	ch2 := make(chan struct{})
	tests := []struct {
		name string
		ring []chan struct{}
		head int
		arg  <-chan struct{}
		want []chan struct{}
	}{
		{
			ring: []chan struct{}{ch0, ch1, ch2},
			head: 0,
			arg:  ch0,
			want: []chan struct{}{ch1, ch2, nil},
		},
		{
			ring: []chan struct{}{ch2, ch0, ch1},
			head: 1,
			arg:  ch2,
			want: []chan struct{}{nil, ch0, ch1},
		},
		{
			ring: []chan struct{}{ch0, ch1, nil},
			head: 2,
			arg:  ch0,
			want: []chan struct{}{ch1, nil, nil},
		},
		{
			ring: []chan struct{}{ch0, ch1, nil},
			head: 2,
			arg:  ch0,
			want: []chan struct{}{ch1, nil, nil},
		},
		{
			ring: []chan struct{}{ch1, nil, ch0},
			head: 1,
			arg:  ch1,
			want: []chan struct{}{nil, nil, ch0},
		},
		{
			ring: []chan struct{}{nil, nil, ch0},
			head: 1,
			arg:  ch0,
			want: []chan struct{}{nil, nil, nil},
		},
		{
			ring: []chan struct{}{ch1, nil, ch0},
			head: 1,
			arg:  ch0,
			want: []chan struct{}{nil, nil, ch1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &counter{
				ring: tt.ring,
				head: tt.head,
			}
			c.remove(tt.arg)
			assert.Equal(t, tt.want, c.ring)
			assert.Nil(t, c.ring[c.head])
		})
	}
}

func TestCounter(t *testing.T) {
	for i := 0; i < 50; i++ {
		c := NewCounter(2, 2)

		ctx3, cancel3 := context.WithCancel(context.Background())
		ch0 := make(chan struct{})
		ch1 := make(chan struct{})
		ch2 := make(chan struct{})

		wg := sync.WaitGroup{}
		wg.Add(4)
		go func() {
			_ = c.Run(context.Background(), func() error {
				<-ch0
				return nil
			})
			wg.Done()
		}()

		go func() {
			_ = c.Run(context.Background(), func() error {
				<-ch1
				return nil
			})
			wg.Done()
		}()

		time.Sleep(time.Millisecond)

		go func() {
			_ = c.Run(ctx3, func() error {
				return nil
			})
			wg.Done()
		}()

		go func() {
			_ = c.Run(context.Background(), func() error {
				<-ch2
				return nil
			})
			wg.Done()
		}()

		fns := []func(){
			cancel3,
			runtime.Gosched,
			runtime.Gosched,
			func() { close(ch0) },
			func() { close(ch1) },
			func() { close(ch2) },
		}

		rand.Shuffle(6, func(i, j int) {
			fns[i], fns[j] = fns[j], fns[i]
		})

		for _, fn := range fns {
			fn()
		}

		wg.Wait()

		assert.Equal(t, 0, c.flying)
	}
}
