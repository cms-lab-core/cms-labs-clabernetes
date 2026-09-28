//go:build linux

//nolint:err113,gosec // diagnostics are structured one-off errors; /proc reads are fixed paths.
package directruntime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type linuxTerminalOperations struct{}

func newTerminalOperations() TerminalOperations {
	return linuxTerminalOperations{}
}

// WaitForProcessID polls the application container's process id file. The device is mid-boot when
// a browser session arrives, so the file legitimately does not exist yet: a missing file is
// retried until the deadline, while content that is not a process id is a real error and is
// reported immediately.
func (linuxTerminalOperations) WaitForProcessID(path string, timeout time.Duration) (int, error) {
	if strings.TrimSpace(path) == "" {
		return 0, fmt.Errorf("%w: process id file path is empty", ErrInvalidTerminal)
	}

	deadline := time.Now().Add(timeout)

	for {
		processID, err := readProcessIDFile(path)
		if err == nil {
			return processID, nil
		}

		if !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}

		if !time.Now().Before(deadline) {
			return 0, fmt.Errorf(
				"%w: %s did not appear within %s", ErrTerminalProcessID, path, timeout,
			)
		}

		time.Sleep(WebTerminalWaitInterval)
	}
}

// readProcessIDFile reads and validates one process id. The file is written by the application
// container's launch boundary and holds a single decimal process id; a file that exists but holds
// nothing usable is reported as such rather than retried, because nothing else writes it.
func readProcessIDFile(path string) (int, error) {
	//nolint:gosec // the path is a fixed shared-runtime mount, not caller input.
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	trimmed := strings.TrimSpace(string(raw))

	processID, err := strconv.Atoi(trimmed)
	if err != nil || processID <= 0 {
		return 0, fmt.Errorf(
			"%w: %s holds %q", ErrTerminalProcessID, path, trimmed,
		)
	}

	return processID, nil
}

// EnterNamespaces joins the application process's mount, UTS, and IPC namespaces. Mount is what
// matters most -- the shell that follows has to resolve against the device image's filesystem --
// and UTS/IPC keep hostname and shared-memory identity consistent with the device. The network
// namespace is deliberately left alone: a device Pod shares one network namespace already, so
// the terminal is reachable on the Pod address without entering it.
func (linuxTerminalOperations) EnterNamespaces(processID int) error {
	// setns(CLONE_NEWNS) requires a single-threaded process; the session is about to be replaced
	// by exec, so pinning one OS thread for the whole sequence is safe and sufficient.
	runtime.LockOSThread()

	for _, namespace := range []string{"mnt", "uts", "ipc"} {
		if err := enterNamespace(processID, namespace); err != nil {
			return err
		}
	}

	return nil
}

func enterNamespace(processID int, namespace string) error {
	//nolint:gosec // the path is derived from a validated process id.
	target, err := os.Open(fmt.Sprintf("/proc/%d/ns/%s", processID, namespace))
	if err != nil {
		return fmt.Errorf(
			"%w: %s namespace of process %d is unavailable",
			ErrTerminalProcessID,
			namespace,
			processID,
		)
	}
	defer func() { _ = target.Close() }()

	if err = unix.Setns(int(target.Fd()), namespaceFlag(namespace)); err != nil {
		return fmt.Errorf(
			"%w: entering the %s namespace of process %d requires a privileged helper container",
			ErrTerminalProcessID, namespace, processID,
		)
	}

	return nil
}

func namespaceFlag(namespace string) int {
	switch namespace {
	case "mnt":
		return unix.CLONE_NEWNS
	case "uts":
		return unix.CLONE_NEWUTS
	case "ipc":
		return unix.CLONE_NEWIPC
	default:
		return 0
	}
}

func (linuxTerminalOperations) Exec(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("%w: no command to run", ErrInvalidTerminal)
	}

	// The shell name comes from the device image, so it is resolved after EnterNamespaces put
	// this process in the device's mount namespace: a bare "bash" is looked up on the device's
	// PATH, exactly as a shell in the device container would.
	resolved, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf(
			"%w: shell %q is unavailable in the device container", ErrInvalidTerminal, argv[0],
		)
	}

	if err = unix.Exec(resolved, argv, terminalEnvironment()); err != nil {
		return fmt.Errorf("running %q in the device container: %w", argv[0], err)
	}

	return nil
}

// terminalEnvironment hands the session a clean interactive identity: TERM from the environment
// ttyd already set for its pty, and no c9s helper variables, so the device's shell does not
// inherit launcher plumbing.
func terminalEnvironment() []string {
	environment := make([]string, 0, len(os.Environ())+2)

	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "TERM=") || strings.HasPrefix(entry, "NO_COLOR=") {
			continue
		}

		environment = append(environment, entry)
	}

	return append(environment, "TERM="+terminalType)
}

const terminalType = "xterm-256color"
