//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func restartProcess(bin string, args []string, env []string) error {
	log.Printf("正在重启进程(windows): %s", bin)
	cmd := exec.Command(bin, args[1:]...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动新进程失败: %w", err)
	}
	os.Exit(0)
	return nil
}
