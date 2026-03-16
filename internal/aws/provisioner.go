package aws

import (
	"context"
	"fmt"
)

// Provisioner orchestrates EC2 instance lifecycle operations (provision, destroy, resize)
// using the EC2Client and SSMClient for shared infrastructure configuration.
type Provisioner struct {
	ec2Client *EC2Client
	ssmClient *SSMClient
}

// ProvisionParams holds the parameters needed to provision a new cloud desktop.
type ProvisionParams struct {
	DeveloperName string
	InstanceType  string
	SSHPublicKey  string
	UserData      []byte
}

// ProvisionResult holds the result of a provision operation.
type ProvisionResult struct {
	InstanceID string
	Recovered  bool // true if an existing orphaned instance was recovered
}

// NewProvisioner creates a new Provisioner with the given EC2 and SSM clients.
func NewProvisioner(ec2Client *EC2Client, ssmClient *SSMClient) *Provisioner {
	return &Provisioner{
		ec2Client: ec2Client,
		ssmClient: ssmClient,
	}
}

// Provision creates a new EC2 instance for the developer, or recovers an existing
// one if found via tag-based lookup. This prevents orphaned instances when config
// is accidentally deleted.
func (p *Provisioner) Provision(ctx context.Context, params ProvisionParams) (*ProvisionResult, error) {
	// Recovery check: look for an existing non-terminated instance.
	existing, err := p.ec2Client.FindInstanceByDeveloper(ctx, params.DeveloperName)
	if err != nil {
		return nil, fmt.Errorf("instance recovery check failed: %w", err)
	}
	if existing != nil {
		fmt.Printf("Found existing instance %s for developer %s. Recovering.\n", existing.InstanceID, params.DeveloperName)
		return &ProvisionResult{
			InstanceID: existing.InstanceID,
			Recovered:  true,
		}, nil
	}

	// Fetch shared infrastructure config from SSM.
	infraCfg, err := p.ssmClient.GetSharedInfraConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read shared infrastructure config: %w", err)
	}

	// Find latest Ubuntu AMI.
	amiID, err := p.ec2Client.FindUbuntuAMI(ctx)
	if err != nil {
		return nil, err
	}

	// Import SSH key pair (idempotent -- deletes existing first).
	keyName := "clouddesktop-" + params.DeveloperName
	if err := p.ec2Client.ImportSSHKeyPair(ctx, keyName, params.SSHPublicKey); err != nil {
		return nil, err
	}

	// Launch instance.
	instanceID, err := p.ec2Client.RunInstance(ctx, RunInstanceParams{
		AMIID:               amiID,
		InstanceType:        params.InstanceType,
		SubnetID:            infraCfg.SubnetID,
		SecurityGroupID:     infraCfg.SecurityGroupID,
		KeyName:             keyName,
		InstanceProfileName: infraCfg.InstanceProfileName,
		UserData:            params.UserData,
		DeveloperName:       params.DeveloperName,
	})
	if err != nil {
		// Cleanup key pair on failure.
		_ = p.ec2Client.DeleteSSHKeyPair(ctx, keyName)
		return nil, err
	}

	return &ProvisionResult{
		InstanceID: instanceID,
		Recovered:  false,
	}, nil
}

// Destroy terminates an EC2 instance and cleans up the associated key pair.
// Both operations are best-effort to ensure maximum cleanup.
func (p *Provisioner) Destroy(ctx context.Context, instanceID, developerName string) error {
	if err := p.ec2Client.TerminateInstance(ctx, instanceID); err != nil {
		return fmt.Errorf("failed to terminate instance: %w", err)
	}

	keyName := "clouddesktop-" + developerName
	// Best-effort key pair cleanup -- don't fail if already gone.
	_ = p.ec2Client.DeleteSSHKeyPair(ctx, keyName)

	return nil
}

// Resize changes the instance type of a stopped EC2 instance.
// The caller must ensure the instance is stopped before calling this method.
func (p *Provisioner) Resize(ctx context.Context, instanceID, newInstanceType string) error {
	return p.ec2Client.ModifyInstanceType(ctx, instanceID, newInstanceType)
}
