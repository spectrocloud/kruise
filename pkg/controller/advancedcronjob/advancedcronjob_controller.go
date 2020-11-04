/*
Copyright 2020 The Kruise Authors.

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

package advancedcronjob

import (
	"context"
	"flag"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/tools/record"

	"github.com/openkruise/kruise/pkg/util/gate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
)

func init() {
	flag.IntVar(&concurrentReconciles, "AdvancedCronJob-workers", concurrentReconciles, "Max concurrent workers for AdvancedCronJob controller.")
}

var (
	concurrentReconciles = 3
	controllerKind       = appsv1alpha1.SchemeGroupVersion.WithKind("AdvancedCronJob")
	jobOwnerKey          = ".metadata.controller"
	apiGVStr             = appsv1alpha1.GroupVersion.String()
)

// Add creates a new AdvancedCronJob Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	klog.Info("Checking Resource enabled AdvancedCronJob")
	if !gate.ResourceEnabled(&appsv1alpha1.AdvancedCronJob{}) {
		klog.Info("Resource not enabled AdvancedCronJob")
		return nil
	}

	//if err := mgr.GetFieldIndexer().IndexField(&appsv1alpha1.BroadcastJob{}, jobOwnerKey, func(rawObj runtime.Object) []string {
	//	// grab the job object, extract the owner...
	//	job := rawObj.(*appsv1alpha1.BroadcastJob)
	//	owner := metav1.GetControllerOf(job)
	//	if owner == nil {
	//		return nil
	//	}
	//	// ...make sure it's a CronJob...
	//	if owner.APIVersion != apiGVStr || owner.Kind != "AdvancedCronJob" {
	//		return nil
	//	}
	//
	//	// ...and if so, return it
	//	return []string{owner.Name}
	//}); err != nil {
	//	return err
	//}
	//

	recorder := mgr.GetEventRecorderFor("AdvancedCronJob-controller")
	if err := (&ReconcileAdvancedCronJob{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: recorder,
		Log:      ctrl.Log.WithName("controllers").WithName("AdvancedCronJob"),
		Clock:    realClock{},
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "SpectroClusterAction")
		return err
	}
	return nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	klog.Info("Starting AdvancedCronJob Controller")
	c, err := controller.New("AdvancedCronJob-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: concurrentReconciles})
	if err != nil {
		klog.Error(err)
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(&appsv1alpha1.BroadcastJob{}, jobOwnerKey, func(rawObj runtime.Object) []string {
		// grab the job object, extract the owner...
		job := rawObj.(*appsv1alpha1.BroadcastJob)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...make sure it's a CronJob...
		if owner.APIVersion != apiGVStr || owner.Kind != "AdvancedCronJob" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	// Watch for changes to AdvancedCronJob
	err = c.Watch(&source.Kind{Type: &appsv1alpha1.AdvancedCronJob{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		klog.Error(err)
		return err
	}

	//also watch for BroadcastJob and create request for AdvancedCronJob if there is any change
	err = c.Watch(&source.Kind{Type: &appsv1alpha1.BroadcastJob{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appsv1alpha1.AdvancedCronJob{},
	})

	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

type realClock struct{}

func (_ realClock) Now() time.Time { return time.Now() }

// clock knows how to get the current time.
// It can be used to fake out timing for testing.
type Clock interface {
	Now() time.Time
}

var (
	scheduledTimeAnnotation = "apps.kruise.io/scheduled-at"
)

var _ reconcile.Reconciler = &ReconcileAdvancedCronJob{}

// ReconcileAdvancedCronJob reconciles a AdvancedCronJob object
type ReconcileAdvancedCronJob struct {
	client.Client
	Log      logr.Logger
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	Clock
}

// +kubebuilder:rbac:groups=apps.kruise.io,resources=advancedcronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kruise.io,resources=advancedcronjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=advancedcronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=advancedcronjobs/status,verbs=get;update;patch

func (r *ReconcileAdvancedCronJob) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("advancedcronjob", req.NamespacedName)

	ctx := context.Background()
	klog.Infof("Running BroadcastCronJob job %s", req.Name)

	namespacedName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	log := r.Log.WithValues("cronjob", namespacedName)

	var advancedCronJob appsv1alpha1.AdvancedCronJob

	if err := r.Get(ctx, namespacedName, &advancedCronJob); err != nil {
		klog.Error(err, "unable to fetch CronJob")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	switch FindTemplateKind(advancedCronJob.Spec) {
	case appsv1alpha1.JobTemplate:
		return r.reconcileJob(ctx, log, advancedCronJob)
	case appsv1alpha1.BroadcastJobTemplate:
		return r.reconcileBroadcastJob(ctx, log, advancedCronJob)
	}

	return ctrl.Result{}, nil
}

func (r *ReconcileAdvancedCronJob) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.AdvancedCronJob{}).
		Complete(r)
}
