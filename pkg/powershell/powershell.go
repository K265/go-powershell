package powershell

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/pkg/errors"
)

type IPs interface {
	Exec(cmd string) (stdout string, err error)
	Exit() error
}

type Ps struct {
	cmd    *exec.Cmd
	mutex  sync.Mutex
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

const newline = "\r\n"

func NewPs(attr *syscall.SysProcAttr) (p IPs, err error) {
	ps := Ps{}
	ps.cmd = exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NoExit", "-Command", "-")
	ps.stdin, err = ps.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	ps.stdout, err = ps.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	ps.stderr, err = ps.cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	ps.cmd.SysProcAttr = attr
	err = ps.cmd.Start()
	if err != nil {
		return nil, err
	}

	return &ps, err
}

func (ps *Ps) Exec(cmd string) (string, error) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	stdoutEnd, err := footer()
	if err != nil {
		return "", errors.Errorf("create stdout footer failed: %v", err)
	}
	stderrEnd, err := footer()
	if err != nil {
		return "", errors.Errorf("create stderr footer failed: %v", err)
	}

	c := "Invoke-Command -ScriptBlock {\n"
	c += cmd + "\n"
	c += "}\n"
	c += "\n"
	c += "if ($?) {\n"
	c += "  echo 0\n"
	c += "} else {\n"
	c += "  echo 1\n"
	c += "}\n"
	c += "echo '" + stdoutEnd + "'\n"
	c += "[Console]::Error.WriteLine('" + stderrEnd + "')" + "\n"

	_, err = fmt.Fprintln(ps.stdin, c)
	if err != nil {
		return "", errors.Errorf("send cmd error: %v", err)
	}

	waiter := &sync.WaitGroup{}
	waiter.Add(2)

	stdout := ""
	stderr := ""
	go ps.read(ps.stdout, stdoutEnd, &stdout, waiter)
	go ps.read(ps.stderr, stderrEnd, &stderr, waiter)

	waiter.Wait()

	if len(stdout) > 1 {
		result := stdout[len(stdout)-1]
		if result == '1' {
			return "", errors.Errorf("exit code 1: %s", stderr)
		}

		return strings.Trim(stdout[:len(stdout)-1], newline), nil
	}

	if stderr != "" {
		return "", errors.New(stderr)
	}

	return stdout, nil
}

func (ps *Ps) Exit() error {
	_, err := ps.stdin.Write([]byte("exit" + newline))
	if err != nil {
		return err
	}

	return ps.cmd.Wait()
}

func (ps *Ps) read(r io.ReadCloser, footer string, buffer *string, signal *sync.WaitGroup) {
	defer signal.Done()

	output := ""
	bufSize := 64
	marker := footer + newline

	for {
		buf := make([]byte, bufSize)
		read, err := r.Read(buf)
		if err != nil {
			output = fmt.Sprintf("read [%s] error: %v", footer, err)
			return
		}

		output = output + string(buf[:read])
		if strings.HasSuffix(output, marker) {
			break
		}
	}

	result := strings.TrimSuffix(output, marker)
	result = strings.Trim(result, newline)

	*buffer = result
}

func footer() (string, error) {
	c := 32
	b := make([]byte, c)

	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}
