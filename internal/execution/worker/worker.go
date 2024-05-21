package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

type ExitEvent struct {
	// Code is the exit code of the process
	Code *int

	// Signal is the signal that caused the process to exit
	Signal *int

	// Stderr is the stderr output of the process
	Stderr string
}

// Success returns true if the process exited successfully.
func (e ExitEvent) Success() bool {
	return e.Code != nil && *e.Code == 0
}

// String returns a string representation of the exit event.
func (e ExitEvent) String() string {
	var code string
	if e.Code != nil {
		code = fmt.Sprintf("%d", *e.Code)
	} else {
		code = "(nil)"
	}

	var signal string
	if e.Signal != nil {
		signal = fmt.Sprintf("%d", *e.Signal)
	} else {
		signal = "(nil)"
	}

	stderr := strings.Trim(strings.ReplaceAll(e.Stderr, "\n", " "), " ")

	return fmt.Sprintf("code=%v, signal=%s, stderr=%s", code, signal, stderr)
}

type Worker interface {
	Start(context.Context) error
	Stream() (io.ReadWriteCloser, error)
	Terminate() error
	Wait(context.Context) (ExitEvent, error)
	WaitFor(context.Context, time.Duration) (ExitEvent, error)
}

type ProcessWorker struct {
	mu  sync.Mutex
	cmd *exec.Cmd

	wait chan struct{}
	done chan struct{}
	exit chan ExitEvent

	stderr   bytes.Buffer
	stderrWg sync.WaitGroup

	log *zap.Logger
}

func NewProcessWorker(ctx context.Context, config StartConfig, log *zap.Logger) *ProcessWorker {
	// start process w/ context, so the process is SIGKILL'd when
	// the context is cancelled. This ensures we don't have zombie
	// processes when normal termination fails.
	cmd := createCmd(ctx, config)

	return &ProcessWorker{
		cmd:  cmd,
		wait: make(chan struct{}),
		done: make(chan struct{}),
		exit: make(chan ExitEvent),
		log:  log.Named("worker"),
	}
}

var _ Worker = (*ProcessWorker)(nil)

// Start starts the worker process.
func (w *ProcessWorker) Start(ctx context.Context) error {
	// synchronize access to the process
	w.mu.Lock()
	defer w.mu.Unlock()

	w.log.With(
		zap.Strings("args", w.cmd.Args),
		zap.String("cwd", w.cmd.Dir),
		zap.Strings("env", w.cmd.Environ()),
	).Debug("starting process")

	// return if the worker is already started
	if w.cmd.Process != nil {
		return ErrWorkerAlreadyStarted
	}

	// exit early if the context is already cancelled
	if ctx.Err() != nil {
		return fmt.Errorf("failed to start process: %w", ctx.Err())
	}

	// create a pipe for stderr
	stderrPipe, err := w.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// wait for the process to terminate,
	// and send the exit event to the channel
	go func() {
		// first, wait for `Wait` to be called
		<-w.wait

		// wait for stderr to be read
		w.stderrWg.Wait()

		// block until the process exits
		err = w.cmd.Wait()

		// get the exit event
		evt := getExitEvent(err, w.stderr.String())

		// log the exit event
		if !evt.Success() {
			w.log.With(
				zap.Any("code", evt.Code),
				zap.Any("signal", evt.Signal),
				zap.String("stderr", evt.Stderr),
			).Warn("process exited with non-zero code")
		}

		// send the exit event to the channel
		w.exit <- evt

		// close the exit channel
		close(w.exit)

		// close the done channel
		close(w.done)
	}()

	// read from stderr in a separate goroutine
	w.stderrWg.Add(1)
	go func() {
		defer w.stderrWg.Done()

		// read from stderr and save it for later use
		_, err := io.Copy(&w.stderr, stderrPipe)
		if err != nil && err != io.EOF {
			w.log.Warn("failed to read from stderr", zap.Error(err))
		}
	}()

	// start the process
	if err := w.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	return nil
}

// Wait blocks until the process exits. The method returns an ExitEvent
// object that contains the exit status of the process. If the process
// is already terminated, the method returns immediately.
//
// Any of `Wait` or `WaitFor` are intended to be called only once.
// Subsequent calls will return an error.
func (w *ProcessWorker) Wait(ctx context.Context) (ExitEvent, error) {
	// close the wait channel to signal that `Wait` has been called
	select {
	case <-w.wait:
		return ExitEvent{}, errors.New("wait has already been called")
	default:
		close(w.wait)
	}

	select {
	case <-ctx.Done():
		return ExitEvent{}, ctx.Err()
	case exitEvent := <-w.exit:
		return exitEvent, nil
	}
}

