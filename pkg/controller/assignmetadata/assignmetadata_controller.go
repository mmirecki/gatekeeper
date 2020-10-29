/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package assignmentmetadata

import (
	"context"
	"strings"

	opa "github.com/open-policy-agent/frameworks/constraint/pkg/client"
	mutationsv1alpha1 "github.com/open-policy-agent/gatekeeper/apis/mutations/v1alpha1"
	"github.com/open-policy-agent/gatekeeper/pkg/logging"
	"github.com/open-policy-agent/gatekeeper/pkg/mutation"
	"github.com/open-policy-agent/gatekeeper/pkg/readiness"
	"github.com/open-policy-agent/gatekeeper/pkg/util"
	"github.com/open-policy-agent/gatekeeper/pkg/watch"
	"k8s.io/apimachinery/pkg/api/errors"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log = logf.Log.WithName("controller").WithValues(logging.Process, "assignmentmetadata_controller")
)

type Adder struct {
	Opa              *opa.Client
	WatchManager     *watch.Manager
	ControllerSwitch *watch.ControllerSwitch
	Tracker          *readiness.Tracker
	MutationCache    *mutation.Cache
}

// Add creates a new AssignMetadata Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (a *Adder) Add(mgr manager.Manager) error {

	r := newReconciler(mgr, a.MutationCache)
	events := make(chan event.GenericEvent, 1024)
	return add(mgr, r, events)
}

func (a *Adder) InjectOpa(o *opa.Client) {
	a.Opa = o
}

func (a *Adder) InjectWatchManager(w *watch.Manager) {
	a.WatchManager = w
}

func (a *Adder) InjectControllerSwitch(cs *watch.ControllerSwitch) {
	a.ControllerSwitch = cs
}

func (a *Adder) InjectTracker(t *readiness.Tracker) {
	a.Tracker = t
}

func (a *Adder) InjectMutationCache(c *mutation.Cache) {
	a.MutationCache = c
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, cache *mutation.Cache) *AssignMetadataReconciler {
	r := &AssignMetadataReconciler{cache: cache, Client: mgr.GetClient()}
	return r
}

type mapper struct {
	packer util.EventPacker
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler, events <-chan event.GenericEvent) error {
	// Create a new controller
	c, err := controller.New("assignmetadata-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to the provided constraint
	err = c.Watch(
		&source.Channel{
			Source:         events,
			DestBufferSize: 1024,
		},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: util.EventPacker{}},
	)
	if err != nil {
		return err
	}

	// Watch for changes to ConstraintStatus
	err = c.Watch(
		&source.Kind{Type: &mutationsv1alpha1.AssignMetadata{}},
		&handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

// AssignMetadataReconciler reconciles a AssignMetadata object
type AssignMetadataReconciler struct {
	client.Client
	cache *mutation.Cache
}

// +kubebuilder:rbac:groups=assignmetadata.gatekeeper.sh,resources=*,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a constraint object and makes changes based on the state read
// and what is in the constraint.Spec
func (r *AssignMetadataReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Info("Reconcile", "request", request)
	deleted := false
	m := &mutationsv1alpha1.AssignMetadata{}
	err := r.Get(context.TODO(), request.NamespacedName, m)
	if err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		deleted = true
		// be sure we are using a blank constraint template so that
		// we know finalizer removal code won't break (can be removed once that
		// code is removed)
		m = &mutationsv1alpha1.AssignMetadata{}
	}
	deleted = deleted || !m.GetDeletionTimestamp().IsZero()

	if !deleted {
		err := r.cache.Insert(mutation.MetadataMutator{AssignMetadata: m.DeepCopy()})
		if err != nil {
			log.Error(err, "Failed to insert")
		}
		log.Info("!!!! PARSING LOCATION ", "resource", m)
		typeEntries := parseLocation(m)
		log.Info("!!!!         PARSED ", "typeEntries", typeEntries)

	} else {
		err := r.cache.Remove(mutation.MetadataMutator{AssignMetadata: m.DeepCopy()})
		if err != nil {
			log.Error(err, "Failed to remove")
		}
	}
	return ctrl.Result{}, nil
}

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
	return o.TypeName
}

type List struct {
	TypeName Type
	KeyField string  // The key field for the member objects
	KeyValue *string // The value of the keyfield, must be populated
	// for the last item in the PathEntry array
	Globbed bool // Globbed. This cannot be true if keyValue is
	// set
}

func (o List) Type() Type {
	return o.TypeName
}

func parseLocation(m *mutationsv1alpha1.AssignMetadata) []PathEntry {
	location := m.Spec.Location
	log.Info("######    LOCATION ", "location", location)

	locationParts := strings.Split(location, ".")
	entries := make([]PathEntry, 0)

	for _, part := range locationParts {
		log.Info("######     ", "part", part)
		log.Info("######     ", "Contains", strings.Contains(part, "["))
		log.Info("######     ", "Index", strings.Index(part, "]") == len(part)-1)
		log.Info("######     ", "Index1", strings.Index(part, "]"))
		log.Info("######     ", "Index2", len(part)-1)

		if strings.Contains(part, "[") && strings.Index(part, "]") == len(part)-1 {
			log.Info("######    LIST!!! ")

			pointsTo := part[:strings.Index(part, "[")]
			key := part[strings.Index(part, "["):]
			keyValue := key[strings.Index(part, ":"):]
			globbed := false
			if keyValue == "" || keyValue == "*" {
				keyValue = ""
				globbed = true
			}
			obj := Object{PointsTo: &pointsTo}

			list := List{
				KeyField: key[:strings.Index(part, ":")],
				KeyValue: &keyValue,
				Globbed:  globbed,
			}
			entries = append(entries, obj)
			entries = append(entries, list)
		} else {
			log.Info("######    OBJECT!!! ")
			name := part
			obj := Object{PointsTo: &name}
			entries = append(entries, obj)

		}

	}
	return entries
}
