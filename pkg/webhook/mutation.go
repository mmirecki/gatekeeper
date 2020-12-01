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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type mutationResponse string

const (
	mutationSkipResponse    mutationResponse = "skip"
	mutationUnknownResponse mutationResponse = "unknown"
	mutationAllowResponse   mutationResponse = "allow"
	mutationErrorResponse   mutationResponse = "error"
)

func init() {
	AddToManagerFuncs = append(AddToManagerFuncs, AddMutatingWebhook)

	if err := apis.AddToScheme(runtimeScheme); err != nil {
		log.Error(err, "unable to add to scheme")
		panic(err)
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/v1/mutate,mutating=true,failurePolicy=ignore,groups=*,resources=*,versions=*,name=mutation.gatekeeper.sh
// +kubebuilder:rbac:groups=*,resources=*,verbs=get;list;watch;update

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
	log.Info("### MUTATION Webhook HANDLE", "req name", req.Name)
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

	requestResponse := mutationUnknownResponse
	defer func() {
		if h.reporter != nil {
			if err := h.reporter.ReportMutationRequest(
				requestResponse, time.Since(timeStart)); err != nil {
				log.Error(err, "failed to report request")
			}
		}
	}()

	// namespace is excluded from webhook using config
	if h.skipExcludedNamespace(req.AdmissionRequest.Namespace, process.Mutation) {
		requestResponse = mutationSkipResponse
		return admission.ValidationResponse(true, "Namespace is set to be ignored by Gatekeeper config")
	}
	log.Info("### MUTATION Webhook mutateRequest START")

	resp, err := h.mutateRequest(ctx, req)
	log.Info("### MUTATION Webhook mutateRequest AFTER", "resp", resp)

	if err != nil {
		requestResponse = mutationErrorResponse
		return admission.Errored(int32(http.StatusInternalServerError), err)
	}
	requestResponse = mutationAllowResponse
	return resp
}

// traceSwitch returns true if a request should be traced
func (h *mutationHandler) mutateRequest(ctx context.Context, req admission.Request) (admission.Response, error) {
	okResp := admission.Response{
		AdmissionResponse: admissionv1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Code: int32(http.StatusOK),
			},
		},
	}

	ns := &corev1.Namespace{}

	if err := h.client.Get(ctx, types.NamespacedName{Name: req.AdmissionRequest.Namespace}, ns); err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Info("Namespace not found", "name", req.AdmissionRequest.Namespace)
			return okResp, nil
		}
		// bypass cached client and ask api-server directly
		err = h.reader.Get(ctx, types.NamespacedName{Name: req.AdmissionRequest.Namespace}, ns)
		if err != nil {
			log.Info("Namespace not found", "name", req.AdmissionRequest.Namespace)
			return okResp, nil
		}
	}

	obj := unstructured.Unstructured{}
	err := obj.UnmarshalJSON(req.Object.Raw)
	if err != nil {
		log.Info("Failed to unmarshal", "object", string(req.Object.Raw))
		return okResp, nil
	}

	err = h.mutationSystem.Mutate(&obj, ns)
	if err != nil {
		return okResp, nil
	}

	newJSON, err := obj.MarshalJSON()
	log.Info("Mutated", "object", obj, "json", string(newJSON))
	if err != nil {
		log.Info("Failed to marshal", "object", obj)
		return okResp, nil
	}
	resp := admission.PatchResponseFromRaw(req.Object.Raw, newJSON)
	log.Info("Response", "resp", resp)
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
	assignMetadata := obj.(*mutationsv1alpha1.AssignMetadata)
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
	assign := obj.(*mutationsv1alpha1.Assign)
	err = mutation.IsValidAssign(assign)
	if err != nil {
		return true, err
	}

	return false, nil
}
