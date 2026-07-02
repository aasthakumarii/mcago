package awsiam

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type IAMManager struct {
	iamClient *iam.Client
	stsClient *sts.Client
}

func NewIAMManager(ctx context.Context) (*IAMManager, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return &IAMManager{
		iamClient: iam.NewFromConfig(cfg),
		stsClient: sts.NewFromConfig(cfg),
	}, nil
}

func (m *IAMManager) EnsureTeamRole(ctx context.Context, roleName string) error {
	// 1. Get current Account ID for the Trust Policy
	callerIdentity, err := m.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("failed to get AWS account ID: %w", err)
	}
	accountID := aws.ToString(callerIdentity.Account)

	// 2. Idempotency Check
	_, err = m.iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err == nil {
		return nil // Role exists, we're good
	}

	var nfe *types.NoSuchEntityException
	if !errors.As(err, &nfe) {
		return fmt.Errorf("failed to check if IAM role exists: %w", err)
	}

	// 3. Create Role with a basic assume role policy allowing local account
	trustPolicy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": { "AWS": "arn:aws:iam::%s:root" },
				"Action": "sts:AssumeRole"
			}
		]
	}`, accountID)

	_, err = m.iamClient.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(trustPolicy),
		Description:              aws.String("Managed by MCAGO Kubernetes Operator"),
	})

	if err != nil {
		var aee *types.EntityAlreadyExistsException
		if errors.As(err, &aee) {
			return nil // Handled concurrent creation safely
		}
		return fmt.Errorf("failed to create IAM role: %w", err)
	}

	return nil
}

func (m *IAMManager) DeleteTeamRole(ctx context.Context, roleName string) error {
	_, err := m.iamClient.DeleteRole(ctx, &iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		var nfe *types.NoSuchEntityException
		if errors.As(err, &nfe) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete IAM role %s: %w", roleName, err)
	}
	return nil
}
