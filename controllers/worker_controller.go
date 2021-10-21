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
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cloudasrv1alpha1 "github.com/smartcommunitylab/cloudasr-operator/api/v1alpha1"
)

// WorkerReconciler reconciles a Worker object
type WorkerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cloudasr.smartcommunitylab.it,namespace=cloudasr-operator-system,resources=workers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cloudasr.smartcommunitylab.it,namespace=cloudasr-operator-system,resources=workers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cloudasr.smartcommunitylab.it,namespace=cloudasr-operator-system,resources=workers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,namespace=cloudasr-operator-system,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,namespace=cloudasr-operator-system,resources=pods;services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Worker object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *WorkerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("worker", req.NamespacedName)

	// Fetch the Worker instance
	worker := &cloudasrv1alpha1.Worker{}
	err := r.Get(ctx, req.NamespacedName, worker)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Worker resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Worker")
	}

	// Check if the StatefulSet already exists, if not create a new one
	found := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: worker.Name, Namespace: worker.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new statefulset
		dep := r.statefulsetForWorker(worker)
		log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", dep.Namespace, "StatefulSet.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new StatefulSet", "StatefulSet.Namespace", dep.Namespace, "StatefulSet.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// StatefulSet created successfully - return and requeue
		return ctrl.Result{}, err
	} else if err != nil {
		log.Error(err, "Failed to get StatefulSet")
		return ctrl.Result{}, err
	}

	// Check if the Service already exists, if not create a new one
	svcfound := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: worker.Name, Namespace: worker.Namespace}, svcfound)
	if err != nil && errors.IsNotFound(err) {
		// Define a new service
		svc, _ := r.serviceForWorker(worker)
		log.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.Create(ctx, svc)
		if err != nil {
			log.Error(err, "Failed to create new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// Service created successfully - return and requeue
		return ctrl.Result{}, err
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	size := worker.Spec.Replicas
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		err = r.Update(ctx, found)
		if err != nil {
			log.Error(err, "Failed to update StatefulSet", "StatefulSet.Namespace", found.Namespace, "StatefulSet.Name", found.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// Update the Worker status with the pod name
	// List the pods for this worker's statefulset
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(worker.Namespace),
		client.MatchingLabels(labelsForWorker(worker.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Worker.Namespace", worker.Namespace, "Worker.Name", worker.Name)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Pods if neeeded
	if !reflect.DeepEqual(podNames, worker.Status.Pods) {
		worker.Status.Pods = podNames
		err := r.Status().Update(ctx, worker)
		if err != nil {
			log.Error(err, "Failed to update Worker status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *WorkerReconciler) serviceForWorker(m *cloudasrv1alpha1.Worker) (*corev1.Service, error) {
	ls := labelsForWorker(m.Name)
	svc := corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "tcp-worker", Port: 5678, Protocol: "TCP", TargetPort: intstr.FromString("tcp-worker")},
			},
			Selector:  ls,
			ClusterIP: "None",
			Type:      corev1.ServiceTypeClusterIP,
		},
	}

	// always set the controller reference so that we know which object owns this.
	if err := ctrl.SetControllerReference(m, &svc, r.Scheme); err != nil {
		return &svc, err
	}

	return &svc, nil
}
func (r *WorkerReconciler) statefulsetForWorker(m *cloudasrv1alpha1.Worker) *appsv1.StatefulSet {
	ls := labelsForWorker(m.Name)
	replicas := m.Spec.Replicas

	dep := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         m.Name,
			PodManagementPolicy: "Parallel",
			Replicas:            &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "cloudasr",
					ImagePullSecrets: []corev1.LocalObjectReference{{
						Name: "regcred"}},
					Containers: []corev1.Container{{
						Name:            "cloudasr-" + m.Name + "-kaldi",
						Image:           m.Spec.KalidImage,
						ImagePullPolicy: "IfNotPresent",
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu":    resource.MustParse("2000m"),
								"memory": resource.MustParse("4096Mi"),
							},
							Requests: corev1.ResourceList{
								"cpu":    resource.MustParse("1000m"),
								"memory": resource.MustParse("512Mi"),
							},
						},
					},
						{
							Name:            "cloudasr-" + m.Name + "-python",
							Image:           m.Spec.PythonImage,
							ImagePullPolicy: "IfNotPresent",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu":    resource.MustParse("2000"),
									"memory": resource.MustParse("512Mi"),
								},
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse("250m"),
									"memory": resource.MustParse("128Mi"),
								},
							},
							Ports: []corev1.ContainerPort{{
								ContainerPort: 5678,
								Name:          "tcp-worker",
							}},
							Env: []corev1.EnvVar{{
								Name:  "MASTER_ADDR",
								Value: "tcp://master:5678",
							},
								{
									Name:  "RECORDINGS_SAVER_ADDR",
									Value: "tcp://recordings:5682",
								},
								{
									Name:  "MODEL",
									Value: m.Spec.ModelName,
								},
								{
									Name:  "PORT0",
									Value: "5678",
								},
								{
									Name:  "SERVICE_NAME",
									Value: "worker-" + m.Spec.ModelName,
								},
								{
									Name:  "LOG_LEVEL",
									Value: "INFO",
								}},
						}},
				},
			},
		},
	}
	// Set Worker istance as the owner and controller
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// labelsForMemcached returns the labels for selecting the resources
// belonging to the given memcached CR name.
func labelsForWorker(name string) map[string]string {
	return map[string]string{"app": "cloudasr", "tier": "backend", "component": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cloudasrv1alpha1.Worker{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}
