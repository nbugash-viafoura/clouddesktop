package aws

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type mockSSMAPI struct {
	params map[string]string
	err    error
}

func (m *mockSSMAPI) GetParameter(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	name := *input.Name
	val, ok := m.params[name]
	if !ok {
		return nil, fmt.Errorf("ParameterNotFound: %s", name)
	}
	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{
			Name:  &name,
			Value: &val,
		},
	}, nil
}

func TestGetSharedInfraConfig_Success(t *testing.T) {
	mock := &mockSSMAPI{
		params: map[string]string{
			"/clouddesktop/shared/subnet_id":              "subnet-abc123",
			"/clouddesktop/shared/security_group_id":      "sg-def456",
			"/clouddesktop/shared/instance_profile_name":  "clouddesktop-developer-instance",
		},
	}
	client := newSSMClientWithAPI(mock)

	cfg, err := client.GetSharedInfraConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SubnetID != "subnet-abc123" {
		t.Errorf("SubnetID = %q, want subnet-abc123", cfg.SubnetID)
	}
	if cfg.SecurityGroupID != "sg-def456" {
		t.Errorf("SecurityGroupID = %q, want sg-def456", cfg.SecurityGroupID)
	}
	if cfg.InstanceProfileName != "clouddesktop-developer-instance" {
		t.Errorf("InstanceProfileName = %q, want clouddesktop-developer-instance", cfg.InstanceProfileName)
	}
}

func TestGetSharedInfraConfig_MissingParameter(t *testing.T) {
	mock := &mockSSMAPI{
		params: map[string]string{
			"/clouddesktop/shared/subnet_id": "subnet-abc123",
			// missing security_group_id and instance_profile_name
		},
	}
	client := newSSMClientWithAPI(mock)

	_, err := client.GetSharedInfraConfig(context.Background())
	if err == nil {
		t.Fatal("expected error for missing parameter, got nil")
	}
}

func TestGetSharedInfraConfig_APIError(t *testing.T) {
	mock := &mockSSMAPI{
		err: fmt.Errorf("AccessDeniedException: not authorized"),
	}
	client := newSSMClientWithAPI(mock)

	_, err := client.GetSharedInfraConfig(context.Background())
	if err == nil {
		t.Fatal("expected error for API failure, got nil")
	}
}
