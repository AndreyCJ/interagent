package port

type Permission string

const (
	PermissionMicrophone    Permission = "microphone"
	PermissionScreenCapture Permission = "screen-recording"
	PermissionAccessibility Permission = "accessibility"
)

type Permissions interface {
	Status(p Permission) (bool, error)
	Request(p Permission) error
	OpenSettings(p Permission) error
}
