package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/aasthakumarii/mcago/internal/models"
)

// TeamResources is the desired set of Kubernetes and Cloud objects for a Team.
type TeamResources struct {
	NamespaceName      string
	ServiceAccountName string
	RoleName           string
	RoleBindingName    string
	IAMRoleName        string
	Labels             map[string]string
}

type TeamService struct{}

func NewTeamService() *TeamService {
	return &TeamService{}
}

// Validate applies validation rules.
func (s *TeamService) Validate(team models.Team) error {
	ns := strings.TrimSpace(team.Namespace)

	if ns == "" {
		return fmt.Errorf("namespace name cannot be empty")
	}
	if len(ns) > 63 {
		return fmt.Errorf("namespace name %q exceeds 63 characters", ns)
	}
	if strings.TrimSpace(team.Owner) == "" {
		return fmt.Errorf("owner cannot be empty")
	}

	return nil
}

// PlanReconcile computes the desired resources for a Team.
func (s *TeamService) PlanReconcile(ctx context.Context, team models.Team) (TeamResources, error) {
	if err := s.Validate(team); err != nil {
		return TeamResources{}, err
	}

	labels := map[string]string{
		"platform.mcago.dev/team":  team.Name,
		"platform.mcago.dev/owner": sanitizeLabelValue(team.Owner),
	}
	for k, v := range team.Labels {
		labels[k] = v
	}

	return TeamResources{
		NamespaceName:      team.Namespace,
		ServiceAccountName: "default",
		RoleName:           fmt.Sprintf("%s-team-role", team.Namespace),
		RoleBindingName:    fmt.Sprintf("%s-team-rolebinding", team.Namespace),
		IAMRoleName:        fmt.Sprintf("mcago-team-%s", team.Namespace),
		Labels:             labels,
	}, nil
}

func sanitizeLabelValue(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, "@", "-at-")
	v = strings.ReplaceAll(v, " ", "-")
	if len(v) > 63 {
		v = v[:63]
	}
	return v
}
