/*
Copyright 2024.

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

package networking

import (
	"context"
	"fmt"
	"strings"

	checklyv1alpha1 "github.com/checkly/checkly-operator/api/checkly/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	ControllerDomain string
}

//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/finalizers,verbs=update
//+kubebuilder:rbac:groups=k8s.checklyhq.com,resources=apichecks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.checklyhq.com,resources=apichecks/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciler started")

	// ////////////////////////////////
	// Setup
	// ///////////////////////////////
	ingress := networkingv1.Ingress{}
	// apiCheck := &checklyv1alpha1.ApiCheck{}
	annotationEnabled := fmt.Sprintf("%s/enabled", r.ControllerDomain)
	checklyFinalizer := fmt.Sprintf("%s/finalizer", r.ControllerDomain)
	// Check if ingress object is still present
	err := r.Get(ctx, req.NamespacedName, &ingress)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Ingress got deleted")
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		logger.Error(err, "Can't read the Ingress object")
		return ctrl.Result{}, err
	}
	logger.Info("Ingress Object found")

	// Gather data for the checkly check
	logger.Info("Gathering data for the check")
	apiCheckResources, err := r.gatherApiCheckData(&ingress)
	if err != nil {
		logger.Info("unable to gather data for the apiCheck resource", "Ingress Name", ingress.Name, "Ingress namespace", ingress.Namespace)
		return ctrl.Result{}, err
	}

	// Do we want to do anything with the ingress?
	_, checklyAnnotationExists := ingress.Annotations[annotationEnabled]
	if (!checklyAnnotationExists) || (ingress.Annotations[annotationEnabled] == "false") {
		logger.Info("Checking to see if we need to delete any resources as we're not handling this ingress.", "Ingress Name", ingress.Name, "Ingress namespace", ingress.Namespace)

		r.deleteIngressApiChecks(ctx, req, apiCheckResources, ingress)

		if ingress.GetDeletionTimestamp() == nil {
			return ctrl.Result{}, nil
		}
	}
	// ////////////////////////////////
	// Delete Logic
	// ///////////////////////////////

	if ingress.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(&ingress, checklyFinalizer) {
			logger.Info("Finalizer present, need to delete ApiCheck first.", "Ingress Name", ingress.Name, "Ingress namespace", ingress.Namespace)

			r.deleteIngressApiChecks(ctx, req, apiCheckResources, ingress)

			// Delete finalizer logic
			logger.Info("Deleting finalizer", "Ingress Name", ingress.Name, "Ingress namespace", ingress.Namespace)
			controllerutil.RemoveFinalizer(&ingress, checklyFinalizer)
			err = r.Update(ctx, &ingress)
			if err != nil {
				logger.Error(err, "Failed to delete finalizer", "Ingress Name", ingress.Name, "Ingress namespace", ingress.Namespace)
				return ctrl.Result{}, err
			}
			logger.Info("Successfully deleted finalizer", "Ingress Name", ingress.Name, "Ingress namespace", ingress.Namespace)
			return ctrl.Result{}, nil
		}
	}

	// /////////////////////////////
	// Finalizer logic
	// ////////////////////////////
	if !controllerutil.ContainsFinalizer(&ingress, checklyFinalizer) {
		controllerutil.AddFinalizer(&ingress, checklyFinalizer)
		err = r.Update(ctx, &ingress)
		if err != nil {
			logger.Error(err, "Failed to update ingress finalizer")
			return ctrl.Result{}, err
		}
		logger.Info("Added finalizer", "ingress", ingress.Name, "Ingress namespace", ingress.Namespace)
		return ctrl.Result{}, nil
	}

	// /////////////////////////////
	// Update/Create logic
	// ////////////////////////////

	newApiChecks, deleteApiChecks, updateApiChecks, err := r.compareApiChecks(ctx, &ingress, apiCheckResources)
	if err != nil {
		logger.Error(err, "Failed to list existing API checks")
		return ctrl.Result{}, err
	}

	// Create new Api Checks
	if len(newApiChecks) > 0 {
		for _, apiCheck := range newApiChecks {
			logger.Info("Creating ApiCheck", "ApiCheck Name:", apiCheck.Name, "ApiCheck spec:", apiCheck.Spec)
			err = r.Create(ctx, apiCheck)
			if err != nil {
				logger.Error(err, "Failed to create ApiCheck", "APICheck name:", apiCheck.Name, "Namespace:", apiCheck.Namespace, "Ingress name:", ingress.Name, "Ingress namespace", ingress.Namespace)
				return ctrl.Result{}, err
			}
		}
	}

	// Update API checks
	if len(updateApiChecks) > 0 {
		for _, apiCheck := range updateApiChecks {
			logger.Info("Updating ApiCheck", "ApiCheck Name:", apiCheck.Name)
			err = r.Update(ctx, apiCheck)
			if err != nil {
				logger.Error(err, "Failed to update APICheck resource", "APICheck name:", apiCheck.Name, "Namespace:", apiCheck.Namespace, "Ingress name:", ingress.Name, "Ingress namespace", ingress.Namespace)
				return ctrl.Result{}, err
			}
		}
	}

	// Delete old API checks
	if len(deleteApiChecks) > 0 {
		for _, apiCheck := range deleteApiChecks {

			logger.Info("Delete ApiCheck", "ApiCheck Name:", apiCheck.Name)
			err = r.Delete(ctx, apiCheck)
			if err != nil {
				logger.Error(err, "Failed to delete ApiCheck resource", "APICheck name:", apiCheck.Name, "Namespace:", apiCheck.Namespace, "Ingress name:", ingress.Name, "Ingress namespace", ingress.Namespace)
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Complete(r)
}

func (r *IngressReconciler) gatherApiCheckData(ingress *networkingv1.Ingress) (apiChecks []*checklyv1alpha1.ApiCheck, err error) {

	annotationHost := r.ControllerDomain
	annotationPath := fmt.Sprintf("%s/path", annotationHost)
	annotationEndpoint := fmt.Sprintf("%s/endpoint", annotationHost)
	annotationSuccess := fmt.Sprintf("%s/success", annotationHost)
	annotationGroup := fmt.Sprintf("%s/group", annotationHost)
	annotationMuted := fmt.Sprintf("%s/muted", annotationHost)

	// Expected success code
	var success string
	if ingress.Annotations[annotationSuccess] != "" {
		success = ingress.Annotations[annotationSuccess]
	} else {
		success = "200"
	}

	// Group
	var group string
	if ingress.Annotations[annotationGroup] != "" {
		group = ingress.Annotations[annotationGroup]
	} else {
		err = fmt.Errorf("could not find a value for the group annotation, can't continue without one")
	}

	// Muted
	var muted bool
	if ingress.Annotations[annotationMuted] == "false" {
		muted = false
	} else {
		muted = true
	}

	labels := make(map[string]string)
	labels["ingress-controller"] = ingress.Name

	// Get the host(s) and path(s) from the ingress object
	// No Rules specified, nothing to do
	if len(ingress.Spec.Rules) == 0 {
		return
	}

	for _, rule := range ingress.Spec.Rules {

		// Get the host
		var host string
		if ingress.Annotations[annotationEndpoint] == "" {
			host = rule.Host
		} else {
			host = ingress.Annotations[annotationEndpoint]
		}

		// Get the path(s)
		var paths []string

		if rule.HTTP == nil { // HTTP may not exist
			paths = append(paths, "/")
		} else if rule.HTTP.Paths == nil { // Paths may not exist
			paths = append(paths, "/")
		} else {
			for _, rulePath := range rule.HTTP.Paths {
				if ingress.Annotations[annotationPath] == "" {
					if rulePath.Path == "" {
						paths = append(paths, "/")
					} else {
						paths = append(paths, rulePath.Path)
					}
				} else {
					paths = append(paths, ingress.Annotations[annotationPath])
				}
			}
		}

		for _, path := range paths {
			// Replace path /
			path = strings.TrimPrefix(path, "/")

			// Set apiCheck Name
			checkName := fmt.Sprintf("%s-%s-%s", ingress.Name, host, path)
			checkName = strings.Replace(checkName, "/", "", -1)
			checkName = strings.Replace(checkName, ".", "", -1)
			checkName = strings.Trim(checkName, "-")

			// Set endpoint
			endpoint := fmt.Sprintf("https://%s/%s", host, path)

			// Construct ApiCheck Spec
			apiCheckSpec := &checklyv1alpha1.ApiCheckSpec{
				Endpoint: endpoint,
				Group:    group,
				Success:  success,
				Muted:    muted,
			}

			newApiCheck := &checklyv1alpha1.ApiCheck{
				ObjectMeta: metav1.ObjectMeta{
					Name:      checkName,
					Namespace: ingress.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(ingress, networkingv1.SchemeGroupVersion.WithKind("ingress")),
					},
					Labels: labels,
				},
				Spec: *apiCheckSpec,
			}

			apiChecks = append(apiChecks, newApiCheck)
		}
	}

	// Last return
	return
}

func (r *IngressReconciler) compareApiChecks(ctx context.Context, ingress *networkingv1.Ingress, ingressApiChecks []*checklyv1alpha1.ApiCheck) (newApiChecks []*checklyv1alpha1.ApiCheck, deleteApiChecks []*checklyv1alpha1.ApiCheck, updateApiChecks []*checklyv1alpha1.ApiCheck, err error) {

	logger := log.FromContext(ctx)

	var existingApiChecks checklyv1alpha1.ApiCheckList
	labels := make(map[string]*string)
	labels["ingress-controller"] = &ingress.Name
	err = r.List(ctx, &existingApiChecks, client.InNamespace(ingress.Namespace), client.MatchingLabels{"ingress-controller": ingress.Name})
	if err != nil {
		return
	}

	existingApiChecksMap := make(map[string]checklyv1alpha1.ApiCheck)
	for _, existingApiCheck := range existingApiChecks.Items {
		existingApiChecksMap[existingApiCheck.Name] = existingApiCheck
	}

	newApiChecksMap := make(map[string]*checklyv1alpha1.ApiCheck)
	for _, ingressApiCheck := range ingressApiChecks {
		newApiChecksMap[ingressApiCheck.Name] = ingressApiCheck
	}

	// Compare items
	for _, existingApiCheck := range existingApiChecksMap {
		_, exists := newApiChecksMap[existingApiCheck.Name]
		if exists {
			if existingApiCheck.Spec == newApiChecksMap[existingApiCheck.Name].Spec {
				logger.Info("ApiCheck data is identical, no need for update", "ApiCheck Name", existingApiCheck.Name)
			} else {
				logger.Info("ApiCheck data is not identical, update needed", "ApiCheck Name", existingApiCheck.Name, "old spec", existingApiCheck.Spec, "new spec", newApiChecksMap[existingApiCheck.Name].Spec)
				updateApiChecks = append(updateApiChecks, newApiChecksMap[existingApiCheck.Name])
			}

			// Remove items from new api checks map
			delete(newApiChecksMap, existingApiCheck.Name)
		} else {
			logger.Info("ApiCheck is not needed anymore, delete.", "ApiCheck Name", existingApiCheck.Name)
			deleteApiChecks = append(deleteApiChecks, &existingApiCheck)
		}
	}

	// Loop over remaining items and add them to the new checks list, these will be created
	for _, newApiCheck := range newApiChecksMap {
		newApiChecks = append(newApiChecks, newApiCheck)
	}

	return
}

func (r *IngressReconciler) deleteIngressApiChecks(ctx context.Context, req ctrl.Request, apiCheckResources []*checklyv1alpha1.ApiCheck, ingress networkingv1.Ingress) {

	logger := log.FromContext(ctx)

	for _, apiCheckResource := range apiCheckResources {

		logger.Info("Checking if ApiCheck was created", "Ingress Name", ingress.Name, "Ingress namespace", ingress.Namespace)
		err := r.Get(ctx, req.NamespacedName, apiCheckResource)
		if err != nil {
			logger.Info("ApiCheck resource is not present, we don't need to do anything.", "Ingress Name", ingress.Name, "Ingress namespace", ingress.Namespace)
			continue
		}

		logger.Info("ApiCheck resource is present, we need to delete it..", "Ingress Name", ingress.Name, "Ingress namespace", ingress.Namespace)
		err = r.Delete(ctx, apiCheckResource)
		if err != nil {
			logger.Error(err, "Failed to delete ApiCheck", "Name:", apiCheckResource.Name, "Namespace:", apiCheckResource.Namespace)
			continue
		}

		logger.Info("ApiCheck resource deleted successfully.", apiCheckResource.Name, "Namespace:", apiCheckResource.Namespace)
	}
}
