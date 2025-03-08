package worker

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

// createCommand creates an OS-appropriate command to execute
func createCommand(ctx context.Context, cmdStr string) *exec.Cmd {
	var cmd *exec.Cmd
	
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}
	
	return cmd
}

// getExitCode extracts the exit code from an exec error
func getExitCode(err error) int {
	if err == nil {
		return 0
	}
	
	// Try to extract exit code from error
	if exitErr, ok := err.(*exec.ExitError); ok {
		// On Unix systems
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
		
		// For Windows compatibility
		return exitErr.ExitCode()
	}
	
	// If we couldn't determine the exit code, return a generic error code
	return 1
}

// isTimeout checks if the error is a timeout error
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for context deadline exceeded error
	if err == context.DeadlineExceeded {
		return true
	}
	
	// Check if the error string contains timeout indicators
	if strings.Contains(err.Error(), "timeout") || 
	   strings.Contains(err.Error(), "deadline exceeded") {
		return true
	}
	
	return false
} 