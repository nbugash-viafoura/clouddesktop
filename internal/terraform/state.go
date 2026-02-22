package terraform

const (
	// SharedBackendKey is the S3 key for the shared infrastructure Terraform state.
	SharedBackendKey = "shared/terraform.tfstate"
)

// S3BackendKey returns the S3 key for a developer's Terraform state file.
func S3BackendKey(developerName string) string {
	return "developers/" + developerName + "/terraform.tfstate"
}
