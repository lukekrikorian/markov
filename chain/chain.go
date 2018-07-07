package chain

import (
	"math/rand"
	"strings"
)

type prefix []string

func (p *prefix) String() string {
	return strings.Join(*p, " ")
}

func (p prefix) Shift(word string) {
	copy(p, p[1:])
	p[len(p)-1] = word
}

// Chain describes a markov chain
type Chain struct {
	chain     map[string][]string
	prefixLen int
}

// NewChain generates a new markov chain
func NewChain(prefixLen int) *Chain {
	return &Chain{make(map[string][]string), prefixLen}
}

// AddComment builds the markov chain from a comment
func (c *Chain) AddComment(comment string) {
	split := strings.Split(comment, " ")
	p := make(prefix, c.prefixLen)
	for _, w := range split {
		key := p.String()
		c.chain[key] = append(c.chain[key], w)
		p.Shift(w)
	}
}

// Generate returns a randomly generated markov string
func (c *Chain) Generate(n int) string {
	p := make(prefix, c.prefixLen)
	var words []string
	for i := 0; i < n; i++ {
		choices := c.chain[p.String()]
		if len(choices) == 0 {
			break
		}
		next := choices[rand.Intn(len(choices))]
		words = append(words, next)
		p.Shift(next)
	}
	return strings.Join(words, " ")
}
