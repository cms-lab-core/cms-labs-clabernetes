package directruntime_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	clabernetesinternaldirectruntime "github.com/clabernetes/clabernetes/internal/directruntime"
)

var errNoDeviceProcess = errors.New("no device process")

// recordingTerminalOperations records the sequencing a web terminal session must follow: it
// finds the application process, enters its namespaces, and only then becomes the shell.
type recordingTerminalOperations struct {
	processID int
	waitedFor string
	waited    time.Duration
	entered   int
	execed    []string
	err       error
}

func (r *recordingTerminalOperations) WaitForProcessID(
	path string,
	timeout time.Duration,
) (int, error) {
	r.waitedFor = path
	r.waited = timeout

	return r.processID, r.err
}

func (r *recordingTerminalOperations) EnterNamespaces(processID int) error {
	r.entered = processID

	return nil
}

func (r *recordingTerminalOperations) Exec(argv []string) error {
	r.execed = argv

	return nil
}

func TestRunTerminalWaitsForTheDeviceProcessThenBecomesTheShell(t *testing.T) {
	t.Parallel()

	operations := &recordingTerminalOperations{processID: 4242}

	err := clabernetesinternaldirectruntime.RunTerminalWithOperations(
		clabernetesinternaldirectruntime.WebTerminalOptions{
			ProcessIDFile: "/var/run/clabernetes/terminal/node-a.pid",
			Shell:         "cli",
		},
		operations,
	)
	if err != nil {
		t.Fatal(err)
	}

	if operations.waitedFor != "/var/run/clabernetes/terminal/node-a.pid" {
		t.Fatalf("waited for %q", operations.waitedFor)
	}
	if operations.waited != clabernetesinternaldirectruntime.WebTerminalWaitTimeout {
		t.Fatalf("waited for %s, want the default timeout", operations.waited)
	}
	if operations.entered != 4242 {
		t.Fatalf("entered namespaces of %d, want 4242", operations.entered)
	}
	if !slices.Equal(operations.execed, []string{"cli"}) {
		t.Fatalf("execed %v, want the requested shell", operations.execed)
	}
}

