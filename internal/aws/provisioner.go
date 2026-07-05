package aws

import (
	"context"
	"fmt"
)

// Provisioner orchestrates EC2 instance lifecycle operations (provision, destroy, resize)
// using the EC2Client, SSMClient, and S3Client for shared infrastructure configuration.
type Provisioner struct {
	ec2Client *EC2Client
	ssmClient *SSMClient
	s3Client  *S3Client
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

// NewProvisioner creates a new Provisioner with the given EC2, SSM, and S3 clients.
func NewProvisioner(ec2Client *EC2Client, ssmClient *SSMClient, s3Client *S3Client) *Provisioner {
	return &Provisioner{
		ec2Client: ec2Client,
		ssmClient: ssmClient,
		s3Client:  s3Client,
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

	// Create S3 bucket for the developer.
	bucketName := BucketName(params.DeveloperName)
	if err := p.s3Client.CreateBucket(ctx, bucketName); err != nil {
		return nil, fmt.Errorf("failed to create S3 bucket: %w", err)
	}

	if err := p.ssmClient.PutDeveloperS3Bucket(ctx, params.DeveloperName, bucketName); err != nil {
		return nil, fmt.Errorf("failed to store S3 bucket param: %w", err)
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

// Destroy terminates an EC2 instance and cleans up the associated key pair and S3 bucket.
// All cleanup operations are best-effort to ensure maximum cleanup.
func (p *Provisioner) Destroy(ctx context.Context, instanceID, developerName string) error {
	if err := p.ec2Client.TerminateInstance(ctx, instanceID); err != nil {
		return fmt.Errorf("failed to terminate instance: %w", err)
	}

	keyName := "clouddesktop-" + developerName
	_ = p.ec2Client.DeleteSSHKeyPair(ctx, keyName)

	// Best-effort S3 bucket cleanup.
	bucketName, err := p.ssmClient.GetDeveloperS3Bucket(ctx, developerName)
	if err == nil && bucketName != "" {
		_ = p.s3Client.EmptyBucket(ctx, bucketName)
		_ = p.s3Client.DeleteBucket(ctx, bucketName)
		_ = p.ssmClient.DeleteDeveloperS3Bucket(ctx, developerName)
	}

	return nil
}

// SetupS3Mount creates the S3 bucket (if needed) and sends an SSM command to mount it
// on an already-running instance. Used for existing instances that predate this feature.
func (p *Provisioner) SetupS3Mount(ctx context.Context, instanceID, developerName string) error {
	bucketName, err := p.ssmClient.GetDeveloperS3Bucket(ctx, developerName)
	if err != nil {
		return err
	}

	if bucketName == "" {
		bucketName = BucketName(developerName)
		if err := p.s3Client.CreateBucket(ctx, bucketName); err != nil {
			return fmt.Errorf("failed to create S3 bucket: %w", err)
		}
		if err := p.ssmClient.PutDeveloperS3Bucket(ctx, developerName, bucketName); err != nil {
			return fmt.Errorf("failed to store S3 bucket param: %w", err)
		}
	}

	cmdID, err := p.ssmClient.RunS3Mount(ctx, instanceID, bucketName)
	if err != nil {
		return err
	}

	return p.ssmClient.WaitUntilCommandComplete(ctx, instanceID, cmdID)
}

// Resize changes the instance type of a stopped EC2 instance.
// The caller must ensure the instance is stopped before calling this method.
func (p *Provisioner) Resize(ctx context.Context, instanceID, newInstanceType string) error {
	return p.ec2Client.ModifyInstanceType(ctx, instanceID, newInstanceType)
}
