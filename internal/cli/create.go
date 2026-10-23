package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/nbugash-viafoura/clouddesktop/scripts"
	"github.com/spf13/cobra"
)

const defaultStorageSizeGB = 100

// NewCreateCmd returns the create command which provisions an additional environment.
func NewCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an additional cloud desktop",
		Long: "Creates a new environment alongside your existing ones; they are not changed.\n\n" +
			"Name it after its function, in one word: for example 'build', 'service' or 'ci'. " +
			"Names use 1-20 lowercase letters, digits or hyphens, starting and ending with a letter or digit.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			instanceType, _ := cmd.Flags().GetString("instance-type")
			storageGB, _ := cmd.Flags().GetInt("storage-size")
			return runCreate(args[0], instanceType, storageGB)
		},
	}

	cmd.Flags().String("instance-type", "", "EC2 instance type (default: instance_type from your config)")
	cmd.Flags().Int("storage-size", defaultStorageSizeGB, "Root volume size in GB")

	return cmd
}

func runCreate(name, instanceType string, storageGB int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	s, err := openSession(ctx)
	if err != nil {
		return err
	}

	if err := environment.ValidateNew(name, s.envs); err != nil {
		return err
	}
	if instanceType == "" {
		instanceType = s.cfg.InstanceType
	}
	if !config.IsValidInstanceType(instanceType) {
		return fmt.Errorf("instance type %q is not supported\nChoose one of: %s", instanceType, strings.Join(instanceTypeValues(), ", "))
	}
	if !config.IsValidStorageSize(storageGB) {
		return fmt.Errorf("storage size %d GB is not supported\nChoose one of: %s", storageGB, strings.Join(storageSizeValues(), ", "))
	}

	alias := environment.SSHAlias(name)
	taken, err := vfaws.HostDefinedOutsideBlock(alias)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("~/.ssh/config already defines host %q outside the clouddesktop block\nChoose another name or remove that entry", alias)
	}

	return provisionEnvironment(ctx, s, name, instanceType, int32(storageGB))
}

// provisionEnvironment launches a new environment, waits until it is running,
// and adds its SSH alias. The developer's first environment becomes the
// default. If the instance never reaches running, it is terminated so no
// half-created environment is left behind.
func provisionEnvironment(ctx context.Context, s *session, name, instanceType string, storageGB int32) error {
	firstEnvironment := len(s.envs) == 0
	fmt.Printf("Environment: %s (creating)\n", name)
	fmt.Printf("Provisioning instance (type: %s, storage: %d GB)...\n", instanceType, storageGB)

	result, err := s.provisioner().Provision(ctx, vfaws.ProvisionParams{
		DeveloperName:    s.cfg.DeveloperName,
		EnvironmentName:  name,
		InstanceType:     instanceType,
		RootVolumeSizeGB: storageGB,
		SSHPublicKey:     s.cfg.SSHPublicKey,
		UserData:         scripts.BootstrapSystem,
	})
	if err != nil {
		return fmt.Errorf("provisioning failed: %w", err)
	}
	printTarget(environment.Environment{Name: name, InstanceID: result.InstanceID})

	if err := s.ec2.WaitUntilRunning(ctx, result.InstanceID); err != nil {
		if !result.Recovered {
			fmt.Printf("Instance %s did not start. Terminating it so nothing is left behind...\n", result.InstanceID)
			_ = s.ec2.TerminateInstance(ctx, result.InstanceID)
		}
		return fmt.Errorf("timed out waiting for instance to start: %w", err)
	}

	if err := s.syncSSHConfig(ctx); err != nil {
		return err
	}

	if firstEnvironment {
		if err := config.Update(func(cfg *config.Config) error {
			if cfg.DefaultEnvironment == "" {
				cfg.DefaultEnvironment = name
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed to set the default environment: %w", err)
		}
	}

	if result.Recovered {
		fmt.Printf("Recovered existing instance %s.\n", result.InstanceID)
		ensureS3Mount(ctx, s, name, result.InstanceID)
	} else {
		fmt.Println()
		fmt.Println("Bootstrap is installing your tooling (~10 minutes on first provision).")
		fmt.Printf("You can check progress: clouddesktop ssh %s, then 'tail -f /var/log/bootstrap-system.log'\n", name)
		fmt.Printf("Run 'clouddesktop up %s' after bootstrap (~10 minutes) to mount your personal bucket at /home/ubuntu/s3\n", name)
	}

	info, err := s.ec2.DescribeInstance(ctx, result.InstanceID)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Cloud desktop is running.")
	printInstanceSummary(name, info, s.cfg)
	return nil
}

// instanceTypeValues lists the supported instance types.
func instanceTypeValues() []string {
	values := make([]string, 0, len(config.ValidInstanceTypes))
	for _, it := range config.ValidInstanceTypes {
		values = append(values, it.Value)
	}
	return values
}

// storageSizeValues lists the supported root volume sizes in GB.
func storageSizeValues() []string {
	values := make([]string, 0, len(config.ValidStorageSizes))
	for _, size := range config.ValidStorageSizes {
		values = append(values, strconv.Itoa(size))
	}
	return values
}
