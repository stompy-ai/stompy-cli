package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/stompy-ai/stompy-cli/internal/api"
	"github.com/stompy-ai/stompy-cli/internal/channel"
)

// STOMPY-1597 / 3181: a person's board message reaches a running Claude Code
// session. Claude Code starts this as a stdio MCP server; stdout is the MCP
// stream, so everything human-readable goes to stderr.
func newChannelCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "channel",
		Short: "Relay a person's board messages into a running Claude Code session (Channels preview)",
		Long: `Run as a Claude Code channel (research preview). Add to .mcp.json:

  {"mcpServers": {"stompy-channel": {"command": "stompy", "args": ["channel", "-p", "<project>"]}}}

then start Claude Code with:

  claude --dangerously-load-development-channels server:stompy-channel

When a PERSON posts a board message on the project, the session receives an
event naming the project and post id. The event never carries the message: the
agent reads it with board_read, as data written by another user.`,
		Args: cobra.NoArgs,
	}
	c.Flags().Duration("interval", 30*time.Second, "How often to check the board (minimum 10s)")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}
		interval, _ := cmd.Flags().GetDuration("interval")
		if interval < 10*time.Second {
			// minimal: polling stands in for a server push stream (1597 slice 1);
			// the floor keeps one session from hammering the API.
			interval = 10 * time.Second
		}
		read := func(p, since string) ([]api.BoardPost, error) {
			r, err := apiClient.ViewBoard(p, since)
			if err != nil {
				return nil, err
			}
			return r.Posts, nil
		}
		server := channel.NewServer(os.Stdin, os.Stdout, Version)
		watcher := channel.NewWatcher(project, read, time.Now())
		go func() {
			for {
				if ev, ok, err := watcher.Poll(); err != nil {
					fmt.Fprintf(os.Stderr, "stompy channel: board read failed: %v\n", err)
				} else if ok {
					if err := server.Notify(ev); err != nil {
						fmt.Fprintf(os.Stderr, "stompy channel: notify failed: %v\n", err)
					}
				}
				time.Sleep(interval)
			}
		}()
		// Claude Code closes stdin when the session ends; that ends the relay.
		return server.Serve()
	}
	return c
}

func init() { rootCmd.AddCommand(newChannelCmd()) }
