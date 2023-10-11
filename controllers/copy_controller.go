/*
Copyright 2023.

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

package controllers

import (
	"context"
	"reflect"

	v1apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	"github.com/penick/copy-operator/api/v1alpha1"
)

// CopyReconciler reconciles a Copy object
type CopyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cache.mezmo.com,resources=copies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cache.mezmo.com,resources=copies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cache.mezmo.com,resources=copies/finalizers,verbs=update

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Copy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *CopyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	copy := &v1alpha1.Copy{}
	err := r.Get(ctx, req.NamespacedName, copy)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.reconcileDeployment(ctx, log, copy)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CopyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Copy{}).
		Owns(&v1apps.Deployment{}).
		Watches(&source.Kind{Type: &v1apps.Deployment{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			log := ctrl.Log.WithName("watch")
			ctx := context.TODO()

			copies := v1alpha1.CopyList{}
			err := r.List(ctx, &copies)
			if err != nil {
				log.Error(err, "No matching copy resources for namespace")
				return nil
			}

			for _, copy := range copies.Items {
				log.Info("Checking if resource should be copied",
					"name", object.GetName(), "namespace", object.GetNamespace())
				if object.GetNamespace() == copy.Spec.SourceNamespace &&
					(!copy.Spec.UseLabels || hasLabel(object.GetLabels(), copy.Spec.TargetLabels)) {
					r.reconcileDeployment(ctx, log, &copy) // TODO: Return the error?
				}
			}
			return nil
		})).
		Complete(r)
}

func (r *CopyReconciler) reconcileDeployment(ctx context.Context, log logr.Logger, copy *v1alpha1.Copy) {
	srcDeploys := v1apps.DeploymentList{}
	err := r.List(ctx, &srcDeploys, &client.ListOptions{Namespace: copy.Spec.SourceNamespace})
	if err != nil {
		log.Error(err, "Unable to get source deployments")
		return
	}

	destDeploys := v1apps.DeploymentList{}
	err = r.List(ctx, &destDeploys, &client.ListOptions{Namespace: copy.GetNamespace()})
	if err != nil {
		log.Error(err, "Unable to get destination deployments")
		return
	}

	for _, srcDeploy := range srcDeploys.Items {
		found := false
		for _, destDeploy := range destDeploys.Items {
			if destDeploy.GetName() == srcDeploy.GetName() {
				found = true
				if !reflect.DeepEqual(destDeploy.Spec, srcDeploy.Spec) {
					destDeploy.Spec = srcDeploy.Spec
					err = r.Update(ctx, &destDeploy)
					if err != nil {
						log.Error(err, "Unable to update deployment", "name", destDeploy.GetName(), "namespace", destDeploy.GetNamespace())
					}
				}
			}
		}

		if !found {
			newDeploy := srcDeploy.DeepCopy()
			newDeploy.SetNamespace(copy.GetNamespace())
			newDeploy.SetResourceVersion("")
			err = ctrl.SetControllerReference(copy, newDeploy, r.Scheme)
			if err != nil {
				log.Error(err, "Unable to set controller reference", "name", newDeploy.GetName(), "namespace", newDeploy.GetNamespace())
				continue
			}
			err = r.Create(ctx, newDeploy)
			if err != nil {
				log.Error(err, "Unable to create deployment", "name", newDeploy.GetName(), "namespace", newDeploy.GetNamespace())
			}
		}
	}

	for _, destDeploy := range destDeploys.Items {
		found := false
		for _, srcDeploy := range srcDeploys.Items {
			if destDeploy.GetName() == srcDeploy.GetName() {
				found = true
				break
			}
		}
		if !found {
			err := r.Delete(ctx, &destDeploy)
			if err != nil {
				log.Error(err, "Unable to delete deployment", destDeploy.GetName(), "namespace", destDeploy.GetNamespace())
			}
		}
	}
}

func hasLabel(labels map[string]string, targetLabels []string) bool {
	if _, ok := labels["copyMe"]; ok {
		return true
	}
	for _, target := range targetLabels {
		if _, ok := labels[target]; ok {
			return true
		}
	}
	return false
}
