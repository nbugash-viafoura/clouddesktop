package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/spf13/cobra"
)

// NewListCmd returns the list command which shows every environment the developer owns.
func NewListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your cloud desktops",
		Long:    "Lists every environment you own with its state, instance type and disk size, marks your default, and shows your personal S3 bucket. It never starts or changes an environment.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList()
		},
	}
}

func runList() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := openSession(ctx)
	if err != nil {
		return err
	}

	if len(s.envs) == 0 {
		fmt.Println("You have no environments. Create one with 'clouddesktop create <name>'.")
	} else {
		volumeIDs := make([]string, 0, len(s.envs))
		for _, env := range s.envs {
			if env.RootVolumeID != "" {
				volumeIDs = append(volumeIDs, env.RootVolumeID)
			}
		}
		sizes, err := s.ec2.RootVolumeSizes(ctx, volumeIDs)
		if err != nil {
			return err
		}
		for i := range s.envs {
			s.envs[i].RootVolumeSizeGB = sizes[s.envs[i].RootVolumeID]
		}
		fmt.Print(renderEnvironmentTable(s.envs, s.cfg.DefaultEnvironment))
	}

	fmt.Println()
	fmt.Println(bucketLine(ctx, s))
	return nil
}

// renderEnvironmentTable returns one row per environment, sorted by name, with
// the default marked "*". A default that no longer exists is called out below the table.
func renderEnvironmentTable(envs []environment.Environment, defaultName string) string {
	sorted := append([]environment.Environment(nil), envs...)
	environment.SortByName(sorted)

	rows := [][]string{{"NAME", "STATE", "TYPE", "DISK", "DEFAULT"}}
	defaultFound := false
	for _, env := range sorted {
		marker := ""
		if env.Name == defaultName {
			marker = "*"
			defaultFound = true
		}
		rows = append(rows, []string{env.Name, env.State, env.InstanceType, diskLabel(env.RootVolumeSizeGB), marker})
	}

	var b strings.Builder
	b.WriteString(alignColumns(rows))
	if defaultName != "" && !defaultFound {
		fmt.Fprintf(&b, "\nDefault environment %q no longer exists. Set one with 'clouddesktop use <name>'.\n", defaultName)
	}
	return b.String()
}

// alignColumns pads every column but the last to its widest cell plus two
// spaces, and trims trailing spaces from each line.
func alignColumns(rows [][]string) string {
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], len(cell))
		}
	}

	var b strings.Builder
	for _, row := range rows {
		var line strings.Builder
		for i, cell := range row {
			line.WriteString(cell)
			if i < len(row)-1 {
				line.WriteString(strings.Repeat(" ", widths[i]-len(cell)+2))
			}
		}
		b.WriteString(strings.TrimRight(line.String(), " ") + "\n")
	}
	return b.String()
}

// diskLabel formats a disk size, or "-" when it is unknown.
func diskLabel(sizeGB int32) string {
	if sizeGB == 0 {
		return "-"
	}
	return fmt.Sprintf("%d GB", sizeGB)
}

// bucketLine describes the developer's personal bucket, which outlives environments.
func bucketLine(ctx context.Context, s *session) string {
	name, err := s.ssm.GetDeveloperS3Bucket(ctx, s.cfg.DeveloperName)
	if err != nil {
		return fmt.Sprintf("Personal bucket: could not be checked (%s)", err)
	}
	if name == "" {
		return "Personal bucket: none yet (created with your next environment)"
	}

	exists, err := s.s3.BucketExists(ctx, name)
	switch {
	case err != nil:
		return fmt.Sprintf("Personal bucket: %s (could not be checked: %s)", name, err)
	case !exists:
		return fmt.Sprintf("Personal bucket: %s (deleted; recreated on your next 'clouddesktop up')", name)
	case len(s.envs) == 0:
		return fmt.Sprintf("Personal bucket: %s (exists; delete with 'clouddesktop s3-reset --confirm')", name)
	default:
		return fmt.Sprintf("Personal bucket: %s (available to all environments; kept when environments are destroyed)", name)
	}
}
