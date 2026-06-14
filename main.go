// Admin daemon: polls /tmp/xipt every 10s; if present, reads contents and runs
// them as a shell command (root when installed as LaunchDaemon). After run,
// removes the claimed input file; on failure writes combined output and error
// to /tmp/xopt.<worker>.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	pollInterval   = 10 * time.Second
	commandTimeout = 5 * time.Minute
	maxCmdBytes    = 256 * 1024
	defaultInPath  = "/tmp/xipt"
	defaultOutPath = "/tmp/xopt"
	defaultGoroNum = 5
)

type workerPool struct {
	ids chan int
}

func newWorkerPool(n int) *workerPool {
	p := &workerPool{ids: make(chan int, n)}
	for i := 0; i < n; i++ {
		p.ids <- i
	}
	return p
}

func (p *workerPool) tryAcquire() (int, bool) {
	select {
	case id := <-p.ids:
		return id, true
	default:
		return 0, false
	}
}

func (p *workerPool) release(id int) {
	p.ids <- id
}

func parseGoroNum() int {
	s := strings.TrimSpace(os.Getenv("ADMIN_DAEMON_PROC_NUM"))
	if s == "" {
		return defaultGoroNum
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		log.Printf("invalid ADMIN_DAEMON_PROC_NUM %q, using %d", s, defaultGoroNum)
		return defaultGoroNum
	}
	return n
}

func workerOutPath(base string, wrkNum int) string {
	return fmt.Sprintf("%s.%d", base, wrkNum)
}

func main() {
	inPath := strings.TrimSpace(os.Getenv("ADMIN_DAEMON_IN"))
	if inPath == "" {
		inPath = defaultInPath
	}
	outPath := strings.TrimSpace(os.Getenv("ADMIN_DAEMON_OUT"))
	if outPath == "" {
		outPath = defaultOutPath
	}
	goroNum := parseGoroNum()
	pool := newWorkerPool(goroNum)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	log.Printf("admin-daemon polling %s every %v (max workers %d)", inPath, pollInterval, goroNum)

	run := func() {
		wrkNum, ok := pool.tryAcquire()
		if !ok {
			return
		}
		go func(w int) {
			defer pool.release(w)
			processOnce(inPath, outPath, w)
		}(wrkNum)
	}
	run()
	for range ticker.C {
		run()
	}
}

func processOnce(inPath, outPath string, wrkNum int) {
	outFile := workerOutPath(outPath, wrkNum)
	workPath := fmt.Sprintf("%s.%d", inPath, wrkNum)

	if err := os.Rename(inPath, workPath); err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("worker %d: claim %s: %v", wrkNum, inPath, err)
		return
	}

	body, err := os.ReadFile(workPath)
	if err != nil {
		log.Printf("worker %d: read %s: %v", wrkNum, workPath, err)
		_ = writeOut(outFile, fmt.Sprintf("read %s: %v\n", workPath, err))
		_ = os.Remove(workPath)
		return
	}
	if len(body) > maxCmdBytes {
		msg := fmt.Sprintf("command exceeds max size (%d bytes)\n", maxCmdBytes)
		_ = writeOut(outFile, msg)
		_ = os.Remove(workPath)
		return
	}

	cmdStr := strings.TrimSpace(string(body))
	if cmdStr == "" {
		_ = writeOut(outFile, "empty command\n")
		_ = os.Remove(workPath)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	sh := exec.CommandContext(ctx, "/bin/sh", "-c", cmdStr)
	out, runErr := sh.CombinedOutput()

	if rmErr := os.Remove(workPath); rmErr != nil {
		log.Printf("worker %d: remove %s: %v", wrkNum, workPath, rmErr)
	}

	if runErr != nil {
		var b strings.Builder
		b.WriteString(runErr.Error())
		b.WriteByte('\n')
		if len(out) > 0 {
			b.Write(out)
		}
		if err := writeOut(outFile, b.String()); err != nil {
			log.Printf("worker %d: write %s: %v", wrkNum, outFile, err)
		}
	}
}

func writeOut(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
