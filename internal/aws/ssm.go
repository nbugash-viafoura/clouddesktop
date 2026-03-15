package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// ssmapi is the subset of the AWS SSM SDK client used by SSMClient.
type ssmapi interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// SSMClient wraps AWS SSM API operations for reading shared infrastructure parameters.
type SSMClient struct {
	client ssmapi
}

// SharedInfraConfig holds the shared infrastructure parameters set by Tier 1 Terraform.
type SharedInfraConfig struct {
	SubnetID            string
	SecurityGroupID     string
	InstanceProfileName string
}

const (
	ssmParamSubnetID            = "/clouddesktop/shared/subnet_id"
	ssmParamSecurityGroupID     = "/clouddesktop/shared/security_group_id"
	ssmParamInstanceProfileName = "/clouddesktop/shared/instance_profile_name"
)

// NewSSMClient creates a new SSM client configured with the specified AWS profile and region.
func NewSSMClient(ctx context.Context, profile, region string) (*SSMClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for SSM: %w", err)
	}

	return &SSMClient{
		client: ssm.NewFromConfig(cfg),
	}, nil
}

// newSSMClientWithAPI creates an SSMClient with a custom API implementation (for testing).
func newSSMClientWithAPI(api ssmapi) *SSMClient {
	return &SSMClient{client: api}
}

// GetSharedInfraConfig reads the shared infrastructure parameters from SSM Parameter Store.
// These parameters are set by the Tier 1 shared Terraform configuration.
func (c *SSMClient) GetSharedInfraConfig(ctx context.Context) (*SharedInfraConfig, error) {
	subnetID, err := c.getParameter(ctx, ssmParamSubnetID)
	if err != nil {
		return nil, fmt.Errorf("failed to read subnet ID: %w", err)
	}

	sgID, err := c.getParameter(ctx, ssmParamSecurityGroupID)
	if err != nil {
		return nil, fmt.Errorf("failed to read security group ID: %w", err)
	}

	profileName, err := c.getParameter(ctx, ssmParamInstanceProfileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read instance profile name: %w", err)
	}

	return &SharedInfraConfig{
		SubnetID:            subnetID,
		SecurityGroupID:     sgID,
		InstanceProfileName: profileName,
	}, nil
}

// getParameter reads a single SSM parameter value.
func (c *SSMClient) getParameter(ctx context.Context, name string) (string, error) {
	output, err := c.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name: &name,
	})
	if err != nil {
		return "", fmt.Errorf("SSM parameter %s: %w", name, err)
	}
	if output.Parameter == nil || output.Parameter.Value == nil {
		return "", fmt.Errorf("SSM parameter %s has no value", name)
	}
	return *output.Parameter.Value, nil
}
