package powershell

import (
	"strings"
	"syscall"
	"testing"
)

func TestRun(t *testing.T) {
	ps, err := NewPs(nil)
	noError(t, err)
	cmd := "$ErrorActionPreference = 'Stop'\n"
	cmd += "Get-Item -Path XX:\n"
	cmd += "Get-Item -Path C:\n"
	_, err = ps.Exec(cmd)
	errorContains(t, err, "DriveNotFoundException")
	_, err = ps.Exec("Get-Item -Path C:")
	noError(t, err)
	err = ps.Exit()
	noError(t, err)
	_, err = ps.Exec("Get-Item -Path C:")
	errorContains(t, err, "file already closed")
	ps, err = NewPs(&syscall.SysProcAttr{HideWindow: true})
	noError(t, err)
	_, err = ps.Exec("Get-Item -Path C:")
	noError(t, err)
}

func noError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("Received unexpected error: %v", err)
	}
}

func errorContains(t *testing.T, err error, msg string) {
	if err == nil {
		t.Fatalf("err: %v does not contain: %s", err, msg)
	}

	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("err: %v does not contain: %s", err, msg)
	}
}
