// Admin daemon: polls /tmp/xipt every 10s; if present, reads contents and runs
// them as a shell command (root when installed as LaunchDaemon). After run,
// removes /tmp/xipt; on failure writes combined output and error to /tmp/xopt.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

const (
	pollInterval  = 10 * time.Second
	commandTimeout = 5 * time.Minute
	maxCmdBytes   = 256 * 1024
	defaultInPath = "/tmp/xipt"
	defaultOutPath = "/tmp/xopt"
)

func main() {
	inPath := strings.TrimSpace(os.Getenv("ADMIN_DAEMON_IN"))
	if inPath == "" {
		inPath = defaultInPath
	}
	outPath := strings.TrimSpace(os.Getenv("ADMIN_DAEMON_OUT"))
	if outPath == "" {
		outPath = defaultOutPath
	}

	var busy int32
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	log.Printf("admin-daemon polling %s every %v", inPath, pollInterval)

	run := func() {
		if !atomic.CompareAndSwapInt32(&busy, 0, 1) {
			return
		}
		go func() {
			defer atomic.StoreInt32(&busy, 0)
			processOnce(inPath, outPath)
		}()
	}
	run()
	for range ticker.C {
		run()
	}
}

func processOnce(inPath, outPath string) {
	if _, err := os.Stat(inPath); err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("stat %s: %v", inPath, err)
		return
	}

	body, err := os.ReadFile(inPath)
	if err != nil {
		log.Printf("read %s: %v", inPath, err)
		_ = writeOut(outPath, fmt.Sprintf("read %s: %v\n", inPath, err))
		_ = os.Remove(inPath)
		return
	}
	if len(body) > maxCmdBytes {
		msg := fmt.Sprintf("command exceeds max size (%d bytes)\n", maxCmdBytes)
		_ = writeOut(outPath, msg)
		_ = os.Remove(inPath)
		return
	}

	cmdStr := strings.TrimSpace(string(body))
	if cmdStr == "" {
		_ = writeOut(outPath, "empty command\n")
		_ = os.Remove(inPath)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	sh := exec.CommandContext(ctx, "/bin/sh", "-c", cmdStr)
	out, runErr := sh.CombinedOutput()

	if rmErr := os.Remove(inPath); rmErr != nil {
		log.Printf("remove %s: %v", inPath, rmErr)
	}

	if runErr != nil {
		var b strings.Builder
		b.WriteString(runErr.Error())
		b.WriteByte('\n')
		if len(out) > 0 {
			b.Write(out)
		}
		if err := writeOut(outPath, b.String()); err != nil {
			log.Printf("write %s: %v", outPath, err)
		}
	}
}

func writeOut(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
