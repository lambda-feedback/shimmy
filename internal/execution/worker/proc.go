package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/lambda-feedback/shimmy/internal/execution/models"
	"go.uber.org/zap"
)

type proc[I, M, O any] struct {
	pid         int
	termination chan error
	stdout      io.ReadCloser
	stderr      io.ReadCloser
	stdin       io.WriteCloser

	msgid     int
	msgidLock sync.Mutex

	log *zap.Logger
}

func startProc[I, M, O any](config StartConfig, log *zap.Logger) (*proc[I, M, O], error) {
	cmd := exec.Command(config.Cmd, config.Args...)

	if config.Env != nil {
		env := make([]string, 0, len(config.Env))
		for k, v := range config.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	if config.Cwd != "" {
		cmd.Dir = config.Cwd
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	log = log.Named("worker_proc").With(zap.Int("pid", cmd.Process.Pid))

	process := &proc[I, M, O]{
		pid:         cmd.Process.Pid,
		termination: make(chan error),
		stdout:      stdout,
		stderr:      stderr,
		stdin:       stdin,
		log:         log,
	}

	go func() {
		// block until the process exits
		err := cmd.Wait()

		// report the exit error to the caller
		process.termination <- err

		// close the termination channel
		close(process.termination)
	}()

	return process, nil
}

func (p *proc[I, M, O]) Terminate(timeout time.Duration) error {
	// terminate should report success if the process terminated
	// by the time supervisor receives the request.
	select {
	case <-p.termination:
		p.log.Debug("process already terminated")
		return nil
	default:
		// continue
	}

	p.kill(syscall.SIGTERM)

	return p.waitForTermination(timeout)
}

func (p *proc[I, M, O]) Kill(timeout time.Duration) error {
	// kill should report success if the process terminated by the time
	// supervisor receives the request.
	select {
	case <-p.termination:
		p.log.Debug("process already terminated")
		return nil
	default:
		// continue
	}

	// kill the process
	p.kill(syscall.SIGKILL)

	return p.waitForTermination(timeout)
}

func (p *proc[I, M, O]) Wait() error {
	return p.waitForTermination(0)
}

func (p *proc[I, M, O]) WaitFor(timeout time.Duration) error {
	return p.waitForTermination(timeout)
}

func (p *proc[I, M, O]) waitForTermination(timeout time.Duration) error {
	// if timeout is < 0, don't wait for the process to exit
	if timeout < 0 {
		return nil
	}

	// if timeout is 0, wait indefinitely
	if timeout == 0 {
		<-p.termination
		return nil
	}

	// block until either:
	//  * the main process exits (parent ctx is cancelled)
	//  * the child process exits (w.termination is closed)
	//  * the timeout is reached
	select {
	case <-p.termination:
		return nil
	case <-time.After(timeout):
		return ErrKillTimeout
	}
}

func (p *proc[I, M, O]) kill(signal syscall.Signal) {
	log := p.log.With(zap.Stringer("signal", signal))

	// close stdin before killing the process, to
	// avoid the process hanging on input
	if err := p.stdin.Close(); err != nil {
		log.Error("close stdin failed", zap.Error(err))
	}

	log.Info("sending signal")

	// best effort, ignore errors
	if err := p.sendKillSignal(signal); err != nil {
		log.Error("stop failed", zap.Error(err))
	}
}

func (p *proc[I, M, O]) sendKillSignal(signal syscall.Signal) error {
	if pgid, err := syscall.Getpgid(p.pid); err == nil {
		// Negative pid sends signal to all in process group
		return syscall.Kill(-pgid, signal)
	} else {
		return syscall.Kill(p.pid, signal)
	}
}

func (p *proc[I, M, O]) Write(ctx context.Context, data models.Message[I, M]) (int, error) {
	reqID := p.nextMsgID()

	req := Request[I, M]{
		ID:   reqID,
		Data: data.GetPayload(),
		Meta: data.GetMeta(),
	}

	// write encoded message to process stdin
	if err := json.NewEncoder(p.stdin).Encode(req); err != nil {
		return 0, err
	}

	return reqID, nil
}

func (p *proc[I, M, O]) Close() error {
	if err := p.stdin.Close(); err != nil {
		return err
	}

	return nil
}

func (p *proc[I, M, O]) nextMsgID() int {
	p.msgidLock.Lock()
	defer p.msgidLock.Unlock()

	id := p.msgid
	p.msgid++

	return id
}

func (p *proc[I, M, O]) Read(ctx context.Context, timeout time.Duration) (Response[O], error) {
	var result Response[O]

	// Create a channel to signal the completion of reading and decoding.
	done := make(chan error, 1)

	// Start a goroutine to read from stdout and decode the JSON.
	go func() {
		if err := json.NewDecoder(p.stdout).Decode(&result); err != nil {
			done <- err
			return
		}
		done <- nil
	}()

	if timeout > 0 {
		// Create a context with the specified timeout.
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		ctx = timeoutCtx
	}

	select {
	case <-ctx.Done(): // Context was cancelled or timed out
		return result, ctx.Err()
	case err := <-done: // Finished reading and decoding
		return result, err
	}
}
