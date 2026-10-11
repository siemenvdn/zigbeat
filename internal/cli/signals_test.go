package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStopSignalsIncludeHangup(t *testing.T) {
	// A closed SSH session must restore the lights like Ctrl-C does.
	if !slices.Contains(StopSignals, os.Signal(syscall.SIGHUP)) || !slices.Contains(StopSignals, os.Signal(syscall.SIGTERM)) {
		t.Fatalf("StopSignals = %v", StopSignals)
	}
}

// restoringChild stops on a hangup, then restores while a closed SSH session
// sends a second hangup and a container stop sends SIGTERM. A second Ctrl-C quits.
func restoringChild() {
	self, _ := os.FindProcess(os.Getpid())
	ctx, stop := StopContext()
	defer stop()
	_ = self.Signal(syscall.SIGHUP)
	<-ctx.Done()
	QuitOnInterrupt()
	_ = self.Signal(syscall.SIGHUP)
	_ = self.Signal(syscall.SIGTERM)
	time.Sleep(200 * time.Millisecond)
	fmt.Println("restored")
	_ = self.Signal(os.Interrupt)
	time.Sleep(2 * time.Second)
	fmt.Println("still running")
	os.Exit(0)
}

// child re-runs the test binary as a child that runs the named test's child part,
// behind prefix (a command such as nohup) when given.
func child(name string, prefix ...string) ([]byte, error) {
	args := append(prefix, os.Args[0], "-test.run=^"+name+"$")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "ZIGBEAT_SIGNAL_CHILD=1")
	return cmd.Output()
}

func TestRestoreSurvivesHangupsButNotASecondCtrlC(t *testing.T) {
	if os.Getenv("ZIGBEAT_SIGNAL_CHILD") == "1" {
		restoringChild()
		return
	}
	out, err := child("TestRestoreSurvivesHangupsButNotASecondCtrlC")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || !exit.Sys().(syscall.WaitStatus).Signaled() || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGINT {
		t.Errorf("want death by the second Ctrl-C, got %v", err)
	}
	if !strings.Contains(string(out), "restored") || strings.Contains(string(out), "still running") {
		t.Errorf("output %q", out)
	}
}

func TestNohupKeepsIgnoringHangup(t *testing.T) {
	if os.Getenv("ZIGBEAT_SIGNAL_CHILD") == "1" {
		ctx, stop := StopContext()
		defer stop()
		self, _ := os.FindProcess(os.Getpid())
		_ = self.Signal(syscall.SIGHUP)
		select {
		case <-ctx.Done():
			fmt.Println("stopped")
		case <-time.After(200 * time.Millisecond):
			fmt.Println("ignored")
		}
		os.Exit(0)
	}
	out, err := child("TestNohupKeepsIgnoringHangup", "nohup")
	if err != nil || strings.TrimSpace(string(out)) != "ignored" {
		t.Errorf("a hangup ignored at start (nohup) must not stop the command: %q, %v", out, err)
	}
}
