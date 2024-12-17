/*
Copyright 2022.

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

package checkly

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/checkly/checkly-go-sdk"
	checklyv1alpha1 "github.com/checkly/checkly-operator/api/checkly/v1alpha1"
	external "github.com/checkly/checkly-operator/external/checkly"
)

// ApiCheckReconciler reconciles a ApiCheck object
type ApiCheckReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	ApiClient        checkly.Client
	ControllerDomain string
}

//+kubebuilder:rbac:groups=k8s.checklyhq.com,resources=apichecks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.checklyhq.com,resources=apichecks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=k8s.checklyhq.com,resources=apichecks/finalizers,verbs=update
//+kubebuilder:rbac:groups=k8s.checklyhq.com,resources=groups,verbs=get;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ApiCheck object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ApiCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	apiCheckFinalizer := fmt.Sprintf("%s/finalizer", r.ControllerDomain)
	logger.V(1).Info("Reconciler started")

	apiCheck := &checklyv1alpha1.ApiCheck{}

	// ////////////////////////////////
	// Delete Logic
	// ///////////////////////////////
	err := r.Get(ctx, req.NamespacedName, apiCheck)
	if err != nil {
		if errors.IsNotFound(err) {
			// The resource has been deleted
			logger.V(1).Info("Deleted", "checkly ID", apiCheck.Status.ID, "endpoint", apiCheck.Spec.Endpoint, "name", apiCheck.Name)
			return ctrl.Result{}, nil
		}
		// Error reading the object
		logger.Error(err, "can't read the object")
		return ctrl.Result{}, nil
	}

	if apiCheck.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(apiCheck, apiCheckFinalizer) {
			logger.V(1).Info("Finalizer is present, trying to delete Checkly check", "checkly ID", apiCheck.Status.ID)
			err := external.Delete(apiCheck.Status.ID, r.ApiClient)
			if err != nil {
				logger.Error(err, "Failed to delete checkly API check")
				return ctrl.Result{}, err
			}

			logger.Info("Successfully deleted checkly API check", "checkly ID", apiCheck.Status.ID)

			controllerutil.RemoveFinalizer(apiCheck, apiCheckFinalizer)
			err = r.Update(ctx, apiCheck)
			if err != nil {
				logger.Error(err, "Failed to delete finalizer")
				return ctrl.Result{}, err
			}
			logger.V(1).Info("Successfully deleted finalizer")
		}
		return ctrl.Result{}, nil
	}

	// Object found, let's do something with it. It's either updated, or it's new.
	logger.V(1).Info("Object found", "endpoint", apiCheck.Spec.Endpoint)

	// /////////////////////////////
	// Finalizer logic
	// ////////////////////////////
	if !controllerutil.ContainsFinalizer(apiCheck, apiCheckFinalizer) {
		controllerutil.AddFinalizer(apiCheck, apiCheckFinalizer)
		err = r.Update(ctx, apiCheck)
		if err != nil {
			logger.Error(err, "Failed to update ApiCheck status")
			return ctrl.Result{}, err
		}
		logger.V(1).Info("Added finalizer", "checkly ID", apiCheck.Status.ID, "endpoint", apiCheck.Spec.Endpoint)
		return ctrl.Result{}, nil
	}

	// /////////////////////////////
	// Lookup group ID
	// ////////////////////////////
	group := &checklyv1alpha1.Group{}
	err = r.Get(ctx, types.NamespacedName{Name: apiCheck.Spec.Group}, group)
	if err != nil {
		if errors.IsNotFound(err) {
			// The resource has been deleted
			logger.Error(err, "Group not found, probably deleted or does not exist", "name", apiCheck.Spec.Group)
			return ctrl.Result{}, err
		}
		// Error reading the object
		logger.Error(err, "can't read the group object")
		return ctrl.Result{}, err
	}

	if group.Status.ID == 0 {
		logger.V(1).Info("Group ID has not been populated, we're too quick, requeuing for retry", "group name", apiCheck.Spec.Group)
		return ctrl.Result{Requeue: true}, nil
	}

	// Create internal Check type
	internalCheck := external.Check{
		Name:            apiCheck.Name,
		Namespace:       apiCheck.Namespace,
		Frequency:       apiCheck.Spec.Frequency,
		MaxResponseTime: apiCheck.Spec.MaxResponseTime,
		Endpoint:        apiCheck.Spec.Endpoint,
		ID:              apiCheck.Status.ID,
		GroupID:         group.Status.ID,
		Muted:           apiCheck.Spec.Muted,
		Labels:          apiCheck.Labels,
		Assertions:      r.mapAssertions(apiCheck.Spec.Assertions),
		Method:          apiCheck.Spec.Method,
		Body:            apiCheck.Spec.Body,
		BodyType:        apiCheck.Spec.BodyType,
	}

	// /////////////////////////////
	// Update logic
	// ////////////////////////////

	// Determine if it's a new object or if it's an update to an existing object
	if apiCheck.Status.ID != "" {
		// Existing object, we need to update it
		logger.V(1).Info("Existing object, with ID", "checkly ID", apiCheck.Status.ID, "endpoint", apiCheck.Spec.Endpoint)
		err := external.Update(internalCheck, r.ApiClient)
		if err != nil {
			logger.Error(err, "Failed to update the checkly check")
			return ctrl.Result{}, err
		}
		logger.Info("Updated checkly check", "checkly ID", apiCheck.Status.ID)
		return ctrl.Result{}, nil
	}

	// /////////////////////////////
	// Create logic
	// ////////////////////////////

	checklyID, err := external.Create(internalCheck, r.ApiClient)
	if err != nil {
		logger.Error(err, "Failed to create checkly alert")
		return ctrl.Result{}, err
	}

	// Update the custom resource Status with the returned ID

	apiCheck.Status.ID = checklyID
	apiCheck.Status.GroupID = group.Status.ID
	err = r.Status().Update(ctx, apiCheck)
	if err != nil {
		logger.Error(err, "Failed to update ApiCheck status")
		return ctrl.Result{}, err
	}
	logger.V(1).Info("New checkly check created with", "checkly ID", apiCheck.Status.ID, "spec", apiCheck.Spec)

	return ctrl.Result{}, nil
}

// mapAssertions maps ApiCheck assertions to external.Check assertions
func (r *ApiCheckReconciler) mapAssertions(assertions []checklyv1alpha1.Assertion) []checkly.Assertion {
	var mapped []checkly.Assertion
	for _, assertion := range assertions {
		mapped = append(mapped, checkly.Assertion{
			Source:     assertion.Source,
			Property:   assertion.Property,
			Comparison: assertion.Comparison,
			Target:     assertion.Target,
		})
	}
	return mapped
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApiCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&checklyv1alpha1.ApiCheck{}).
		Complete(r)
}
