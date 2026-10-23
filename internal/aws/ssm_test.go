package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type mockSSMAPI struct {
	params                 map[string]string
	err                    error
	sendCommandFn          func(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	getCommandInvocationFn func(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error)
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

func (m *mockSSMAPI) PutParameter(_ context.Context, input *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.params == nil {
		m.params = make(map[string]string)
	}
	m.params[*input.Name] = *input.Value
	return &ssm.PutParameterOutput{}, nil
}

func (m *mockSSMAPI) DeleteParameter(_ context.Context, input *ssm.DeleteParameterInput, _ ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.params != nil {
		delete(m.params, *input.Name)
	}
	return &ssm.DeleteParameterOutput{}, nil
}

func (m *mockSSMAPI) SendCommand(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	if m.sendCommandFn != nil {
		return m.sendCommandFn(ctx, params, optFns...)
	}
	cmdID := "cmd-0123456789abcdef0"
	return &ssm.SendCommandOutput{
		Command: &ssmtypes.Command{CommandId: &cmdID},
	}, nil
}

func (m *mockSSMAPI) GetCommandInvocation(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	if m.getCommandInvocationFn != nil {
		return m.getCommandInvocationFn(ctx, params, optFns...)
	}
	return &ssm.GetCommandInvocationOutput{
		Status: ssmtypes.CommandInvocationStatusSuccess,
	}, nil
}

// fastSSMClient creates an SSMClient with minimal polling intervals for fast tests.
func fastSSMClient(api ssmapi) *SSMClient {
	c := newSSMClientWithAPI(api)
	c.pollInitialInterval = 1 * time.Millisecond
	c.pollMaxInterval = 5 * time.Millisecond
	c.sendRetryInitialInterval = 1 * time.Millisecond
	c.sendRetryMaxInterval = 2 * time.Millisecond
	c.sendRetryLimit = 50 * time.Millisecond
	return c
}

func TestGetSharedInfraConfig_Success(t *testing.T) {
	mock := &mockSSMAPI{
		params: map[string]string{
			"/clouddesktop/shared/subnet_id":             "subnet-abc123",
			"/clouddesktop/shared/security_group_id":     "sg-def456",
			"/clouddesktop/shared/instance_profile_name": "clouddesktop-developer-instance",
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

func TestRunFilesystemExtension_Success(t *testing.T) {
	mock := &mockSSMAPI{}
	client := newSSMClientWithAPI(mock)

	commandID, err := client.RunFilesystemExtension(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commandID == "" {
		t.Error("expected non-empty command ID")
	}
}

func TestRunFilesystemExtension_SendFails(t *testing.T) {
	mock := &mockSSMAPI{
		sendCommandFn: func(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			return nil, errors.New("instance not in SSM inventory")
		},
	}
	client := newSSMClientWithAPI(mock)

	_, err := client.RunFilesystemExtension(context.Background(), "i-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to send filesystem extension command") {
		t.Errorf("error = %q, want to contain 'failed to send filesystem extension command'", err)
	}
}

func TestWaitUntilCommandComplete_ImmediateSuccess(t *testing.T) {
	mock := &mockSSMAPI{}
	client := fastSSMClient(mock)

	err := client.WaitUntilCommandComplete(context.Background(), "i-123", "cmd-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitUntilCommandComplete_EventualSuccess(t *testing.T) {
	callCount := 0
	mock := &mockSSMAPI{
		getCommandInvocationFn: func(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			callCount++
			status := ssmtypes.CommandInvocationStatusInProgress
			if callCount >= 3 {
				status = ssmtypes.CommandInvocationStatusSuccess
			}
			return &ssm.GetCommandInvocationOutput{Status: status}, nil
		},
	}
	client := fastSSMClient(mock)

	err := client.WaitUntilCommandComplete(context.Background(), "i-123", "cmd-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 poll calls, got %d", callCount)
	}
}

func TestWaitUntilCommandComplete_Failed(t *testing.T) {
	mock := &mockSSMAPI{
		getCommandInvocationFn: func(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{Status: ssmtypes.CommandInvocationStatusFailed}, nil
		},
	}
	client := fastSSMClient(mock)

	err := client.WaitUntilCommandComplete(context.Background(), "i-123", "cmd-abc")
	if err == nil {
		t.Fatal("expected error for failed command")
	}
	if !strings.Contains(err.Error(), string(ssmtypes.CommandInvocationStatusFailed)) {
		t.Errorf("error = %q, want to contain 'Failed'", err)
	}
}

func TestWaitUntilCommandComplete_TimedOut(t *testing.T) {
	mock := &mockSSMAPI{
		getCommandInvocationFn: func(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{Status: ssmtypes.CommandInvocationStatusTimedOut}, nil
		},
	}
	client := fastSSMClient(mock)

	err := client.WaitUntilCommandComplete(context.Background(), "i-123", "cmd-abc")
	if err == nil {
		t.Fatal("expected error for timed out command")
	}
	if !strings.Contains(err.Error(), string(ssmtypes.CommandInvocationStatusTimedOut)) {
		t.Errorf("error = %q, want to contain 'TimedOut'", err)
	}
}

func TestWaitUntilCommandComplete_ContextCancelled(t *testing.T) {
	mock := &mockSSMAPI{
		getCommandInvocationFn: func(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{Status: ssmtypes.CommandInvocationStatusInProgress}, nil
		},
	}
	client := fastSSMClient(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.WaitUntilCommandComplete(ctx, "i-123", "cmd-abc")
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestWaitUntilCommandComplete_InvocationNotYetExistsRetried(t *testing.T) {
	callCount := 0
	mock := &mockSSMAPI{
		getCommandInvocationFn: func(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			callCount++
			if callCount < 3 {
				return nil, fmt.Errorf("operation error SSM: GetCommandInvocation, InvocationDoesNotExist: ")
			}
			return &ssm.GetCommandInvocationOutput{Status: ssmtypes.CommandInvocationStatusSuccess}, nil
		},
	}
	client := fastSSMClient(mock)

	err := client.WaitUntilCommandComplete(context.Background(), "i-123", "cmd-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 poll calls (retry on InvocationDoesNotExist), got %d", callCount)
	}
}

func TestRunFilesystemExtension_ScriptSentToInstance(t *testing.T) {
	var capturedInstanceID string
	var capturedScript string
	mock := &mockSSMAPI{
		sendCommandFn: func(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			if len(params.InstanceIds) > 0 {
				capturedInstanceID = params.InstanceIds[0]
			}
			if cmds, ok := params.Parameters["commands"]; ok && len(cmds) > 0 {
				capturedScript = cmds[0]
			}
			cmdID := "cmd-test"
			return &ssm.SendCommandOutput{Command: &ssmtypes.Command{CommandId: aws.String(cmdID)}}, nil
		},
	}
	client := newSSMClientWithAPI(mock)

	_, err := client.RunFilesystemExtension(context.Background(), "i-target")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedInstanceID != "i-target" {
		t.Errorf("instanceID = %q, want i-target", capturedInstanceID)
	}
	if !strings.Contains(capturedScript, "growpart") {
		t.Errorf("script does not contain 'growpart': %s", capturedScript)
	}
	if !strings.Contains(capturedScript, "resize2fs") {
		t.Errorf("script does not contain 'resize2fs': %s", capturedScript)
	}
}

// --- RunS3Mount script ---

func captureScript(t *testing.T, send func(client *SSMClient) error) string {
	t.Helper()
	var script string
	mock := &mockSSMAPI{
		sendCommandFn: func(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			script = params.Parameters["commands"][0]
			return &ssm.SendCommandOutput{Command: &ssmtypes.Command{CommandId: aws.String("cmd-1")}}, nil
		},
	}
	if err := send(fastSSMClient(mock)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return script
}

func TestRunS3Mount_Script(t *testing.T) {
	script := captureScript(t, func(client *SSMClient) error {
		_, err := client.RunS3Mount(context.Background(), "i-123", "clouddesktop-john-dev")
		return err
	})

	guard := strings.Index(script, "/var/log/bootstrap-system-complete")
	healthy := strings.Index(script, "mountpoint -q")
	umount := strings.Index(script, "umount")

	if guard == -1 || !strings.Contains(script, `echo "bootstrap in progress" >&2`) {
		t.Errorf("script must report bootstrap in progress on stderr:\n%s", script)
	}
	if healthy == -1 || !strings.Contains(script, "timeout 10 ls") || !strings.Contains(script, "/etc/fstab") {
		t.Errorf("script must check for a healthy mount of the right bucket:\n%s", script)
	}
	if guard >= healthy || healthy >= umount {
		t.Errorf("expected bootstrap guard, then healthy-mount check, then umount; got positions %d, %d, %d", guard, healthy, umount)
	}
	if !strings.Contains(script, "clouddesktop-john-dev") {
		t.Errorf("script does not reference the bucket:\n%s", script)
	}
	if !strings.Contains(script, ">> /etc/fstab") {
		t.Errorf("script must still add the fstab entry:\n%s", script)
	}
}

func TestWaitUntilCommandComplete_IncludesStandardError(t *testing.T) {
	mock := &mockSSMAPI{
		getCommandInvocationFn: func(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{
				Status:               ssmtypes.CommandInvocationStatusFailed,
				StandardErrorContent: aws.String("bootstrap in progress\n"),
			}, nil
		},
	}
	client := fastSSMClient(mock)

	err := client.WaitUntilCommandComplete(context.Background(), "i-123", "cmd-abc")
	if err == nil || !strings.HasSuffix(err.Error(), ": bootstrap in progress") {
		t.Errorf("error = %v, want it to end with the trimmed stderr", err)
	}
}

// --- SendCommand retry while SSM can't reach the instance ---

func TestSendCommand_RetriesUntilInstanceReachable(t *testing.T) {
	senders := map[string]func(*SSMClient) (string, error){
		"RunS3Mount": func(c *SSMClient) (string, error) {
			return c.RunS3Mount(context.Background(), "i-123", "clouddesktop-john-dev")
		},
		"RunFilesystemExtension": func(c *SSMClient) (string, error) {
			return c.RunFilesystemExtension(context.Background(), "i-123")
		},
	}

	for name, send := range senders {
		t.Run(name, func(t *testing.T) {
			calls := 0
			mock := &mockSSMAPI{
				sendCommandFn: func(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
					calls++
					if calls < 3 {
						return nil, errors.New("api error InvalidInstanceId: Instances [[i-123]] not in a valid state")
					}
					return &ssm.SendCommandOutput{Command: &ssmtypes.Command{CommandId: aws.String("cmd-ok")}}, nil
				},
			}

			commandID, err := send(fastSSMClient(mock))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if commandID != "cmd-ok" || calls != 3 {
				t.Errorf("commandID=%q calls=%d, want cmd-ok after 3 calls", commandID, calls)
			}
		})
	}
}

func TestSendCommand_GivesUpAfterLimit(t *testing.T) {
	mock := &mockSSMAPI{
		sendCommandFn: func(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			return nil, errors.New("api error InvalidInstanceId: not in a valid state")
		},
	}

	_, err := fastSSMClient(mock).RunS3Mount(context.Background(), "i-123", "clouddesktop-john-dev")
	if err == nil || !strings.Contains(err.Error(), "SSM can't reach instance i-123 yet") {
		t.Errorf("error = %v, want it to say SSM can't reach the instance", err)
	}
}

func TestSendCommand_DoesNotRetryOtherErrors(t *testing.T) {
	calls := 0
	mock := &mockSSMAPI{
		sendCommandFn: func(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			calls++
			return nil, errors.New("AccessDeniedException")
		},
	}

	if _, err := fastSSMClient(mock).RunS3Mount(context.Background(), "i-123", "b"); err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("SendCommand calls = %d, want 1", calls)
	}
}
