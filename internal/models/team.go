package models

// Team is the internal domain representation of a team, independent of the
// Kubernetes CRD shape. The controller maps v1alpha1.Team <-> models.Team.
type Team struct {
	Name        string
	Namespace   string
	Description string
	Owner       string
	Labels      map[string]string

	AWSEnabled    bool
	AWSPolicyARNs []string
}
