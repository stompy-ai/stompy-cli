package cmd

import (
	"fmt"
	"time"

	"github.com/banton/stompy-cli/internal/api"
	"github.com/spf13/cobra"
)

func newBoardCmd() *cobra.Command {
	board := &cobra.Command{Use: "board", Short: "Read and write ephemeral project coordination (stored data)"}
	post := &cobra.Command{Use: "post", Short: "Post a status or message; author identity comes from authentication", Args: cobra.NoArgs}
	post.Flags().String("kind", "status", "status or message")
	post.Flags().String("body", "", "Plain-text post, 1-500 characters")
	post.Flags().String("agent-label", "", "Optional caller-supplied label; not authenticated identity")
	post.Flags().StringArray("ref", nil, "Ticket/topic identifier; repeat for multiple refs")
	post.Flags().Duration("ttl", 0, "Hard lifetime, at most 720h; default 24h status or 168h message")
	post.RunE = func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}
		body, _ := cmd.Flags().GetString("body")
		kind, _ := cmd.Flags().GetString("kind")
		label, _ := cmd.Flags().GetString("agent-label")
		refs, _ := cmd.Flags().GetStringArray("ref")
		request := api.BoardPostRequest{Kind: kind, Body: body, AgentLabel: label, Refs: refs}
		if cmd.Flags().Changed("ttl") {
			duration, _ := cmd.Flags().GetDuration("ttl")
			if duration < time.Second || duration > 720*time.Hour || duration%time.Second != 0 {
				return fmt.Errorf("--ttl must be whole seconds between 1s and 720h")
			}
			seconds := int64(duration / time.Second)
			request.TTLSeconds = &seconds
		}
		result, err := apiClient.PostBoard(project, request)
		if err != nil {
			return err
		}
		if !isTableOutput() {
			fmt.Print(getFormatter().FormatRaw(result))
			return nil
		}
		printBoardPost(result.Post)
		return nil
	}
	read := &cobra.Command{Use: "read", Short: "Read relevant board posts and record read evidence", Args: cobra.NoArgs}
	read.Flags().String("since", "", "Only posts created after this RFC3339 timestamp")
	read.Flags().String("kinds", "status,message", "Comma-separated status,message filter")
	read.Flags().Int("limit", 20, "Maximum posts, 1-100")
	read.RunE = func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}
		since, _ := cmd.Flags().GetString("since")
		if since != "" {
			if _, err = time.Parse(time.RFC3339, since); err != nil {
				return fmt.Errorf("--since must be an RFC3339 timestamp: %w", err)
			}
		}
		kinds, _ := cmd.Flags().GetString("kinds")
		limit, _ := cmd.Flags().GetInt("limit")
		result, err := apiClient.ReadBoard(project, since, kinds, limit)
		if err != nil {
			return err
		}
		if !isTableOutput() {
			fmt.Print(getFormatter().FormatRaw(result))
			return nil
		}
		if len(result.Posts) == 0 {
			fmt.Println("No relevant board posts.")
		}
		for _, post := range result.Posts {
			printBoardPost(post)
		}
		return nil
	}
	board.AddCommand(post, read)
	return board
}

func printBoardPost(post api.BoardPost) {
	// Quote all caller-controlled text: terminal escapes and newlines are data.
	fmt.Printf("#%d account:%d agent:%q (%q, %s): %q\n", post.ID, post.AuthorID, post.AgentLabel, post.Kind, post.CreatedAt.Format(time.RFC3339), post.Body)
}

func init() { rootCmd.AddCommand(newBoardCmd()) }
