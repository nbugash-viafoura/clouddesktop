package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// stsapi is the subset of the AWS STS SDK client used for session validation.
type stsapi interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// stsRetrySleep is the delay between retries for transient STS errors.
// Tests override this to zero for fast execution.
var stsRetrySleep = 2 * time.Second

// ValidateSession validates the current AWS session by calling sts:GetCallerIdentity.
// A single retry is attempted for transient network errors. Permission errors fail immediately.
func ValidateSession(ctx context.Context, profile, region string) error {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := sts.NewFromConfig(cfg)
	return validateSessionWithClient(ctx, client)
}

// validateSessionWithClient contains the retry logic for session validation.
func validateSessionWithClient(ctx context.Context, client stsapi) error {
	_, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		if isTransientError(err) {
			time.Sleep(stsRetrySleep)
			_, err = client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
			if err == nil {
				return nil
			}
		}
		return errors.New("AWS session expired or invalid - run 'sts' to refresh your credentials")
	}

	return nil
}

// GetAccountID returns the AWS account ID for the current session.
func GetAccountID(ctx context.Context, profile, region string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := sts.NewFromConfig(cfg)
	output, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get account ID: %w", err)
	}

	if output.Account == nil {
		return "", errors.New("STS returned no account ID")
	}

	return *output.Account, nil
}
