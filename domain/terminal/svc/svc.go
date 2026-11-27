package svc

import (
	"context"
	"errors"
	"os"
	"os/user"
	"strconv"
	"sync"
)

const maxSessions = 8

var ErrUnavailable = errors.New("terminal is at capacity or shutting down")

type connection struct {
	authSessionID uint
	cancel        context.CancelFunc
}

type Service struct {
	loginAs  string
	mu       sync.Mutex
	active   map[*connection]struct{}
	stopping bool
	done     chan struct{}
}

func New(loginAs string) *Service {
	return &Service{
		loginAs: loginAs,
		active:  make(map[*connection]struct{}),
		done:    make(chan struct{}),
	}
}

func (s *Service) AccountName() string {
	if s.loginAs != "" {
		return s.loginAs
	}
	return currentAccount().Username
}

func (s *Service) Reserve(parent context.Context, authSessionID uint) (context.Context, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping || len(s.active) >= maxSessions {
		return nil, nil, ErrUnavailable
	}
	if err := parent.Err(); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	conn := &connection{authSessionID: authSessionID, cancel: cancel}
	s.active[conn] = struct{}{}
	return ctx, func() {
		cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.active, conn)
		if s.stopping && len(s.active) == 0 {
			close(s.done)
		}
	}, nil
}

func (s *Service) Disconnect(authSessionID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.active {
		if conn.authSessionID == authSessionID {
			conn.cancel()
		}
	}
}

func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if !s.stopping {
		s.stopping = true
		for conn := range s.active {
			conn.cancel()
		}
		if len(s.active) == 0 {
			close(s.done)
		}
	}
	s.mu.Unlock()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func currentAccount() *user.User {
	uid := strconv.Itoa(os.Geteuid())
	account, err := user.LookupId(uid)
	if err == nil {
		return account
	}
	return &user.User{Username: uid, HomeDir: "/"}
}
