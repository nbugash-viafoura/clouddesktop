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
	DeveloperName    string
	EnvironmentName  string
	InstanceType     string
	RootVolumeSizeGB int32 // defaults to 100 when zero
	SSHPublicKey     string
	UserData         []byte
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

// Provision creates a new EC2 instance for the developer's environment, or
// recovers the existing instance tagged with that environment name. This
// prevents orphaned instances when config is accidentally deleted.
func (p *Provisioner) Provision(ctx context.Context, params ProvisionParams) (*ProvisionResult, error) {
	// Recovery check: look for an existing non-terminated instance for this environment.
	envs, err := p.ec2Client.ListEnvironments(ctx, params.DeveloperName)
	if err != nil {
		return nil, fmt.Errorf("instance recovery check failed: %w", err)
	}
	for _, env := range envs {
		if env.Name == params.EnvironmentName {
			fmt.Printf("Found existing instance %s for environment %s. Recovering.\n", env.InstanceID, env.Name)
			return &ProvisionResult{
				InstanceID: env.InstanceID,
				Recovered:  true,
			}, nil
		}
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

	// Import SSH key pair (idempotent -- deletes existing first). The key pair is
	// shared by all of the developer's environments.
	keyName := keyPairName(params.DeveloperName)
	if err := p.ec2Client.ImportSSHKeyPair(ctx, keyName, params.SSHPublicKey); err != nil {
		return nil, err
	}

	// The developer's bucket is shared by all environments; create it only if missing.
	if _, err := p.ensureBucket(ctx, params.DeveloperName); err != nil {
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
		EnvironmentName:     params.EnvironmentName,
		RootVolumeSizeGB:    params.RootVolumeSizeGB,
	})
	if err != nil {
		// The key pair is left in place: another environment's launch may be using it.
		return nil, err
	}

	return &ProvisionResult{
		InstanceID: instanceID,
		Recovered:  false,
	}, nil
}

// Destroy terminates one environment's instance; its root volume is deleted on
// termination. The developer's bucket is never touched: only s3-reset deletes it.
// The shared key pair is deleted (best-effort) only when deleteKeyPair is true,
// which callers set when no other environment remains.
func (p *Provisioner) Destroy(ctx context.Context, instanceID, developerName string, deleteKeyPair bool) error {
	if err := p.ec2Client.TerminateInstance(ctx, instanceID); err != nil {
		return fmt.Errorf("failed to terminate instance: %w", err)
	}

	if deleteKeyPair {
		_ = p.ec2Client.DeleteSSHKeyPair(ctx, keyPairName(developerName))
	}

	return nil
}

// ensureBucket returns the developer's bucket name, creating the bucket and its
// SSM parameter when they are missing or the parameter points at a deleted bucket.
func (p *Provisioner) ensureBucket(ctx context.Context, developerName string) (string, error) {
	bucketName, err := p.ssmClient.GetDeveloperS3Bucket(ctx, developerName)
	if err != nil {
		return "", err
	}

	if bucketName != "" {
		exists, err := p.s3Client.BucketExists(ctx, bucketName)
		if err != nil {
			return "", err
		}
		if exists {
			return bucketName, nil
		}
		fmt.Println("S3 bucket no longer exists. Recreating it...")
	}

	bucketName = BucketName(developerName)
	if err := p.s3Client.CreateBucket(ctx, bucketName); err != nil {
		return "", fmt.Errorf("failed to create S3 bucket: %w", err)
	}
	if err := p.ssmClient.PutDeveloperS3Bucket(ctx, developerName, bucketName); err != nil {
		return "", fmt.Errorf("failed to store S3 bucket param: %w", err)
	}
	return bucketName, nil
}

// keyPairName returns the EC2 key pair shared by all of a developer's environments.
func keyPairName(developerName string) string {
	return "clouddesktop-" + developerName
}

// EnsureS3Mount makes sure the developer's bucket exists and is mounted on the
// given instance. The mount command runs on every call: the script does nothing
// when the bucket is already mounted and healthy, and repairs a stale mount.
func (p *Provisioner) EnsureS3Mount(ctx context.Context, instanceID, developerName string) error {
	bucketName, err := p.ensureBucket(ctx, developerName)
	if err != nil {
		return err
	}

	commandID, err := p.ssmClient.RunS3Mount(ctx, instanceID, bucketName)
	if err != nil {
		return err
	}

	return p.ssmClient.WaitUntilCommandComplete(ctx, instanceID, commandID)
}

// Resize changes the instance type of a stopped EC2 instance.
// The caller must ensure the instance is stopped before calling this method.
func (p *Provisioner) Resize(ctx context.Context, instanceID, newInstanceType string) error {
	return p.ec2Client.ModifyInstanceType(ctx, instanceID, newInstanceType)
}
