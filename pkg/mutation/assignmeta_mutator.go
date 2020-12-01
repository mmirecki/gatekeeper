package mutation

import (
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	mutationsv1alpha1 "github.com/open-policy-agent/gatekeeper/apis/mutations/v1alpha1"
	"github.com/open-policy-agent/gatekeeper/pkg/mutation/path/parser"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	labelsValidSubPath = []parser.Node{
		&parser.Object{
			Reference: "metadata",
		},
		&parser.Object{
			Reference: "labels",
		},
	}

	annotationValidSubPath = []parser.Node{
		&parser.Object{
			Reference: "metadata",
		},
		&parser.Object{
			Reference: "annotations",
		},
	}
)

//AssignMetadataMutator is a mutator built out of an
// AssignMeta instance.
type AssignMetadataMutator struct {
	id             ID
	assignMetadata *mutationsv1alpha1.AssignMetadata
	path           *parser.Path
}

// assignMetadataMutator implements mutator
var _ Mutator = &AssignMetadataMutator{}

func (m *AssignMetadataMutator) Matches(obj metav1.Object, ns *corev1.Namespace) bool {
	// TODO implement using matches function
	return false
}

func (m *AssignMetadataMutator) Mutate(obj *unstructured.Unstructured) error {
	return Mutate(m, obj)
}
func (m *AssignMetadataMutator) ID() ID {
	return m.id
}

func (m *AssignMetadataMutator) Path() *parser.Path {
	return m.path
}

func (m *AssignMetadataMutator) HasDiff(mutator Mutator) bool {
	toCheck, ok := mutator.(*AssignMetadataMutator)
	if !ok { // different types, different
		return true
	}

	if !cmp.Equal(toCheck.id, m.id) {
		return true
	}
	// any difference in spec may be enough
	if !cmp.Equal(toCheck.assignMetadata.Spec, m.assignMetadata.Spec) {
		return true
	}

	return false
}

func (m *AssignMetadataMutator) DeepCopy() Mutator {
	res := &AssignMetadataMutator{
		id:             m.id,
		assignMetadata: m.assignMetadata.DeepCopy(),
		path: &parser.Path{
			Nodes: make([]parser.Node, len(m.path.Nodes)),
		},
	}
	copy(res.path.Nodes, m.path.Nodes)
	return res
}

func (m *AssignMetadataMutator) Value() (interface{}, error) {
	return unmarshalValue(m.assignMetadata.Spec.Parameters.Assign.Raw)
}

// MutatorForAssignMetadata builds an AssignMetadataMutator from the given AssignMetadata object.
func MutatorForAssignMetadata(assignMeta *mutationsv1alpha1.AssignMetadata) (*AssignMetadataMutator, error) {
	id, err := MakeID(assignMeta)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to retrieve id for assignMetadata type")
	}

	path, err := parser.Parse(assignMeta.Spec.Location)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse location for assign metadata")
	}

	if !isMetadataPath(path) {
		return nil, fmt.Errorf("Invalid location for assignmetadata: %s", assignMeta.Spec.Location)
	}
	return &AssignMetadataMutator{
		id:             id,
		assignMetadata: assignMeta.DeepCopy(),
		path:           path,
	}, nil
}

// Verifies that the given path is valid for metadata
func isMetadataPath(path *parser.Path) bool {
	// Path must be metadata.annotations.something or metadata.labels.something
	if len(path.Nodes) != 3 ||
		path.Nodes[0].Type() != parser.ObjectNode ||
		path.Nodes[1].Type() != parser.ObjectNode ||
		path.Nodes[2].Type() != parser.ObjectNode {

		return false
	}

	if reflect.DeepEqual(path.Nodes[0:2], labelsValidSubPath) {
		return true
	}
	if reflect.DeepEqual(path.Nodes[0:2], annotationValidSubPath) {
		return true
	}
	return false
}
