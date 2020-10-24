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

package broadcastcronjob

import (
	"context"
	"flag"
	"time"

	"k8s.io/klog"

	"k8s.io/client-go/tools/record"

	"github.com/openkruise/kruise/pkg/util/gate"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	appsv1alpha1 "github.com/openkruise/kruise/apis/apps/v1alpha1"
)

func init() {
	flag.IntVar(&concurrentReconciles, "broadcastcronjob-workers", concurrentReconciles, "Max concurrent workers for BroadCastCronJob controller.")
}

var (
	concurrentReconciles = 3
	controllerKind       = appsv1alpha1.SchemeGroupVersion.WithKind("BroadcastCronJob")
)

// Add creates a new BroadcastCronJob Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	klog.Info("Checking Resource enabled BroadcastCronJob")
	if !gate.ResourceEnabled(&appsv1alpha1.BroadcastCronJob{}) {
		klog.Info("Resource not enabled BroadcastCronJob")
		return nil
	}
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	recorder := mgr.GetEventRecorderFor("broadcastcronjob-controller")
	return &ReconcileBroadcastCronJob{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: recorder,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	klog.Info("Starting BroadcastCronJob Controller")
	c, err := controller.New("broadcastcronjob-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: concurrentReconciles})
	if err != nil {
		klog.Error(err)
		return err
	}

	// Watch for changes to BroadcastCronJob
	err = c.Watch(&source.Kind{Type: &appsv1alpha1.BroadcastCronJob{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		klog.Error(err)
		return err
	}

	//also watch for BroadcastJob and create request for BroadcastCronJob if there is any change
	err = c.Watch(&source.Kind{Type: &appsv1alpha1.BroadcastJob{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appsv1alpha1.BroadcastCronJob{},
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

var _ reconcile.Reconciler = &ReconcileBroadcastCronJob{}

// ReconcileBroadcastCronJob reconciles a BroadcastCronJob object
type ReconcileBroadcastCronJob struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kruise.io,resources=broadcastcronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kruise.io,resources=broadcastcronjobs/status,verbs=get;update;patch

func (r *ReconcileBroadcastCronJob) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	klog.Infof("Running BroadcastCronJob job %s", req.Name)

	return ctrl.Result{}, nil
}

func (r *ReconcileBroadcastCronJob) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.BroadcastCronJob{}).
		Owns(&appsv1alpha1.BroadcastJob{}).
		Complete(r)
}
