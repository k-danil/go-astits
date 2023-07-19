package astits

import "errors"

type wrappingCounter struct {
	value  int
	wrapAt int
}

var ErrCounterOverflow = errors.New("counter overflow")

func newWrappingCounter(wrapAt int) wrappingCounter {
	return wrappingCounter{
		value:  wrapAt + 1,
		wrapAt: wrapAt,
	}
}

func (c *wrappingCounter) get() int {
	return c.value
}

func (c *wrappingCounter) set(v int) error {
	if v > c.wrapAt {
		return ErrCounterOverflow
	}
	c.value = v
	return nil
}

func (c *wrappingCounter) inc() int {
	c.value++
	if c.value > c.wrapAt {
		c.value = 0
	}
	return c.value
}
