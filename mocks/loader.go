package mocks

import (
	"fmt"

	"github.com/onsi/gomega/gbytes"

	"github.com/sclevine/forge/engine"
)

type MockLoader struct {
	Err      error
	Out      *gbytes.Buffer
	Reply    map[string]string
	Progress chan engine.Progress
}

func NewMockLoader() *MockLoader {
	return &MockLoader{
		Out:      gbytes.NewBuffer(),
		Reply:    map[string]string{},
		Progress: make(chan engine.Progress, 1),
	}
}

func (m *MockLoader) Loading(message string, progress <-chan engine.Progress) error {
	fmt.Fprintln(m.Out, "Loading: "+message)
	for p := range progress {
		m.Progress <- p
	}
	return nil
}
