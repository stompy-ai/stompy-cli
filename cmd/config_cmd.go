package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stompy-ai/stompy-cli/internal/config"
	"github.com/stompy-ai/stompy-cli/internal/output"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.SetValue(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("%s = %s\n", args[0], args[1])
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		val := config.GetValue(args[0])
		if val == "" {
			fmt.Printf("%s is not set\n", args[0])
		} else {
			fmt.Println(val)
		}
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show all configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		settings := config.GetAllSettings()
		f := getFormatter()

		var fields []output.KeyValue
		flattenSettings("", settings, &fields)

		fmt.Print(f.FormatSingle(fields))
		fmt.Printf("\nConfig file: %s\n", config.GetConfigPath())
		return nil
	},
}

// flattenSettings recursively flattens nested maps into dot-separated key-value pairs.
func flattenSettings(prefix string, m map[string]any, fields *[]output.KeyValue) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch vt := v.(type) {
		case map[string]any:
			flattenSettings(key, vt, fields)
		default:
			val := fmt.Sprintf("%v", v)
			if isSensitive(key) && len(val) > 12 {
				val = val[:8] + "..." + val[len(val)-4:]
			}
			*fields = append(*fields, output.KeyValue{Key: key, Value: val})
		}
	}
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}

func isSensitive(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "key") ||
		strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret")
}
