package jpf

import "sync"

// Usage defines how many tokens were used when making calls to LLMs.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

func (u Usage) Add(u2 Usage) Usage {
	return Usage{
		u.InputTokens + u2.InputTokens,
		u.OutputTokens + u2.OutputTokens,
	}
}

func NewUsageCounter() *UsageCounter {
	return &UsageCounter{
		usage: Usage{},
		lock:  &sync.Mutex{},
	}
}

type UsageCounter struct {
	usage Usage
	lock  *sync.Mutex
}

func (c *UsageCounter) Add(u Usage) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.usage = c.usage.Add(u)
}
