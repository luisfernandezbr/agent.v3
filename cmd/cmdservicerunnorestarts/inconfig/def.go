package inconfig

// IntegrationType is the enumeration type for system_type
type IntegrationType int32

const (
	// IntegrationTypeWork is the enumeration value for work
	IntegrationTypeWork IntegrationType = 0
	// IntegrationTypeSourcecode is the enumeration value for sourcecode
	IntegrationTypeSourcecode IntegrationType = 1
	// IntegrationTypeCodequality is the enumeration value for codequality
	IntegrationTypeCodequality IntegrationType = 2
)

// String returns the string value for IntegrationSystemType
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
