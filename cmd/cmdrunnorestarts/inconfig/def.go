package inconfig

// IntegrationType is the enumeration type for backend system_type
type IntegrationType int32

const (
	// IntegrationTypeWork is for work
	IntegrationTypeWork IntegrationType = 0
	// IntegrationTypeSourcecode is for sourcecode
	IntegrationTypeSourcecode IntegrationType = 1
	// IntegrationTypeCodequality is for codequality
	IntegrationTypeCodequality IntegrationType = 2
)

// String returns the string value
func (v IntegrationType) String() string {
	switch int32(v) {
	case 0:
		return "WORK"
	case 1:
		return "SOURCECODE"
	case 2:
		return "CODEQUALITY"
	}
	return "unset"
}
