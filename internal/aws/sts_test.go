package aws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type mockSTSAPI struct {
	calls int
	fn    func(callNum int) (*sts.GetCallerIdentityOutput, error)
}

func (m *mockSTSAPI) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	m.calls++
	return m.fn(m.calls)
}

func init() {
	// Override retry sleep so tests run instantly.
	stsRetrySleep = 0 * time.Millisecond
}

func TestValidateSessionWithClient_Success(t *testing.T) {
	mock := &mockSTSAPI{
		fn: func(callNum int) (*sts.GetCallerIdentityOutput, error) {
			return &sts.GetCallerIdentityOutput{}, nil
		},
	}

	err := validateSessionWithClient(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.calls != 1 {
		t.Errorf("calls = %d, want 1", mock.calls)
	}
}

func TestValidateSessionWithClient_PermanentError(t *testing.T) {
	mock := &mockSTSAPI{
		fn: func(callNum int) (*sts.GetCallerIdentityOutput, error) {
			return nil, errors.New("AccessDenied: not authorized")
		},
	}

	err := validateSessionWithClient(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "session expired or invalid") {
		t.Errorf("error = %q, want to contain 'session expired or invalid'", err)
	}
	if mock.calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry for permanent error)", mock.calls)
	}
}

func TestValidateSessionWithClient_TransientThenSuccess(t *testing.T) {
	mock := &mockSTSAPI{
		fn: func(callNum int) (*sts.GetCallerIdentityOutput, error) {
			if callNum == 1 {
				return nil, errors.New("Throttling: rate exceeded")
			}
			return &sts.GetCallerIdentityOutput{}, nil
		},
	}

	err := validateSessionWithClient(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.calls != 2 {
		t.Errorf("calls = %d, want 2 (one retry)", mock.calls)
	}
}

func TestValidateSessionWithClient_TransientThenFail(t *testing.T) {
	mock := &mockSTSAPI{
		fn: func(callNum int) (*sts.GetCallerIdentityOutput, error) {
			return nil, errors.New("i/o timeout")
		},
	}

	err := validateSessionWithClient(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error")
	}
	if mock.calls != 2 {
		t.Errorf("calls = %d, want 2 (one retry then give up)", mock.calls)
	}
}
