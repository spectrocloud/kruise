/*
Copyright 2019 The Kruise Authors.

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

package validating

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	batchv1beta1 "k8s.io/api/batch/v1beta1"

	corevalidation "k8s.io/kubernetes/pkg/apis/core/validation"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
	v1 "k8s.io/api/core/v1"
	genericvalidation "k8s.io/apimachinery/pkg/api/validation"
	validationutil "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core"
	corev1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	AdvancedCronJobNameMaxLen = 63
)

var (
	validateAdvancedCronJobNameMsg   = "AdvancedCronJob name must consist of alphanumeric characters or '-'"
	validateAdvancedCronJobNameRegex = regexp.MustCompile(validAdvancedCronJobNameFmt)
	validAdvancedCronJobNameFmt      = `^[a-zA-Z0-9\-]+$`
)

// AdvancedCronJobCreateUpdateHandler handles AdvancedCronJob
type AdvancedCronJobCreateUpdateHandler struct {
	Client client.Client

	// Decoder decodes objects
	Decoder *admission.Decoder
}

func (h *AdvancedCronJobCreateUpdateHandler) validatingAdvancedCronJobFn(ctx context.Context, obj *appsv1alpha1.AdvancedCronJob) (bool, string, error) {

	allErrs := h.validateAdvancedCronJob(obj)
	if len(allErrs) != 0 {
		return false, "", allErrs.ToAggregate()
	}
	return true, "allowed to be admitted", nil
}

func (h *AdvancedCronJobCreateUpdateHandler) validateAdvancedCronJob(obj *appsv1alpha1.AdvancedCronJob) field.ErrorList {
	allErrs := genericvalidation.ValidateObjectMeta(&obj.ObjectMeta, true, validateAdvancedCronJobName, field.NewPath("metadata"))
	allErrs = append(allErrs, validateAdvancedCronJobSpec(&obj.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateAdvancedCronJobSpec(spec *appsv1alpha1.AdvancedCronJobSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	//validate multiple template
	isOnlyOneTemplate := false
	if spec.JobTemplate != nil {
		isOnlyOneTemplate = true
	} else if isOnlyOneTemplate && spec.BroadcastJobTemplate != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("spec").Child("broadcastJobTemplate"),
			spec.BroadcastJobTemplate,
			"either job template or broadcast job template can be present"))
		return allErrs
	}

	if spec.JobTemplate != nil {
		allErrs = append(allErrs, validateJobTemplateSpec(spec.JobTemplate, fldPath)...)
	}

	if spec.BroadcastJobTemplate != nil {
		allErrs = append(allErrs, validateBroadcastJobTemplateSpec(spec.BroadcastJobTemplate, fldPath)...)
	}

	return allErrs
}

func validateJobTemplateSpec(jobSpec *batchv1beta1.JobTemplateSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	coreTemplate, err := convertPodTemplateSpec(&jobSpec.Spec.Template)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Root(), jobSpec.Spec.Template, fmt.Sprintf("Convert_v1_PodTemplateSpec_To_core_PodTemplateSpec failed: %v", err)))
		return allErrs
	}
	return append(allErrs, corevalidation.ValidatePodTemplateSpec(coreTemplate, fldPath.Child("template"))...)
}

func validateBroadcastJobTemplateSpec(brJobSpec *appsv1alpha1.BroadcastJobTemplateSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	coreTemplate, err := convertPodTemplateSpec(&brJobSpec.Spec.Template)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Root(), brJobSpec.Spec.Template, fmt.Sprintf("Convert_v1_PodTemplateSpec_To_core_PodTemplateSpec failed: %v", err)))
		return allErrs
	}
	return append(allErrs, corevalidation.ValidatePodTemplateSpec(coreTemplate, fldPath.Child("template"))...)
}

func convertPodTemplateSpec(template *v1.PodTemplateSpec) (*core.PodTemplateSpec, error) {
	coreTemplate := &core.PodTemplateSpec{}
	if err := corev1.Convert_v1_PodTemplateSpec_To_core_PodTemplateSpec(template.DeepCopy(), coreTemplate, nil); err != nil {
		return nil, err
	}
	return coreTemplate, nil
}

func validateAdvancedCronJobName(name string, prefix bool) (allErrs []string) {
	if !validateAdvancedCronJobNameRegex.MatchString(name) {
		allErrs = append(allErrs, validationutil.RegexError(validateAdvancedCronJobNameMsg, validAdvancedCronJobNameFmt, "example-com"))
	}
	if len(name) > AdvancedCronJobNameMaxLen {
		allErrs = append(allErrs, validationutil.MaxLenError(AdvancedCronJobNameMaxLen))
	}
	return allErrs
}

func (h *AdvancedCronJobCreateUpdateHandler) validateAdvancedCronJobUpdate(obj, oldObj *appsv1alpha1.AdvancedCronJob) field.ErrorList {
	allErrs := genericvalidation.ValidateObjectMeta(&obj.ObjectMeta, true, validateAdvancedCronJobName, field.NewPath("metadata"))

	//check if advanved cron job exists
	namespacedName := types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Name,
	}

	var advancedCronJob appsv1alpha1.AdvancedCronJob

	if err := h.Client.Get(context.Background(), namespacedName, &advancedCronJob); err != nil {
		klog.Error(err, "unable to fetch CronJob")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		allErrs = append(allErrs, field.InternalError(field.NewPath("object"), err))
		return allErrs
	}

	allErrs = append(allErrs, validateAdvancedCronJobSpec(&obj.Spec, field.NewPath("spec"))...)
	return allErrs
}

var _ admission.Handler = &AdvancedCronJobCreateUpdateHandler{}

// Handle handles admission requests.
func (h *AdvancedCronJobCreateUpdateHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	obj := &appsv1alpha1.AdvancedCronJob{}

	err := h.Decoder.Decode(req, obj)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.AdmissionRequest.Operation {
	case admissionv1beta1.Create:
		if allErrs := h.validateAdvancedCronJob(obj); len(allErrs) > 0 {
			return admission.Errored(http.StatusUnprocessableEntity, allErrs.ToAggregate())
		}
	case admissionv1beta1.Update:
		oldObj := &appsv1alpha1.AdvancedCronJob{}
		if err := h.Decoder.DecodeRaw(req.AdmissionRequest.OldObject, oldObj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if allErrs := h.validateAdvancedCronJobUpdate(obj, oldObj); len(allErrs) > 0 {
			return admission.Errored(http.StatusUnprocessableEntity, allErrs.ToAggregate())
		}
	}

	return admission.ValidationResponse(true, "")
}

var _ inject.Client = &AdvancedCronJobCreateUpdateHandler{}

// InjectClient injects the client into the AdvancedCronJobCreateUpdateHandler
func (h *AdvancedCronJobCreateUpdateHandler) InjectClient(c client.Client) error {
	h.Client = c
	return nil
}

var _ admission.DecoderInjector = &AdvancedCronJobCreateUpdateHandler{}

// InjectDecoder injects the decoder into the AdvancedCronJobCreateUpdateHandler
func (h *AdvancedCronJobCreateUpdateHandler) InjectDecoder(d *admission.Decoder) error {
	h.Decoder = d
	return nil
}
