//go:build !linux

package directruntime

import (
	"fmt"
	"time"
)

type unsupportedTerminalOperations struct{}

func newTerminalOperations() TerminalOperations {
	return unsupportedTerminalOperations{}
}

func (unsupportedTerminalOperations) WaitForProcessID(string, time.Duration) (int, error) {
	return 0, fmt.Errorf("web terminals require Linux")
}

func (unsupportedTerminalOperations) EnterNamespaces(int) error {
	return fmt.Errorf("namespace entry requires Linux")
}

func (unsupportedTerminalOperations) Exec(_ []string) error {
	return fmt.Errorf("web terminals require Linux")
}
