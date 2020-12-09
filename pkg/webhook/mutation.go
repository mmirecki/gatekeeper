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

package webhook

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	opa "github.com/open-policy-agent/frameworks/constraint/pkg/client"
	"github.com/open-policy-agent/gatekeeper/apis"
	mutationsv1alpha1 "github.com/open-policy-agent/gatekeeper/apis/mutations/v1alpha1"
	"github.com/open-policy-agent/gatekeeper/pkg/controller/config/process"
	"github.com/open-policy-agent/gatekeeper/pkg/mutation"
	"github.com/open-policy-agent/gatekeeper/pkg/util"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func init() {
	AddToManagerFuncs = append(AddToManagerFuncs, AddMutatingWebhook)

	if err := apis.AddToScheme(runtimeScheme); err != nil {
		log.Error(err, "unable to add to scheme")
		panic(err)
	}
}

// TODO enable this once mutation is beta +kubebuilder:webhook:verbs=create;update,path=/v1/mutate,mutating=true,failurePolicy=ignore,groups=*,resources=*,versions=*,name=mutation.gatekeeper.sh
// TODO enable this once mutation is beta +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;update

// AddMutatingWebhook registers the mutating webhook server with the manager
func AddMutatingWebhook(mgr manager.Manager, client *opa.Client, processExcluder *process.Excluder, mutationSystem *mutation.System) error {
	if !*mutation.MutationEnabled {
		return nil
	}
	reporter, err := newStatsReporter()
	if err != nil {
		return err
	}
	eventBroadcaster := record.NewBroadcaster()
	kubeClient := kubernetes.NewForConfigOrDie(mgr.GetConfig())

	eventBroadcaster.StartRecordingToSink(&clientcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{Component: "gatekeeper-mutation-webhook"})

	wh := &admission.Webhook{
		Handler: &mutationHandler{
			webhookHandler: webhookHandler{
				client:          mgr.GetClient(),
				reader:          mgr.GetAPIReader(),
				reporter:        reporter,
				processExcluder: processExcluder,
				eventRecorder:   recorder,
				gkNamespace:     util.GetNamespace(),
			},
			mutationSystem: mutationSystem,
		},
	}

	// TODO(https://github.com/open-policy-agent/gatekeeper/issues/661): remove log injection if the race condition in the cited bug is eliminated.
	// Otherwise we risk having unstable logger names for the webhook.
	if err := wh.InjectLogger(log); err != nil {
		return err
	}
	mgr.GetWebhookServer().Register("/v1/mutate", wh)

	return nil
}

var _ admission.Handler = &mutationHandler{}

type mutationHandler struct {
	webhookHandler
	mutationSystem *mutation.System
}

// Handle the mutation request
func (h *mutationHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := log.WithValues("hookType", "mutation")
	var timeStart = time.Now()

	if isGkServiceAccount(req.AdmissionRequest.UserInfo) {
		return admission.ValidationResponse(true, "Gatekeeper does not self-manage")
	}

	if req.AdmissionRequest.Operation != admissionv1beta1.Create &&
		req.AdmissionRequest.Operation != admissionv1beta1.Update {
		return admission.ValidationResponse(true, "Mutating only on create or update")
	}

	if userErr, err := h.validateGatekeeperResources(ctx, req); err != nil {
		vResp := admission.ValidationResponse(false, err.Error())
		if vResp.Result == nil {
			vResp.Result = &metav1.Status{}
		}
		if userErr {
			vResp.Result.Code = http.StatusUnprocessableEntity
		} else {
			vResp.Result.Code = http.StatusInternalServerError
		}
		return vResp
	}

	if h.isGatekeeperResource(ctx, req) {
		return admission.ValidationResponse(true, "Not mutating gatekeeper resources")
	}

	requestResponse := unknownResponse
	defer func() {
		if h.reporter != nil {
			if err := h.reporter.ReportMutationRequest(requestResponse, time.Since(timeStart)); err != nil {
				log.Error(err, "failed to report request")
			}
		}
	}()

	// namespace is excluded from webhook using config
	if h.skipExcludedNamespace(req.AdmissionRequest.Namespace, process.Mutation) {
		requestResponse = skipResponse
		return admission.ValidationResponse(true, "Namespace is set to be ignored by Gatekeeper config")
	}

	resp, err := h.mutateRequest(ctx, req)
	if err != nil {
		requestResponse = errorResponse
		return admission.Errored(int32(http.StatusInternalServerError), err)
	}
	requestResponse = successResponse
	return resp
}

func (h *mutationHandler) mutateRequest(ctx context.Context, req admission.Request) (admission.Response, error) {
	// TODO: place mutation logic here
	mutatedObject := req.Object
	resp := admission.PatchResponseFromRaw(req.Object.Raw, mutatedObject.Raw)
	return resp, nil
}

func AppendMutationWebhookIfEnabled(webhooks []rotator.WebhookInfo) []rotator.WebhookInfo {
	if *mutation.MutationEnabled {
		return append(webhooks, rotator.WebhookInfo{
			Name: MwhName,
			Type: rotator.Mutating,
		})
	}
	return webhooks
}

// validateGatekeeperResources returns whether an issue is user error (vs internal) and any errors
// validating internal resources
func (h *mutationHandler) validateGatekeeperResources(ctx context.Context, req admission.Request) (bool, error) {
	if req.AdmissionRequest.Kind.Group == mutationsGroup && req.AdmissionRequest.Kind.Kind == "AssignMetadata" {
		return h.validateAssignMetadata(ctx, req)
	}
	if req.AdmissionRequest.Kind.Group == mutationsGroup && req.AdmissionRequest.Kind.Kind == "Assign" {
		return h.validateAssign(ctx, req)
	}
	return false, nil
}

func (h *mutationHandler) validateAssignMetadata(ctx context.Context, req admission.Request) (bool, error) {
	obj, _, err := deserializer.Decode(req.AdmissionRequest.Object.Raw, nil, &mutationsv1alpha1.AssignMetadata{})
	if err != nil {
		return false, err
	}
	assignMetadata, ok := obj.(*mutationsv1alpha1.AssignMetadata)
	if !ok {
		return false, fmt.Errorf("Deserialized object is not of type AssignMetadata")
	}
	err = mutation.IsValidAssignMetadata(assignMetadata)
	if err != nil {
		return true, err
	}

	return false, nil
}

func (h *mutationHandler) validateAssign(ctx context.Context, req admission.Request) (bool, error) {
	obj, _, err := deserializer.Decode(req.AdmissionRequest.Object.Raw, nil, &mutationsv1alpha1.Assign{})
	if err != nil {
		return false, err
	}
	assign, ok := obj.(*mutationsv1alpha1.Assign)
	if !ok {
		return false, fmt.Errorf("Deserialized object is not of type Assign")
	}

	err = mutation.IsValidAssign(assign)
	if err != nil {
		return true, err
	}

	return false, nil
}
