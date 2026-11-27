//go:build linux || darwin

package svc

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func openShell(loginAs string, size Size) (*exec.Cmd, *os.File, error) {
	account := currentAccount()
	cmd := exec.Command("/bin/bash", "-i")
	if loginAs != "" {
		var err error
		account, err = user.Lookup(loginAs)
		if err != nil {
			return nil, nil, fmt.Errorf("look up terminal account %q: %w", loginAs, err)
		}
		credential, err := shellCredential(account)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve terminal credentials for %q: %w", loginAs, err)
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
	}
	home := account.HomeDir
	if home == "" {
		home = "/"
	}
	cmd.Dir = home
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = "C.UTF-8"
	}
	cmd.Env = []string{
		"HOME=" + home,
		"USER=" + account.Username,
		"LOGNAME=" + account.Username,
		"SHELL=/bin/bash",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TERM=xterm-256color",
		"LANG=" + lang,
	}
	if runtime.GOOS == "darwin" {
		cmd.Env = append(cmd.Env, "BASH_SILENCE_DEPRECATION_WARNING=1")
	}
	master, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: size.Cols, Rows: size.Rows})
	if err != nil {
		return nil, nil, fmt.Errorf("start Bash as %q: %w", account.Username, err)
	}
	defer master.Close()
	file, err := pollablePTY(master)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, nil, fmt.Errorf("configure terminal I/O: %w", err)
	}
	return cmd, file, nil
}

func shellCredential(account *user.User) (*syscall.Credential, error) {
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("parse UID: %w", err)
	}
	gid, err := strconv.ParseUint(account.Gid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("parse GID: %w", err)
	}
	if uid == uint64(os.Geteuid()) && gid == uint64(os.Getegid()) {
		return nil, nil
	}
	groupIDs, err := account.GroupIds()
	if err != nil {
		return nil, err
	}
	groups := make([]uint32, 0, len(groupIDs))
	for _, id := range groupIDs {
		group, err := strconv.ParseUint(id, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parse supplementary GID: %w", err)
		}
		groups = append(groups, uint32(group))
	}
	return &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: groups}, nil
}

func pollablePTY(master *os.File) (*os.File, error) {
	// Give Go's poller a descriptor already in nonblocking mode so a closed
	// connection can interrupt a read or write on the PTY.
	fd, err := unix.FcntlInt(master.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	if err := unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	file := os.NewFile(uintptr(fd), master.Name())
	if err := file.SetDeadline(time.Time{}); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func resizePTY(file *os.File, size Size) error {
	raw, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var resizeErr error
	if err := raw.Control(func(fd uintptr) {
		resizeErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, &unix.Winsize{
			Col: size.Cols,
			Row: size.Rows,
		})
	}); err != nil {
		return err
	}
	return resizeErr
}
