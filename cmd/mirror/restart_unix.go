//go:build !windows

package main

import (
	"log"
	"os"
	"syscall"
)

func restartProcess(bin string, args []string, env []string) error {
	log.Printf("正在重启进程: %s", bin)
	return syscall.Exec(bin, args, env)
}
