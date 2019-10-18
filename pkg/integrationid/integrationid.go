package integrationid

import "fmt"

// ID defines a unique integration.
// Since some integration binaries contain different
// integrations based on type, we neeed to include type as well.
type ID struct {
	// Name is the name of the integration binary
	Name string

	// Type is the value of the type option passed to binary.
	// Can be empty if binary contains only one integration.
	Type Type
}

func (s ID) String() string {
	if s.Type == "" {
		return s.Name
	}
	return s.Name + "@" + s.Type.String()
}

type Type string

func (s Type) String() string {
	return string(s)
}

const (
	TypeEmpty      Type = ""
	TypeSourcecode      = "sourcecode"
	TypeWork            = "work"
)

func TypeFromString(v string) (Type, error) {
	switch v {
	case "":
		return "", nil
	case "sourcecode":
		return TypeSourcecode, nil
	case "work":
		return TypeWork, nil
	}
	return "", fmt.Errorf("invalid integration id type: %v", v)
}
