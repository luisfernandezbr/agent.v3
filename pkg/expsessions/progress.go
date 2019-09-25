package expsessions

import "strings"

// ProgressPath describes the path that is in progress.
//
// Progress path example:
// organization/pinpoint/repo/test_repo/sourcecode.PullRequest/1
type ProgressPath []ProgressPathComponent

func (s ProgressPath) StringWithObjectNames() string {
	res := []string{}
	for _, c := range s {
		if c.ModelName != "" {
			res = append(res, c.ModelName)
		} else if c.TrackingName != "" {
			res = append(res, c.TrackingName)
		} else {
			if c.ObjectName == c.ObjectID {
				res = append(res, c.ObjectID)
			} else {
				res = append(res, c.ObjectName+":"+c.ObjectID)
			}
		}
	}
	return strings.Join(res, "/")
}

func (s ProgressPath) Strings() (res []string) {
	for _, c := range s {
		if c.ModelName != "" {
			res = append(res, c.ModelName)
		} else if c.TrackingName != "" {
			res = append(res, c.TrackingName)
		} else {
			res = append(res, c.ObjectID)
		}
	}
	return
}

func (s ProgressPath) String() string {
	return strings.Join(s.Strings(), "/")
}

func (s ProgressPath) Copy() ProgressPath {
	res := make([]ProgressPathComponent, len(s))
	copy(res, s)
	return res
}

// ProgressComponent describes one component of progress path.
type ProgressPathComponent struct {
	// ModelName is set for model sessions
	ModelName string
	// TrackingName is set for tracking sessions
	TrackingName string
	// ObjectName is to describe a specific object in model or tracking session
	ObjectName string
	// ObjectID is to describe a specific object in model or tracking session
	ObjectID string
}
