package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/burmaster/openclaw-agentd/internal/config"
)

func newStopCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Send SIGTERM to a running openclaw-agentd expose process",
		Long: `stop reads the PID file written by 'expose' and sends SIGTERM,
triggering a graceful shutdown (tunnel teardown + deregister).

Use --force to send SIGKILL instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop(force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "send SIGKILL instead of SIGTERM")
	return cmd
}

func runStop(force bool) error {
	pidPath, err := config.PIDFilePath()
	if err != nil {
		return fmt.Errorf("finding PID file: %w", err)
	}

	data, err := os.ReadFile(pidPath)
	if os.IsNotExist(err) {
		fmt.Println("No running agent found (no PID file).")
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return fmt.Errorf("invalid PID in file: %s", string(data))
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process %d: %w", pid, err)
	}

	sig := syscall.SIGTERM
	sigName := "SIGTERM"
	if force {
		sig = syscall.SIGKILL
		sigName = "SIGKILL"
	}

	fmt.Printf("→ Sending %s to process %d...", sigName, pid)
	if err := proc.Signal(sig); err != nil {
		fmt.Println(" ✗")
		return fmt.Errorf("sending signal: %w", err)
	}
	fmt.Println(" ✓")

	if !force {
		fmt.Println("   Agent will shut down gracefully (tunnel + deregister).")
	}
	return nil
}
