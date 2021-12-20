/*
Copyright 2021.

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

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/argoproj/gitops-engine/pkg/engine"

	packagesv1alpha1 "cattle.io/kevi/api/v1alpha1"
	"cattle.io/kevi/pkg/fetcher"
	"cattle.io/kevi/pkg/pack"
)

// KeviReconciler reconciles a Kevi object
type KeviReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Fetcher fetcher.Fetcher
	Engine  engine.GitOpsEngine
}

type GCMark struct {
	Mark string
}

const (
	GCAnnotationMark = "kevi.cattle.io/gc-mark"
)

// TODO: Extremely privileged b/c of gitopsengine's scope, should tone this down a notch
// +kubebuilder:rbac:groups=*,resources=*,verbs=*
// OLD: +kubebuilder:rbac:groups=packages.cattle.io,resources=kevis,verbs=get;list;watch;create;update;patch;delete
// OLD: +kubebuilder:rbac:groups=packages.cattle.io,resources=kevis/status,verbs=get;update;patch
// OLD: +kubebuilder:rbac:groups=packages.cattle.io,resources=kevis/finalizers,verbs=update

func (r *KeviReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var kevi packagesv1alpha1.Kevi
	if err := r.Get(ctx, req.NamespacedName, &kevi); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	for _, pkg := range kevi.Spec.Packages {
		l.Info("processing package", "pkg", pkg.Name)

		p, err := pack.Load(ctx, r.Fetcher, pkg)
		if err != nil {
			return ctrl.Result{}, err
		}

		data, err := p.Generate()
		if err != nil {
			return ctrl.Result{}, err
		}

		objs, err := kube.SplitYAML(data)
		if err != nil {
			return ctrl.Result{}, err
		}

		l.Info("Syncing package", "package", pkg.Name, "# objects", len(objs))
		if err := r.sync(ctx, objs); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeviReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&packagesv1alpha1.Kevi{}).
		Complete(r)
}

func (r *KeviReconciler) sync(ctx context.Context, objs []*unstructured.Unstructured) error {
	l := log.FromContext(ctx)

	result, err := r.Engine.Sync(ctx, objs, func(r *cache.Resource) bool {
		if r.Info != nil {
			return r.Info.(*GCMark).Mark == "donk"
		}
		return false
	}, "latest", "default", sync.WithPrune(true), sync.WithLogr(l))
	if err != nil {
		return err
	}

	_ = result
	return nil
}