// WaitFor blocks until the process exits or the timeout is reached.
// The method returns an ExitEvent that contains the exit status. If
// the process is already terminated, the method returns immediately.
//
// Any of `Wait` or `WaitFor` are intended to be called only once.
// Subsequent calls will return an error.
func (w *ProcessWorker) WaitFor(
	ctx context.Context,
	deadline time.Duration,
) (ExitEvent, error) {
	var waitCtx context.Context
	var cancel context.CancelFunc

	if deadline <= 0 {
		waitCtx, cancel = context.WithCancel(ctx)
	} else {
		waitCtx, cancel = context.WithTimeout(ctx, deadline)
	}

	defer cancel()

	return w.Wait(waitCtx)
}

// Terminate sends a SIGTERM signal to the worker process. The method
// returns immediately, without waiting for the process to stop.
func (w *ProcessWorker) Terminate() error {
	return w.halt(syscall.SIGTERM)
}

// Terminate sends a SIGKILL signal to the worker process. The method
// returns immediately, without waiting for the process to stop.
func (w *ProcessWorker) Kill() error {
	return w.halt(syscall.SIGKILL)
}

func (w *ProcessWorker) halt(signal syscall.Signal) error {
	if w.cmd.Process == nil {
		return errors.New("process is not running")
	}

	log := w.log.With(zap.Stringer("signal", signal))

	// close stdin before killing the process, to
	// avoid the process hanging on input
	// if err := p.stdin.Close(); err != nil {
	// 	log.Warn("close stdin failed", zap.Error(err))
	// }

	// best effort, ignore errors
	if err := w.sendKillSignal(signal); err != nil {
		log.Warn("sending signal failed", zap.Error(err))
	}

	return nil
}

func (p *ProcessWorker) sendKillSignal(signal syscall.Signal) error {
	if pgid, err := syscall.Getpgid(p.cmd.Process.Pid); err == nil {
		// Negative pid sends signal to all in process group
		return syscall.Kill(-pgid, signal)
	} else {
		return syscall.Kill(p.cmd.Process.Pid, signal)
	}
}

func (w *ProcessWorker) Pid() int {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cmd == nil || w.cmd.Process == nil {
		return 0
	}

	return w.cmd.Process.Pid
}

// MARK: - Helpers

func getExitEvent(err error, stderr string) ExitEvent {
	var cell int
	var exitStatus *int
	var signo *int

	if err == nil {
		// the process exited successfully, set the exit code to 0
		exitStatus = &cell
	} else if exitError, ok := err.(*exec.ExitError); ok {
		// the process exited with an error
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
			if code := status.ExitStatus(); code >= 0 {
				// the process exited with an exit code
				cell = code
				exitStatus = &cell
			} else {
				// the process was terminated by a signal
				cell = int(status.Signal())
				signo = &cell
			}
		}
	}

	if signo == nil && exitStatus == nil {
		// could not determine the exit status or signal,
		// set exit status to 1
		cell = 1
		exitStatus = &cell
	}

	return ExitEvent{
		Code:   exitStatus,
		Signal: signo,
		Stderr: stderr,
	}
}

// Stream returns a `io.ReadWriteCloser` that can be used to
// read from and write to the process' stdout and stdin.
//
// The stream has to be created before the
func (w *ProcessWorker) Stream() (io.ReadWriteCloser, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	stdin, err := w.cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := w.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	return &iostream{
		stdout: stdout,
		stdin:  stdin,
	}, nil
}

type iostream struct {
	stdout io.ReadCloser
	stdin  io.WriteCloser
}

func (s *iostream) Read(p []byte) (int, error) {
	return s.stdout.Read(p)
}

func (s *iostream) Write(p []byte) (int, error) {
	return s.stdin.Write(p)
}

func (s *iostream) Close() error {
	// we only close stdin, as stdout is closed by the process
	// TODO: check if we need to close stdout as well
	return s.stdin.Close()
}

func createCmd(ctx context.Context, config StartConfig) *exec.Cmd {
	// start process w/ context, so the process is SIGKILL'd when
	// the context is cancelled. This ensures we don't have zombie
	// processes when normal termination fails.
	cmd := exec.CommandContext(ctx, config.Cmd, config.Args...)

	env := os.Environ()
	if config.Env != nil {
		env = append(env, config.Env...)
	}
	cmd.Env = env

	if config.Cwd != "" {
		cmd.Dir = config.Cwd
	}

	// TODO: we open all pipes here. make sure to read from all of them,
	// as we could run into deadlocks otherwise, if the system's stdout
	// or stderr buffers run full.

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	return cmd
}
