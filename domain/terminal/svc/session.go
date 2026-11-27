package svc

import (
	"os"
	"os/exec"
	"time"
)

type Size struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func (size Size) Valid() bool {
	return size.Cols > 0 && size.Cols <= 1000 && size.Rows > 0 && size.Rows <= 1000
}

type Session struct {
	pty  *os.File
	cmd  *exec.Cmd
	done chan struct{}
}

func (s *Service) Start(size Size) (*Session, error) {
	cmd, file, err := openShell(s.loginAs, size)
	if err != nil {
		return nil, err
	}
	session := &Session{pty: file, cmd: cmd, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		// Drain the remaining output even if a background job holds the terminal open.
		_ = file.SetReadDeadline(time.Now().Add(time.Second))
		close(session.done)
	}()
	return session, nil
}

func (s *Session) Read(p []byte) (int, error) {
	return s.pty.Read(p)
}

func (s *Session) Write(p []byte) (int, error) {
	return s.pty.Write(p)
}

func (s *Session) Resize(size Size) error {
	return resizePTY(s.pty, size)
}

func (s *Session) Close() {
	// Closing the PTY hangs up the controlling terminal and unblocks its I/O.
	_ = s.pty.Close()
	_ = s.cmd.Process.Kill()
	<-s.done
}
