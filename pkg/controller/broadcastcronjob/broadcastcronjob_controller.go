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
	"fmt"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ref "k8s.io/client-go/tools/reference"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/klog"

	"k8s.io/client-go/tools/record"

	"github.com/openkruise/kruise/pkg/util/gate"
	"github.com/robfig/cron"
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
	jobOwnerKey          = ".metadata.controller"
	apiGVStr             = appsv1alpha1.GroupVersion.String()
)

// Add creates a new BroadcastCronJob Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	klog.Info("Checking Resource enabled BroadcastCronJob")
	if !gate.ResourceEnabled(&appsv1alpha1.BroadcastCronJob{}) {
		klog.Info("Resource not enabled BroadcastCronJob")
		return nil
	}

	if err := mgr.GetFieldIndexer().IndexField(&appsv1alpha1.BroadcastJob{}, jobOwnerKey, func(rawObj runtime.Object) []string {
		// grab the job object, extract the owner...
		job := rawObj.(*appsv1alpha1.BroadcastJob)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...make sure it's a CronJob...
		if owner.APIVersion != apiGVStr || owner.Kind != "BroadcastCronJob" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	recorder := mgr.GetEventRecorderFor("broadcastcronjob-controller")
	if err := (&ReconcileBroadcastCronJob{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: recorder,
		Log:      ctrl.Log.WithName("controllers").WithName("BroadcastCronJob"),
		Clock:    realClock{},
	}).SetupWithManager(mgr); err != nil {
		klog.Error(err, "unable to create controller", "controller", "SpectroClusterAction")
		return err
	}
	return nil
}

//// newReconciler returns a new reconcile.Reconciler
//func newReconciler(mgr manager.Manager) reconcile.Reconciler {
//
//	return &ReconcileBroadcastCronJob{
//		Client:   mgr.GetClient(),
//		scheme:   mgr.GetScheme(),
//		recorder: recorder,
//		Log:      ctrl.Log.WithName("controllers").WithName("BroadcastCronJob"),
//		Clock:    realClock{},
//	}
//}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	klog.Info("Starting BroadcastCronJob Controller")
	c, err := controller.New("broadcastcronjob-controller", mgr, controller.Options{Reconciler: r, MaxConcurrentReconciles: concurrentReconciles})
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
		if owner.APIVersion != apiGVStr || owner.Kind != "BroadcastCronJob" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
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
	Log      logr.Logger
	Clock
}

// +kubebuilder:rbac:groups=apps.kruise.io,resources=broadcastcronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kruise.io,resources=broadcastcronjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=broadcastcronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=broadcastcronjobs/status,verbs=get;update;patch

