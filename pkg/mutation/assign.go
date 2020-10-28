package mutation

import (
	"fmt"

	mutationsv1 "github.com/open-policy-agent/gatekeeper/apis/mutations/v1alpha1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

// MetadataMutator is a mutator wrapping
// the AssignMetadata type.
type AssignMutator struct {
	*mutationsv1.Assign
}

func (m AssignMutator) Obj() runtime.Object {
	return m.Assign
}

// Mutate tries to apply the mutation to the given object.
func (m AssignMutator) Mutate(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	copy := obj.DeepCopy()

	// TODO: mutate here

	return copy, nil
}

// Matches verifies if the given object belonging to the given namespace
// matches the current mutator.
func (m AssignMutator) Matches(obj *unstructured.Unstructured, gvk metav1.GroupVersionKind, ns *corev1.Namespace) (bool, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return false, fmt.Errorf("Accessor failed for %s", obj.GetObjectKind().GroupVersionKind().Kind)
	}

	for _, k := range m.Spec.Match.Kinds {
		if k.Kinds != gvk.Kind ||
			k.APIGroups != gvk.Group {
			return false, nil
		}
	}

	if m.Spec.Match.Scope == apiextensionsv1beta1.ClusterScoped &&
		meta.GetNamespace() != "" {
		return false, nil
	}

	if m.Spec.Match.Scope == apiextensionsv1beta1.NamespaceScoped &&
		meta.GetNamespace() == "" {
		return false, nil
	}

	found := false
	for _, n := range m.Spec.Match.Namespaces {
		if meta.GetNamespace() == n {
			found = true
			break
		}
	}
	if !found && len(m.Spec.Match.Namespaces) > 0 {
		return false, nil
	}

	for _, n := range m.Spec.Match.ExcludedNamespaces {
		if meta.GetNamespace() == n {
			return false, nil
		}
	}
	if m.Spec.Match.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(m.Spec.Match.LabelSelector)
		if err != nil {
			return false, err
		}
		if !selector.Matches(labels.Set(meta.GetLabels())) {
			return false, nil
		}
	}

	if m.Spec.Match.NamespaceSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(m.Spec.Match.NamespaceSelector)
		if err != nil {
			return false, err
		}
		if !selector.Matches(labels.Set(ns.Labels)) {
			return false, nil
		}
	}

	return true, nil
}
