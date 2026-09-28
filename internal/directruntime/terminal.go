//nolint:err113 // diagnostics are structured one-off errors carrying typed classification.
package directruntime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// WebTerminalShellDefault is the shell a per-node web terminal runs when the node's
	// ttyd-shell names a bare shell name with no path: the launcher's entrypoint resolves it
	// inside the device container's mount namespace, so "bash" only works on images that ship it.
	WebTerminalShellDefault = "/bin/sh"

	// WebTerminalWaitInterval is how often a terminal re-checks for the application container's
	// process id while the device is still starting. The device can take minutes to boot, so this
	// only paces the poll; WebTerminalWaitTimeout bounds the whole wait.
	WebTerminalWaitInterval = time.Second

	// WebTerminalWaitTimeout bounds how long a terminal session waits for the application
	// container to publish its process id. A device image that never launches fails the session
	// rather than leaving a browser spinning forever.
	WebTerminalWaitTimeout = 10 * time.Minute

	webTerminalRuntimeDirectoryMode os.FileMode = 0o750
)

var (
	// ErrInvalidTerminal classifies a web terminal request that cannot be realized.
	ErrInvalidTerminal = errors.New("invalid web terminal request")
	// ErrTerminalProcessID classifies a terminal whose application process id is unusable.
	ErrTerminalProcessID = errors.New("web terminal application process id is unusable")
)

// WebTerminalOptions describes one per-node web terminal session: which application container to
// attach to, and which shell to run there. The device's own filesystem is what the shell resolves
// against, so the shell is a path *inside the device image*.
type WebTerminalOptions struct {
	// ProcessIDFile is the shared-runtime file the application container's launch boundary
	// writes its own process id to before it becomes the device's process.
	ProcessIDFile string
	// Shell is the shell (or any other command) the terminal runs in the device container.
	Shell string
	// WaitTimeout bounds the wait for ProcessIDFile to appear; zero means WebTerminalWaitTimeout.
	WaitTimeout time.Duration
}

// TerminalOperations is the narrow boundary a web terminal needs: find the application
// container's process, join its namespaces, and become the requested command there. It exists so
// the terminal's sequencing is testable without namespaces.
type TerminalOperations interface {
	// WaitForProcessID polls path until it holds a usable process id or the timeout expires.
	WaitForProcessID(path string, timeout time.Duration) (int, error)
	// EnterNamespaces joins the named process's mount, UTS, and IPC namespaces, so the command
	// that follows sees the device's filesystem, hostname, and IPC identity.
	EnterNamespaces(processID int) error
	// Exec replaces this process with argv.
	Exec(argv []string) error
}

// RunTerminal realizes one web terminal session: it waits for the application container to publish
// its process id, joins that process's namespaces, and replaces itself with the requested shell.
// The session therefore runs *inside* the device container's mount namespace while the ttyd/tmux
// front end stays in the launcher container -- which is how a browser session reaches the device
// without a second shell on the node's console.
func RunTerminal(options WebTerminalOptions) error {
	return RunTerminalWithOperations(options, newTerminalOperations())
}

// RunTerminalWithOperations exposes the generic syscall seam for deterministic tests.
func RunTerminalWithOperations(
	options WebTerminalOptions,
	operations TerminalOperations,
) error {
	shell, err := terminalShell(options)
	if err != nil {
		return err
	}

	if operations == nil {
		return errors.New("web terminal operations are nil")
	}

	timeout := options.WaitTimeout
	if timeout <= 0 {
		timeout = WebTerminalWaitTimeout
	}

	processID, err := operations.WaitForProcessID(options.ProcessIDFile, timeout)
	if err != nil {
		return err
	}

	if err = operations.EnterNamespaces(processID); err != nil {
		return err
	}

	return operations.Exec([]string{shell})
}

// terminalShell validates the requested shell. The empty "attach" word is the one value the
// launcher image used to accept for "show me the device's own console" and is rejected with a
// diagnostic rather than silently answering a different question: a container runtime owns the
// pty master an application container's console lives on, so no helper container can read it. The
// device's own console ports (see a node's "ports") are the supported way to reach a console.
func terminalShell(options WebTerminalOptions) (string, error) {
	shell := strings.TrimSpace(options.Shell)
	if shell == "" {
		return WebTerminalShellDefault, nil
	}

	if strings.EqualFold(shell, "attach") {
		return "", fmt.Errorf(
			"%w: ttyd-shell %q is not realizable in a direct device Pod;"+
				" the container runtime owns the application console's pty master --"+
				" name a shell (for example \"bash\" or \"cli\"), or reach the console through the"+
				" node's own exposed ports",
			ErrInvalidTerminal,
			shell,
		)
	}

	if strings.ContainsRune(shell, '\n') {
		return "", fmt.Errorf(
			"%w: ttyd-shell holds a line break", ErrInvalidTerminal,
		)
	}

	return shell, nil
}

// WriteProcessIDFile publishes this process's id at path. The application container's launch
// boundary calls it immediately before it replaces itself with the image's process: the
// replacement keeps the process id, so the file names the device process for as long as the
// device runs, and a terminal session joining those namespaces lands in the device.
//
// The id is written to a sibling temporary file and renamed into place: the sidecar polls this
// path from the moment its container starts, and a plain write would let it observe the file
// between creation and content, which it must read as "not published yet" rather than as a
// corrupt id.
func WriteProcessIDFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%w: process id file path is empty", ErrInvalidTerminal)
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, webTerminalRuntimeDirectoryMode); err != nil {
		return fmt.Errorf("creating the web terminal runtime directory: %w", err)
	}

	temporary, err := os.CreateTemp(directory, ".process-id-*")
	if err != nil {
		return fmt.Errorf("publishing the application process id: %w", err)
	}
	defer func() {
		_ = os.Remove(temporary.Name())
	}()

	if _, err := temporary.WriteString(strconv.Itoa(os.Getpid()) + "\n"); err != nil {
		_ = temporary.Close()

		return fmt.Errorf("publishing the application process id: %w", err)
	}

	if err := temporary.Close(); err != nil {
		return fmt.Errorf("publishing the application process id: %w", err)
	}

	if err := os.Rename(temporary.Name(), path); err != nil {
		return fmt.Errorf("publishing the application process id: %w", err)
	}

	return nil
}
