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

	v1apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

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
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CopyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Copy{}).
		Watches(&source.Kind{Type: &v1apps.Deployment{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			log := ctrl.Log.WithName("watch")

			deploy := object.(*v1apps.Deployment)

			copies := v1alpha1.CopyList{}
			err := r.List(context.TODO(), &copies)
			if err != nil {
				log.Error("No matching copy resources for namespace")
				return nil
			}
			for _, copy := range copies.Items {
				if deploy.Namespace == copy.Spec.SourceNamespace && hasLabel(deploy.GetLabels(), copy.Spec.TargetLabels) {
					// Do copy
					log.Info("We would have copied")
				}
			}
		})).
		Complete(r)
}

func hasLabel(labels map[string]string, targetLabels []string) bool {
	for _, target := range targetLabels {
		if _, ok := labels[target]; ok {
			return true
		}
	}
	return false
}
