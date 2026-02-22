package pkgspec

// ManifestType identifies the type of an Elastic package.
type ManifestType string

// Enum values for ManifestType.
const (
	ManifestTypeIntegration ManifestType = "integration"
	ManifestTypeInput       ManifestType = "input"
	ManifestTypeContent     ManifestType = "content"
)
