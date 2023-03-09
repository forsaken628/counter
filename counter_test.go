package limhook

import (
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
