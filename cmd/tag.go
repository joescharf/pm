package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/output"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manage issue tags",
	Long:  "Create, list, and delete tags for organizing issues.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tagListRun()
	},
}

var tagListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all tags",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tagListRun()
	},
}

var tagCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tagCreateRun(args[0])
	},
}

var tagDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm"},
	Short:   "Delete a tag",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tagDeleteRun(args[0])
	},
}

func init() {
	tagCmd.AddCommand(tagListCmd)
	tagCmd.AddCommand(tagCreateCmd)
	tagCmd.AddCommand(tagDeleteCmd)
	rootCmd.AddCommand(tagCmd)
}

func tagListRun() error {
	s, err := getStore()
	if err != nil {
		return err
	}

	tags, err := s.ListTags(context.Background())
	if err != nil {
		return err
	}

	if len(tags) == 0 {
		ui.Info("No tags. Use 'pm tag create <name>' to create one.")
		return nil
	}

	table := ui.Table([]string{"Name", "Created"})
	for _, t := range tags {
		_ = table.Append([]string{
			output.Cyan(t.Name),
			t.CreatedAt.Format("2006-01-02"),
		})
	}
	_ = table.Render()
	return nil
}

func tagCreateRun(name string) error {
	s, err := getStore()
	if err != nil {
		return err
	}

	if dryRun {
		ui.DryRunMsg("Would create tag: %s", name)
		return nil
	}

	tag := &models.Tag{Name: name}
	if err := s.CreateTag(context.Background(), tag); err != nil {
		return fmt.Errorf("create tag: %w", err)
	}

	ui.Success("Created tag: %s", output.Cyan(name))
	return nil
}

func tagDeleteRun(name string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	// Find tag by name
	tags, err := s.ListTags(ctx)
	if err != nil {
		return err
	}

	var tagID string
	for _, t := range tags {
		if t.Name == name {
			tagID = t.ID
			break
		}
	}

	if tagID == "" {
		return fmt.Errorf("tag not found: %s", name)
	}

	if dryRun {
		ui.DryRunMsg("Would delete tag: %s", name)
		return nil
	}

	if err := s.DeleteTag(ctx, tagID); err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}

	ui.Success("Deleted tag: %s", name)
	return nil
}