func TestRunTerminalDefaultsToAShellWhenNoneIsNamed(t *testing.T) {
	t.Parallel()

	operations := &recordingTerminalOperations{processID: 1}

	err := clabernetesinternaldirectruntime.RunTerminalWithOperations(
		clabernetesinternaldirectruntime.WebTerminalOptions{
			ProcessIDFile: "/run/terminal.pid",
		},
		operations,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(
		operations.execed,
		[]string{clabernetesinternaldirectruntime.WebTerminalShellDefault},
	) {
		t.Fatalf("execed %v, want the default shell", operations.execed)
	}
}

func TestRunTerminalPassesAnExplicitWaitTimeout(t *testing.T) {
	t.Parallel()

	operations := &recordingTerminalOperations{processID: 1}

	err := clabernetesinternaldirectruntime.RunTerminalWithOperations(
		clabernetesinternaldirectruntime.WebTerminalOptions{
			ProcessIDFile: "/run/terminal.pid",
			WaitTimeout:   30 * time.Second,
		},
		operations,
	)
	if err != nil {
		t.Fatal(err)
	}

	if operations.waited != 30*time.Second {
		t.Fatalf("waited for %s, want 30s", operations.waited)
	}
}

// TestRunTerminalRejectsAttach is the load-bearing case: "attach" used to mean "show me the node's
// own container console". A helper container cannot read that pty, so answering it with any shell
// would hand the operator a different session than the one they asked for.
func TestRunTerminalRejectsAttach(t *testing.T) {
	t.Parallel()

	for _, shell := range []string{"attach", "ATTACH", " attach "} {
		operations := &recordingTerminalOperations{processID: 1}

		err := clabernetesinternaldirectruntime.RunTerminalWithOperations(
			clabernetesinternaldirectruntime.WebTerminalOptions{
				ProcessIDFile: "/run/terminal.pid",
				Shell:         shell,
			},
			operations,
		)
		if !errors.Is(err, clabernetesinternaldirectruntime.ErrInvalidTerminal) {
			t.Fatalf("shell %q error = %v, want an invalid terminal error", shell, err)
		}
		if operations.execed != nil {
			t.Fatalf("shell %q execed %v, want no shell at all", shell, operations.execed)
		}
	}
}

func TestRunTerminalRejectsAMultiLineShell(t *testing.T) {
	t.Parallel()

	operations := &recordingTerminalOperations{processID: 1}

	err := clabernetesinternaldirectruntime.RunTerminalWithOperations(
		clabernetesinternaldirectruntime.WebTerminalOptions{
			ProcessIDFile: "/run/terminal.pid",
			Shell:         "bash\nrm -rf /",
		},
		operations,
	)
	if !errors.Is(err, clabernetesinternaldirectruntime.ErrInvalidTerminal) {
		t.Fatalf("error = %v, want an invalid terminal error", err)
	}
}

func TestRunTerminalReportsAnUnresolvableProcessID(t *testing.T) {
	t.Parallel()

	operations := &recordingTerminalOperations{err: errNoDeviceProcess}

	err := clabernetesinternaldirectruntime.RunTerminalWithOperations(
		clabernetesinternaldirectruntime.WebTerminalOptions{
			ProcessIDFile: "/run/terminal.pid",
		},
		operations,
	)
	if !errors.Is(err, errNoDeviceProcess) {
		t.Fatalf("error = %v, want the wait failure", err)
	}
	if operations.execed != nil {
		t.Fatalf("execed %v without a device process", operations.execed)
	}
}

func TestRunTerminalRejectsNilOperations(t *testing.T) {
	t.Parallel()

	err := clabernetesinternaldirectruntime.RunTerminalWithOperations(
		clabernetesinternaldirectruntime.WebTerminalOptions{ProcessIDFile: "/run/terminal.pid"},
		nil,
	)
	if err == nil {
		t.Fatal("nil operations rendered no error")
	}
}

// TestWriteProcessIDFilePublishesThisProcess guards the contract the sidecar depends on: the file
// names a process that is alive right now, and the exec that follows keeps that identity.
func TestWriteProcessIDFilePublishesThisProcess(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "terminal", "node-a.pid")

	if err := clabernetesinternaldirectruntime.WriteProcessIDFile(path); err != nil {
		t.Fatal(err)
	}

	//nolint:gosec // path is created inside this test's private temporary directory.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	published, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}

	if published != os.Getpid() {
		t.Fatalf("published process id %d, want this process %d", published, os.Getpid())
	}
}

func TestWriteProcessIDFileRejectsAnEmptyPath(t *testing.T) {
	t.Parallel()

	err := clabernetesinternaldirectruntime.WriteProcessIDFile("")
	if !errors.Is(err, clabernetesinternaldirectruntime.ErrInvalidTerminal) {
		t.Fatalf("error = %v, want an invalid terminal error", err)
	}
}

// TestWriteProcessIDFilePublishesAtomically covers the rewrite a relaunch performs and the
// property the sidecar's poll depends on: the path only ever holds a complete id, so a reader
// that arrives mid-publish retries instead of reading an empty file it must treat as broken.
func TestWriteProcessIDFilePublishesAtomically(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "node-a.pid")

	// A relaunch re-publishes over the file the previous process left behind.
	if err := os.WriteFile(path, []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := clabernetesinternaldirectruntime.WriteProcessIDFile(path); err != nil {
		t.Fatal(err)
	}

	//nolint:gosec // path is created inside this test's private temporary directory.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	published, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("published id %q is not a complete process id: %v", raw, err)
	}

	if published != os.Getpid() {
		t.Fatalf("published process id %d, want this process %d", published, os.Getpid())
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 || entries[0].Name() != "node-a.pid" {
		t.Fatalf("runtime directory holds temporary publish files: %v", entries)
	}
}
