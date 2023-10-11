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
	v1core "k8s.io/api/core/v1"
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

	r.reconcileAny(ctx, log, copy, &copyDeploy)
	r.reconcileAny(ctx, log, copy, &copyPod)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CopyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Copy{}).
		Owns(&v1apps.Deployment{}).
		Owns(&v1core.Pod{}).
		Watches(&source.Kind{Type: &v1apps.Deployment{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			return r.handleWatch(object, func(ctx context.Context, log logr.Logger, copy *v1alpha1.Copy) {
				r.reconcileAny(ctx, log, copy, &copyDeploy)
			})
		})).
		Watches(&source.Kind{Type: &v1core.Pod{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			return r.handleWatch(object, func(ctx context.Context, log logr.Logger, copy *v1alpha1.Copy) {
				r.reconcileAny(ctx, log, copy, &copyPod)
			})
		})).
		Complete(r)
}

func (r *CopyReconciler) handleWatch(object client.Object, reconcileFn func(context.Context, logr.Logger, *v1alpha1.Copy)) []reconcile.Request {
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
			reconcileFn(ctx, log, &copy)
		}
	}
	return nil

}

func (r *CopyReconciler) reconcileAny(ctx context.Context, log logr.Logger, copy *v1alpha1.Copy, copyResource CopyResource) {
	srcObjs, err := copyResource.GetList(ctx, r.Client, copy.Spec.SourceNamespace)
	if err != nil {
		log.Error(err, "Unable to get source resources")
		return
	}

	destObjs, err := copyResource.GetList(ctx, r.Client, copy.GetNamespace())
	if err != nil {
		log.Error(err, "Unable to get destination resources")
		return
	}

	for _, srcObj := range srcObjs {
		found := false
		for _, destObj := range destObjs {
			if destObj.GetName() == srcObj.GetName() {
				found = true
				if !copyResource.CompareSpec(destObj, srcObj) {
					copyResource.CopySpec(srcObj, destObj)
					err = r.Update(ctx, destObj)
					if err != nil {
						log.Error(err, "Unable to update deployment", "name", destObj.GetName(), "namespace", destObj.GetNamespace())
					}
				}
			}
		}

		if !found {
			newObj := srcObj.DeepCopyObject().(client.Object)
			newObj.SetNamespace(copy.GetNamespace())
			newObj.SetResourceVersion("")
			err = ctrl.SetControllerReference(copy, newObj, r.Scheme)
			if err != nil {
				log.Error(err, "Unable to set controller reference", "name", newObj.GetName(), "namespace", newObj.GetNamespace())
				continue
			}
			err = r.Create(ctx, newObj)
			if err != nil {
				log.Error(err, "Unable to create deployment", "name", newObj.GetName(), "namespace", newObj.GetNamespace())
			}
		}
	}

	for _, destObj := range destObjs {
		found := false
		for _, srcObj := range srcObjs {
			if destObj.GetName() == srcObj.GetName() {
				found = true
				break
			}
		}
		if !found {
			err := r.Delete(ctx, destObj)
			if err != nil {
				log.Error(err, "Unable to delete deployment", destObj.GetName(), "namespace", destObj.GetNamespace())
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