func (r *ReconcileBroadcastCronJob) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	klog.Infof("Running BroadcastCronJob job %s", req.Name)

	namespacedName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	log := r.Log.WithValues("cronjob", namespacedName)

	var brCronJob appsv1alpha1.BroadcastCronJob

	if err := r.Get(ctx, namespacedName, &brCronJob); err != nil {
		klog.Error(err, "unable to fetch CronJob")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var childJobs appsv1alpha1.BroadcastJobList
	if err := r.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		klog.Error(err, "unable to list child Jobs")
		return ctrl.Result{}, err
	}

	var activeJobs []*appsv1alpha1.BroadcastJob
	var successfulJobs []*appsv1alpha1.BroadcastJob
	var failedJobs []*appsv1alpha1.BroadcastJob
	var mostRecentTime *time.Time
	isJobFinished := func(job *appsv1alpha1.BroadcastJob) (bool, appsv1alpha1.JobConditionType) {
		for _, c := range job.Status.Conditions {
			if (c.Type == appsv1alpha1.JobComplete || c.Type == appsv1alpha1.JobFailed) && c.Status == corev1.ConditionTrue {
				return true, c.Type
			}
		}

		return false, ""
	}

	// +kubebuilder:docs-gen:collapse=isJobFinished
	getScheduledTimeForJob := func(job *appsv1alpha1.BroadcastJob) (*time.Time, error) {
		timeRaw := job.Annotations[scheduledTimeAnnotation]
		if len(timeRaw) == 0 {
			return nil, nil
		}

		timeParsed, err := time.Parse(time.RFC3339, timeRaw)
		if err != nil {
			return nil, err
		}
		return &timeParsed, nil
	}

	// +kubebuilder:docs-gen:collapse=getScheduledTimeForJob

	for i, job := range childJobs.Items {
		_, finishedType := isJobFinished(&job)
		switch finishedType {
		case "": // ongoing
			activeJobs = append(activeJobs, &childJobs.Items[i])
		case appsv1alpha1.JobFailed:
			failedJobs = append(failedJobs, &childJobs.Items[i])
		case appsv1alpha1.JobComplete:
			successfulJobs = append(successfulJobs, &childJobs.Items[i])
		}

		// We'll store the launch time in an annotation, so we'll reconstitute that from
		// the active jobs themselves.
		scheduledTimeForJob, err := getScheduledTimeForJob(&job)
		if err != nil {
			klog.Error(err, "unable to parse schedule time for child job", "job", &job)
			continue
		}
		if scheduledTimeForJob != nil {
			if mostRecentTime == nil {
				mostRecentTime = scheduledTimeForJob
			} else if mostRecentTime.Before(*scheduledTimeForJob) {
				mostRecentTime = scheduledTimeForJob
			}
		}
	}

	if mostRecentTime != nil {
		brCronJob.Status.LastScheduleTime = &metav1.Time{Time: *mostRecentTime}
	} else {
		brCronJob.Status.LastScheduleTime = nil
	}

	brCronJob.Status.Active = nil
	for _, activeJob := range activeJobs {
		jobRef, err := ref.GetReference(r.scheme, activeJob)
		if err != nil {
			klog.Error(err, "unable to make reference to active job", "job", activeJob)
			continue
		}
		brCronJob.Status.Active = append(brCronJob.Status.Active, *jobRef)
	}

	klog.V(1).Info("job count", "active jobs", len(activeJobs), "successful jobs", len(successfulJobs), "failed jobs", len(failedJobs))
	//
	//if err := r.Status().Update(ctx, &brCronJob); err != nil {
	//	klog.Error(err, "unable to update CronJob status")
	//	return ctrl.Result{}, err
	//}

	/*
		Once we've updated our status, we can move on to ensuring that the status of
		the world matches what we want in our spec.
		### 3: Clean up old jobs according to the history limit
		First, we'll try to clean up old jobs, so that we don't leave too many lying
		around.
	*/

	// NB: deleting these is "best effort" -- if we fail on a particular one,
	// we won't requeue just to finish the deleting.
	if brCronJob.Spec.FailedJobsHistoryLimit != nil {
		sort.Slice(failedJobs, func(i, j int) bool {
			if failedJobs[i].Status.StartTime == nil {
				return failedJobs[j].Status.StartTime != nil
			}
			return failedJobs[i].Status.StartTime.Before(failedJobs[j].Status.StartTime)
		})
		for i, job := range failedJobs {
			if int32(i) >= int32(len(failedJobs))-*brCronJob.Spec.FailedJobsHistoryLimit {
				break
			}
			if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				klog.Error(err, "unable to delete old failed job", "job", job)
			} else {
				klog.V(0).Info("deleted old failed job", "job", job)
			}
		}
	}

	if brCronJob.Spec.SuccessfulJobsHistoryLimit != nil {
		sort.Slice(successfulJobs, func(i, j int) bool {
			if successfulJobs[i].Status.StartTime == nil {
				return successfulJobs[j].Status.StartTime != nil
			}
			return successfulJobs[i].Status.StartTime.Before(successfulJobs[j].Status.StartTime)
		})
		for i, job := range successfulJobs {
			if int32(i) >= int32(len(successfulJobs))-*brCronJob.Spec.SuccessfulJobsHistoryLimit {
				break
			}
			if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); (err) != nil {
				klog.Error(err, "unable to delete old successful job", "job", job)
			} else {
				klog.V(0).Info("deleted old successful job", "job", job)
			}
		}
	}

	/* ### 4: Check if we're suspended
	If this object is suspended, we don't want to run any jobs, so we'll stop now.
	This is useful if something's broken with the job we're running and we want to
	pause runs to investigate or putz with the cluster, without deleting the object.
	*/

	if brCronJob.Spec.Paused {
		klog.V(1).Info("cronjob paused, skipping")
		return ctrl.Result{}, nil
	}

	/*
		### 5: Get the next scheduled run
		If we're not paused, we'll need to calculate the next scheduled run, and whether
		or not we've got a run that we haven't processed yet.
	*/

	/*
		We'll calculate the next scheduled time using our helpful cron library.
		We'll start calculating appropriate times from our last run, or the creation
		of the CronJob if we can't find a last run.
		If there are too many missed runs and we don't have any deadlines set, we'll
		bail so that we don't cause issues on controller restarts or wedges.
		Otherwise, we'll just return the missed runs (of which we'll just use the latest),
		and the next run, so that we can know when it's time to reconcile again.
	*/
	getNextSchedule := func(cronJob *appsv1alpha1.BroadcastCronJob, now time.Time) (lastMissed time.Time, next time.Time, err error) {
		sched, err := cron.ParseStandard(cronJob.Spec.Schedule)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("Unparseable schedule %q: %v", cronJob.Spec.Schedule, err)
		}

		// for optimization purposes, cheat a bit and start from our last observed run time
		// we could reconstitute this here, but there's not much point, since we've
		// just updated it.
		var earliestTime time.Time
		if cronJob.Status.LastScheduleTime != nil {
			earliestTime = cronJob.Status.LastScheduleTime.Time
		} else {
			earliestTime = cronJob.ObjectMeta.CreationTimestamp.Time
		}
		if cronJob.Spec.StartingDeadlineSeconds != nil {
			// controller is not going to schedule anything below this point
			schedulingDeadline := now.Add(-time.Second * time.Duration(*cronJob.Spec.StartingDeadlineSeconds))

			if schedulingDeadline.After(earliestTime) {
				earliestTime = schedulingDeadline
			}
		}
		if earliestTime.After(now) {
			return time.Time{}, sched.Next(now), nil
		}

		starts := 0
		for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
			lastMissed = t
			// An object might miss several starts. For example, if
			// controller gets wedged on Friday at 5:01pm when everyone has
			// gone home, and someone comes in on Tuesday AM and discovers
			// the problem and restarts the controller, then all the hourly
			// jobs, more than 80 of them for one hourly scheduledJob, should
			// all start running with no further intervention (if the scheduledJob
			// allows concurrency and late starts).
			//
			// However, if there is a bug somewhere, or incorrect clock
			// on controller's server or apiservers (for setting creationTimestamp)
			// then there could be so many missed start times (it could be off
			// by decades or more), that it would eat up all the CPU and memory
			// of this controller. In that case, we want to not try to list
			// all the missed start times.
			starts++
			if starts > 100 {
				// We can't get the most recent times so just return an empty slice
				return time.Time{}, time.Time{}, fmt.Errorf("Too many missed start times (> 100). Set or decrease .spec.startingDeadlineSeconds or check clock skew.")
			}
		}
		return lastMissed, sched.Next(now), nil
	}
	// +kubebuilder:docs-gen:collapse=getNextSchedule

	// figure out the next times that we need to create
	// jobs at (or anything we missed).
	now := realClock{}.Now()
	missedRun, nextRun, err := getNextSchedule(&brCronJob, now)
	if err != nil {
		klog.Error(err, "unable to figure out CronJob schedule")
		// we don't really care about requeuing until we get an update that
		// fixes the schedule, so don't return an error
		return ctrl.Result{}, nil
	}

	/*
		We'll prep our eventual request to requeue until the next job, and then figure
		out if we actually need to run.
	*/
	scheduledResult := ctrl.Result{RequeueAfter: nextRun.Sub(now)} // save this so we can re-use it elsewhere
	log = log.WithValues("now", now, "next run", nextRun)

	/*
		### 6: Run a new job if it's on schedule, not past the deadline, and not blocked by our concurrency policy
		If we've missed a run, and we're still within the deadline to start it, we'll need to run a job.
	*/
	if missedRun.IsZero() {
		klog.V(1).Info("no upcoming scheduled times, sleeping until next")
		return scheduledResult, nil
	}

	// make sure we're not too late to start the run
	log = log.WithValues("current run", missedRun)
	tooLate := false
	if brCronJob.Spec.StartingDeadlineSeconds != nil {
		tooLate = missedRun.Add(time.Duration(*brCronJob.Spec.StartingDeadlineSeconds) * time.Second).Before(now)
	}
	if tooLate {
		klog.V(1).Info("missed starting deadline for last run, sleeping till next")
		// TODO(directxman12): events
		return scheduledResult, nil
	}

	/*
		If we actually have to run a job, we'll need to either wait till existing ones finish,
		replace the existing ones, or just add new ones.  If our information is out of date due
		to cache delay, we'll get a requeue when we get up-to-date information.
	*/
	// figure out how to run this job -- concurrency policy might forbid us from running
	// multiple at the same time...
	if brCronJob.Spec.ConcurrencyPolicy == appsv1alpha1.ForbidConcurrent && len(activeJobs) > 0 {
		klog.V(1).Info("concurrency policy blocks concurrent runs, skipping", "num active", len(activeJobs))
		return scheduledResult, nil
	}

	// ...or instruct us to replace existing ones...
	if brCronJob.Spec.ConcurrencyPolicy == appsv1alpha1.ReplaceConcurrent {
		for _, activeJob := range activeJobs {
			// we don't care if the job was already deleted
			if err := r.Delete(ctx, activeJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete active job", "job", activeJob)
				return ctrl.Result{}, err
			}
		}
	}

	/*
		Once we've figured out what to do with existing jobs, we'll actually create our desired job
	*/

	/*
		We need to construct a job based on our CronJob's template.  We'll copy over the spec
		from the template and copy some basic object meta.
		Then, we'll set the "scheduled time" annotation so that we can reconstitute our
		`LastScheduleTime` field each reconcile.
		Finally, we'll need to set an owner reference.  This allows the Kubernetes garbage collector
		to clean up jobs when we delete the CronJob, and allows controller-runtime to figure out
		which cronjob needs to be reconciled when a given job changes (is added, deleted, completes, etc).
	*/
	constructBrJobForCronJob := func(brCronJob *appsv1alpha1.BroadcastCronJob, scheduledTime time.Time) (*appsv1alpha1.BroadcastJob, error) {
		// We want job names for a given nominal start time to have a deterministic name to avoid the same job being created twice
		name := fmt.Sprintf("%s-%d", brCronJob.Name, scheduledTime.Unix())

		job := &appsv1alpha1.BroadcastJob{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				Name:        name,
				Namespace:   brCronJob.Namespace,
			},
			Spec: *brCronJob.Spec.BroadcastJobTemplate.Spec.DeepCopy(),
		}
		for k, v := range brCronJob.Spec.BroadcastJobTemplate.Annotations {
			job.Annotations[k] = v
		}
		job.Annotations[scheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)
		for k, v := range brCronJob.Spec.BroadcastJobTemplate.Labels {
			job.Labels[k] = v
		}
		if err := ctrl.SetControllerReference(brCronJob, job, r.scheme); err != nil {
			return nil, err
		}

		return job, nil
	}
	// +kubebuilder:docs-gen:collapse=constructJobForCronJob

	// actually make the job...
	job, err := constructBrJobForCronJob(&brCronJob, missedRun)
	if err != nil {
		klog.Error(err, "unable to construct job from template")
		// don't bother requeuing until we get a change to the spec
		return scheduledResult, nil
	}

	// ...and create it on the cluster
	if err := r.Create(ctx, job); err != nil {
		klog.Error(err, "unable to create Job for CronJob", "job", job)
		return ctrl.Result{}, err
	}

	klog.V(1).Info("created Job for CronJob run", "job", job)

	/*
		### 7: Requeue when we either see a running job or it's time for the next scheduled run
		Finally, we'll return the result that we prepped above, that says we want to requeue
		when our next run would need to occur.  This is taken as a maximum deadline -- if something
		else changes in between, like our job starts or finishes, we get modified, etc, we might
		reconcile again sooner.
	*/
	// we'll requeue once we see the running job, and update our status
	return scheduledResult, nil
}

func (r *ReconcileBroadcastCronJob) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.BroadcastCronJob{}).
		Owns(&appsv1alpha1.BroadcastJob{}).
		Complete(r)
}
