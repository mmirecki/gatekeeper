package mutation

import (
	"strings"

	mutationsv1alpha1 "github.com/open-policy-agent/gatekeeper/apis/mutations/v1alpha1"
)

// NOTE: this is just temporary placeholder code.
// The proper implementation will be provided by the GK community

const (
	ObjectType Type = "OBJECT"
	ListType   Type = "LIST"
)

type Type string

type PathEntry interface {
	Type() Type
}

// QUESTION: should we use nil-pointers instead of empty strings
// to represent unset paths?
type Object struct {
	TypeName Type
	PointsTo *string // the field name this entry points to for
	// the next entry in the list, should be
	// an empty string for the last item in the
	// PathEntry array
}

func (o Object) Type() Type {
	return ObjectType
}

type List struct {
	TypeName Type
	KeyField string  // The key field for the member objects
	KeyValue *string // The value of the keyfield, must be populated for the last item in the PathEntry array
	Globbed  bool    // Globbed. This cannot be true if keyValue is set
}

func (o List) Type() Type {
	return ListType
}

func parseLocation(m *mutationsv1alpha1.AssignMetadata) []PathEntry {
	location := m.Spec.Location
	locationParts := strings.Split(location, ".")
	entries := make([]PathEntry, 0)

	for _, part := range locationParts {
		if strings.Contains(part, "[") && strings.Index(part, "]") == len(part)-1 {
			pointsTo := part[:strings.Index(part, "[")]
			mapPart := part[strings.Index(part, "[")+1 : len(part)-2]
			keyValue := mapPart[strings.Index(mapPart, ":")+1:]
			globbed := false
			if keyValue == "" || keyValue == "*" {
				keyValue = ""
				globbed = true
			}
			list := List{
				KeyField: mapPart[:strings.Index(mapPart, ":")],
				KeyValue: &keyValue,
				Globbed:  globbed,
			}
			entries = append(entries, Object{PointsTo: &pointsTo})
			entries = append(entries, list)
		} else {
			name := part
			obj := Object{PointsTo: &name}
			entries = append(entries, obj)
		}
	}
	return entries
}
