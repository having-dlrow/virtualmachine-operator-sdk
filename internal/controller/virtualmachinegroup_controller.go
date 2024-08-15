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

package controller

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	vmv1 "github.com/example/virtualmachine/api/v1"
)

// VirtualMachineGroupReconciler reconciles a VirtualMachineGroup object
type VirtualMachineGroupReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=vm.example.com,resources=virtualmachinegroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vm.example.com,resources=virtualmachinegroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vm.example.com,resources=virtualmachinegroups/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

func (r *VirtualMachineGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the VirtualMachineGroup instance
	vmg := &vmv1.VirtualMachineGroup{}
	err := r.Get(ctx, req.NamespacedName, vmg)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("VirtualMachineGroup resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get VirtualMachineGroup")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status is available
	if vmg.Status.Conditions == nil || len(vmg.Status.Conditions) == 0 {
		meta.SetStatusCondition(&vmg.Status.Conditions, metav1.Condition{Type: TypeAvailableVirtualMachine, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err := r.Status().Update(ctx, vmg); err != nil {
			log.Error(err, "Failed to update VirtualMachineGroup status")
			return ctrl.Result{}, err
		}
		// Let's re-fetch the VirtualMachine Custom Resource after updating the status
		if err := r.Get(ctx, req.NamespacedName, vmg); err != nil {
			log.Error(err, "Failed to re-fetch VirtualMachineGroup")
			return ctrl.Result{}, err
		}
	}

	// Let's add a finalizer.
	if !controllerutil.ContainsFinalizer(vmg, FinalizerNameVirtualMachine) {
		log.Info("Adding Finalizer for VirtualMachineGroup")
		if ok := controllerutil.AddFinalizer(vmg, FinalizerNameVirtualMachine); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Update(ctx, vmg); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Check if the VirtualMachine instance is marked to be deleted
	isVirtualMachineMarkedToBeDeleted := vmg.GetDeletionTimestamp() != nil
	if isVirtualMachineMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(vmg, FinalizerNameVirtualMachine) {
			log.Info("Performing Finalizer Operations for VirtualMachineGroup before delete CR")

			// Let's add here a status "Deleting" to reflect that this resource began its process to be terminated.
			meta.SetStatusCondition(&vmg.Status.Conditions, metav1.Condition{Type: TypeDeletingVirtualMachine,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", vmg.Name)})

			if err := r.Status().Update(ctx, vmg); err != nil {
				log.Error(err, "Failed to update VirtualMachineGroup status")
				return ctrl.Result{}, err
			}

			// API Call
			r.doFinalizerOperationsForVirtualMachineGroup(vmg)

			// Re-fetch the VirtualMachine Custom Resource before updating the status
			if err := r.Get(ctx, req.NamespacedName, vmg); err != nil {
				log.Error(err, "Failed to re-fetch VirtualMachine")
				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&vmg.Status.Conditions, metav1.Condition{Type: TypeDeletingVirtualMachine,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", vmg.Name)})

			if err := r.Status().Update(ctx, vmg); err != nil {
				log.Error(err, "Failed to update VirtualMachine status")
				return ctrl.Result{}, err
			}

			log.Info("Removing Finalizer for VirtualMachine after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(vmg, FinalizerNameVirtualMachine); !ok {
				log.Error(err, "Failed to remove finalizer for VirtualMachine")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, vmg); err != nil {
				log.Error(err, "Failed to remove finalizer for Memcached")
				return ctrl.Result{}, err
			}
		}
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: vmg.Name, Namespace: vmg.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep, err := r.deploymentForVirtualMachineGroup(vmg)
		if err != nil {
			log.Error(err, "Failed to define new Deployment resource for VirtualMachine")

			// The following implementation will update the status
			meta.SetStatusCondition(&vmg.Status.Conditions, metav1.Condition{Type: TypeAvailableVirtualMachine,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Deployment for the custom resource (%s): (%s)", vmg.Name, err)})

			if err := r.Status().Update(ctx, vmg); err != nil {
				log.Error(err, "Failed to update VirtualMachine status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new Deployment",
			"Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		if err = r.Create(ctx, dep); err != nil {
			log.Error(err, "Failed to create new Deployment",
				"Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// Assume the deployment exists, now fetch the Pods associated with it
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(vmg.Namespace),
		client.MatchingLabels(labelsForVirtualMachineGroup(vmg)),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list Pods")
		return ctrl.Result{}, err
	}

	// Update the VirtualMachineGroup status
	var vmStatuses []vmv1.VirtualMachineStatus
	for _, pod := range podList.Items {
		vmStatus := vmv1.VirtualMachineStatus{
			Hostname: pod.Spec.Hostname,
			UUID:     string(pod.UID),
			IsActive: pod.Status.Phase == corev1.PodRunning,
		}
		vmStatuses = append(vmStatuses, vmStatus) // 배열에 추가
	}

	vmg.Status.Replicas = int32(len(vmStatuses))
	vmg.Status.VirtualMachines = vmStatuses
	vmg.Status.DevAdmin = vmg.Spec.DevAdmin

	// Set condition status to reflect the current state
	if len(vmStatuses) == int(vmg.Spec.Replicas) {
		meta.SetStatusCondition(&vmg.Status.Conditions, metav1.Condition{
			Type:    "AllVMsActive",
			Status:  metav1.ConditionTrue,
			Reason:  "AllVirtualMachinesRunning",
			Message: "All Virtual Machines are active",
		})
	} else {
		meta.SetStatusCondition(&vmg.Status.Conditions, metav1.Condition{
			Type:    "AllVMsActive",
			Status:  metav1.ConditionFalse,
			Reason:  "VirtualMachinesNotReady",
			Message: "Some Virtual Machines are not active",
		})
	}

	if err := r.Status().Update(ctx, vmg); err != nil {
		log.Error(err, "Failed to update VirtualMachineGroup status")
		return ctrl.Result{}, err
	}

	size := vmg.Spec.Replicas
	if found.Spec.Replicas == nil || *found.Spec.Replicas != size {
		if found.Spec.Replicas == nil {
			found.Spec.Replicas = new(int32)
		}
		*found.Spec.Replicas = size
		if err = r.Update(ctx, found); err != nil {
			log.Error(err, "Failed to update Deployment",
				"Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)

			if err := r.Get(ctx, req.NamespacedName, vmg); err != nil {
				log.Error(err, "Failed to re-fetch memcached")
				return ctrl.Result{}, err
			}

			// The following implementation will update the status
			meta.SetStatusCondition(&vmg.Status.Conditions, metav1.Condition{Type: TypeAvailableVirtualMachine,
				Status: metav1.ConditionFalse, Reason: "Resizing",
				Message: fmt.Sprintf("Failed to update the size for the custom resource (%s): (%s)", vmg.Name, err)})

			if err := r.Status().Update(ctx, vmg); err != nil {
				log.Error(err, "Failed to update Memcached status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// The following implementation will update the status
	meta.SetStatusCondition(&vmg.Status.Conditions, metav1.Condition{Type: TypeAvailableVirtualMachine,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Deployment for custom resource (%s) with %d replicas created successfully", vmg.Name, size)})

	if err := r.Status().Update(ctx, vmg); err != nil {
		log.Error(err, "Failed to update Memcached status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *VirtualMachineGroupReconciler) deploymentForVirtualMachineGroup(vmg *vmv1.VirtualMachineGroup) (*appsv1.Deployment, error) {
	// Define labels for the deployment and its pods
	ls := labelsForVirtualMachineGroup(vmg)

	// Define the deployment
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmg.Name,
			Namespace: vmg.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &vmg.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Image:           vmg.Spec.Image,
						Name:            vmg.Name,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/bin/bash", "-c", "--"},
						Args:            []string{"while true; do sleep 30; done;"},
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             &[]bool{true}[0],
							RunAsUser:                &[]int64{1001}[0],
							AllowPrivilegeEscalation: &[]bool{false}[0],
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
					}},
				},
			},
		},
	}

	// Set the owner reference for the Deployment
	if err := ctrl.SetControllerReference(vmg, dep, r.Scheme); err != nil {
		return nil, err
	}

	return dep, nil
}

func labelsForVirtualMachineGroup(vmg *vmv1.VirtualMachineGroup) map[string]string {

	imageTag := strings.Split(vmg.Spec.Image, ":")[1]
	return map[string]string{"app.kubernetes.io/name": "v1.vmg",
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/app.name":   vmg.Name,
		"app.kubernetes.io/managed-by": "vmg.Controller",
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualMachineGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vmv1.VirtualMachineGroup{}).
		Complete(r)
}

func (r *VirtualMachineGroupReconciler) doFinalizerOperationsForVirtualMachineGroup(vmg *vmv1.VirtualMachineGroup) {

	//r.Recorder.Event(vmg, "Warning", "Deleting",
	//	fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
	//		vmg.Name,
	//		vmg.Namespace))
}
