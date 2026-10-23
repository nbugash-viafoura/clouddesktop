package cli

import (
	"context"
	"fmt"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
)

// session holds what every environment command needs: the developer's config,
// the AWS clients, and the developer's environments as AWS currently reports them.
type session struct {
	cfg  *config.Config
	ec2  *vfaws.EC2Client
	ssm  *vfaws.SSMClient
	s3   *vfaws.S3Client
	envs []environment.Environment
}

// openSession loads the config, validates the AWS session, and lists the
// developer's environments. An instance created before multiple environments
// existed is tagged as "main" on a best-effort basis.
func openSession(ctx context.Context) (*session, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	if err := vfaws.ValidateSession(ctx, cfg.AWSProfile, cfg.Region); err != nil {
		return nil, err
	}

	ec2Client, err := vfaws.NewEC2Client(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return nil, err
	}
	ssmClient, err := vfaws.NewSSMClient(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return nil, err
	}
	s3Client, err := vfaws.NewS3Client(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return nil, err
	}

	envs, err := ec2Client.ListEnvironments(ctx, cfg.DeveloperName)
	if err != nil {
		return nil, err
	}

	s := &session{cfg: cfg, ec2: ec2Client, ssm: ssmClient, s3: s3Client, envs: envs}
	s.tagLegacyEnvironments(ctx)
	return s, nil
}

// tagLegacyEnvironments records the environment name on untagged instances so
// cost reports show it. Failures are ignored: listing treats an untagged
// instance as "main" either way.
func (s *session) tagLegacyEnvironments(ctx context.Context) {
	for i, env := range s.envs {
		if !env.Legacy {
			continue
		}
		if err := s.ec2.TagEnvironment(ctx, env.InstanceID, env.RootVolumeID, env.Name); err == nil {
			s.envs[i].Legacy = false
		}
	}
}

// resolve picks the environment the command acts on (see environment.Resolve).
func (s *session) resolve(name string, op environment.Operation) (environment.Environment, error) {
	return environment.Resolve(environment.ResolveRequest{
		Name:         name,
		Environments: s.envs,
		Default:      s.cfg.DefaultEnvironment,
		Operation:    op,
	})
}

// provisioner returns a Provisioner built from the session's clients.
func (s *session) provisioner() *vfaws.Provisioner {
	return vfaws.NewProvisioner(s.ec2, s.ssm, s.s3)
}

// syncSSHConfig rewrites the managed block in ~/.ssh/config from a fresh
// listing. The listing is taken while holding the clouddesktop lock, so
// commands running at the same time converge on the latest set of environments.
func (s *session) syncSSHConfig(ctx context.Context) error {
	return config.WithLock(func() error {
		envs, err := s.ec2.ListEnvironments(ctx, s.cfg.DeveloperName)
		if err != nil {
			return err
		}
		s.envs = envs
		if err := vfaws.SyncSSHConfig(envs, sshSettings(s.cfg)); err != nil {
			return fmt.Errorf("failed to update SSH config: %w", err)
		}
		return nil
	})
}

// sshSettings returns the SSH connection settings shared by every environment.
func sshSettings(cfg *config.Config) vfaws.SSHSettings {
	return vfaws.SSHSettings{
		User:         "ubuntu",
		IdentityFile: cfg.SSHKeyPath,
		AWSProfile:   cfg.AWSProfile,
		Region:       cfg.Region,
	}
}

// printTarget names the environment a command is about to act on, before it acts.
func printTarget(env environment.Environment) {
	fmt.Printf("Environment: %s (%s)\n", env.Name, env.InstanceID)
}

// nameArg returns the optional environment name argument, or "" when absent.
func nameArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}
