package libspector

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"
)

func parseLStart(s string) (time.Time, error) {
	return time.Parse(layoutLStart, s)
}

func ProcessByPID(pid int) Process {
	return &process{
		pid: pid,
	}
}

type process struct {
	pid         int
	started     *time.Time
	commandName string
	commandArgs string
}

// PID returns the process ID.
func (p *process) PID() int {
	return p.pid
}

// CommandArgs returns the full command line string used to invoke the process.
func (p *process) CommandArgs() (string, error) {
	if p.commandArgs != "" {
		return p.commandArgs, nil
	}

	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", p.PID()), "-o", "args=")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	p.commandArgs = strings.TrimSpace(string(out))
	return p.commandArgs, nil
}

func (p *process) CommandName() (string, error) {
	if p.commandName != "" {
		return p.commandName, nil
	}

	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", p.PID()), "-o", "comm=")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	p.commandName = strings.TrimSpace(string(out))
	return p.commandName, nil
}

// Started uses `ps` to query the start timestamp of the process.
func (p *process) Started() (time.Time, error) {
	if p.started != nil {
		return *p.started, nil
	}

	started := time.Time{}
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", p.PID()), "-o", "lstart=")
	out, err := cmd.Output()
	if err != nil {
		return started, err
	}

	started, err = parseLStart(strings.TrimSpace(string(out)))
	if err != nil {
		return started, err
	}
	p.started = &started
	return started, nil
}

// Libraries returns the dynamically linked libraries used by this process.
func (p *process) Libraries() ([]Library, error) {
	return findLibraryByPID(p.pid)
}

func AllProcesses() ([]Process, error) {
	cmd := exec.Command("ps", "axww", "-o", "lstart:30,pid:10,comm:15,args")
	buf := new(bytes.Buffer)
	cmd.Stdout = buf

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	procs := []Process{}

	scanner := bufio.NewScanner(buf)
	scanner.Scan() // Skip the first line

	for scanner.Scan() {
		line := scanner.Text()

		startTime, err := parseLStart(strings.TrimSpace(line[:30]))
		if err != nil {
			log.Printf("Failed to parse process start time: %v", err)
			continue
		}

		pidString := bytes.NewBufferString(line[31:41])

		var pid int
		if _, err := fmt.Fscanf(pidString, "%d\n", &pid); err != nil {
			log.Printf("Failed to parse process PID: %v", err)
			continue
		}

		commandName := strings.TrimSpace(line[42:57])

		commandArgs := line[58:]

		// Skip our own `ps` process
		if pid != cmd.Process.Pid {
			procs = append(procs, &process{
				pid:         pid,
				commandName: commandName,
				commandArgs: commandArgs,
				started:     &startTime,
			})
		}
	}

	return procs, nil
}

// FindProcess uses `pgrep` to find all processes that match a command.
func FindProcess(command string) ([]Process, error) {
	// TODO: Do we want more flexible querying abilities? Such as full arg
	// substring, or parent pid, etc?
	// UPDATE: I've added `-f` which should give full arg substring matching,
	// but we might still want to add a more flexible set of controls for
	// matching.

	// TODO: Consider getting the full process list at once, including start
	// time with `ps aux` or similar?

	cmd := exec.Command("pgrep", "-f", command)
	buf := new(bytes.Buffer)
	cmd.Stdout = buf

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	procs := []Process{}
	for {
		var pid int
		_, err := fmt.Fscanf(buf, "%d\n", &pid)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		procs = append(procs, &process{pid: pid})
	}

	return procs, nil
}
