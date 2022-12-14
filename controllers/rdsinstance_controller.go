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

package controllers

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	dbaasv1alpha1 "github.com/RHEcosystemAppEng/dbaas-operator/api/v1alpha1"
	rdsdbaasv1alpha1 "github.com/RHEcosystemAppEng/rds-dbaas-operator/api/v1alpha1"
	rdsv1alpha1 "github.com/aws-controllers-k8s/rds-controller/apis/v1alpha1"
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ophandler "github.com/operator-framework/operator-lib/handler"
)

const (
	rdsInstanceType = "RDSInstance.dbaas.redhat.com"
	rdsInstanceKind = "DBInstance"

	instanceType = "instance"

	instanceFinalizer = "rds.dbaas.redhat.com/instance"

	engine              = "Engine"
	engineVersion       = "EngineVersion"
	dbInstanceClass     = "DBInstanceClass"
	storageType         = "StorageType"
	allocatedStorage    = "AllocatedStorage"
	iops                = "IOPS"
	maxAllocatedStorage = "MaxAllocatedStorage"
	dbSubnetGroupName   = "DBSubnetGroupName"
	publiclyAccessible  = "PubliclyAccessible"
	vpcSecurityGroupIDs = "VPCSecurityGroupIDs"
	licenseModel        = "LicenseModel"

	defaultDBInstanceClass    = "db.t3.micro"
	defaultAllocatedStorage   = 20
	defaultPubliclyAccessible = true
	defaultAvailabilityZone   = "us-east-1a"

	instanceConditionReady = "ProvisionReady"

	instanceStatusReasonReady        = "Ready"
	instanceStatusReasonCreating     = "Creating"
	instanceStatusReasonUpdating     = "Updating"
	instanceStatusReasonDeleting     = "Deleting"
	instanceStatusReasonTerminated   = "Terminated"
	instanceStatusReasonInputError   = "InputError"
	instanceStatusReasonBackendError = "BackendError"
	instanceStatusReasonNotFound     = "NotFound"
	instanceStatusReasonUnreachable  = "Unreachable"

	instanceStatusReasonDBInstance = "DBInstance"

	instanceStatusMessageUpdateError         = "Failed to update Instance"
	instanceStatusMessageCreating            = "Creating Instance"
	instanceStatusMessageUpdating            = "Updating Instance"
	instanceStatusMessageDeleting            = "Deleting Instance"
	instanceStatusMessageError               = "Instance with error"
	instanceStatusMessageCreateOrUpdateError = "Failed to create or update DB Instance"
	instanceStatusMessageGetError            = "Failed to get DB Instance"
	instanceStatusMessageDeleteError         = "Failed to delete DB Instance"
	instanceStatusMessageInventoryNotFound   = "Inventory not found"
	instanceStatusMessageInventoryNotReady   = "Inventory not ready"
	instanceStatusMessageGetInventoryError   = "Failed to get Inventory"

	requiredParameterErrorTemplate = "required parameter %s is missing"
	invalidParameterErrorTemplate  = "value of parameter %s is invalid"
)

// RDSInstanceReconciler reconciles a RDSInstance object
type RDSInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsinstances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsinstances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsinstances/finalizers,verbs=update
//+kubebuilder:rbac:groups=rds.services.k8s.aws,resources=dbinstances,verbs=get;list;watch;create;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *RDSInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	var inventory rdsdbaasv1alpha1.RDSInventory
	var instance rdsdbaasv1alpha1.RDSInstance

	var provisionStatus, provisionStatusReason, provisionStatusMessage string
	var phase dbaasv1alpha1.DBaasInstancePhase

	returnUpdating := func() {
		result = ctrl.Result{Requeue: true}
		err = nil
		provisionStatus = string(metav1.ConditionUnknown)
		provisionStatusReason = instanceStatusReasonUpdating
		provisionStatusMessage = instanceStatusMessageUpdating
	}

	returnError := func(e error, reason, message string) {
		result = ctrl.Result{}
		err = e
		provisionStatus = string(metav1.ConditionFalse)
		if len(reason) > 0 {
			provisionStatusReason = reason
		}
		if len(provisionStatusMessage) > 0 {
			provisionStatusMessage = fmt.Sprintf("%s: %s", message, provisionStatusMessage)
		} else {
			provisionStatusMessage = message
		}
	}

	returnNotReady := func(reason, message string) {
		result = ctrl.Result{}
		err = nil
		provisionStatus = string(metav1.ConditionFalse)
		provisionStatusReason = reason
		provisionStatusMessage = message
	}

	returnRequeue := func(reason, message string) {
		result = ctrl.Result{Requeue: true}
		err = nil
		provisionStatus = string(metav1.ConditionFalse)
		provisionStatusReason = reason
		provisionStatusMessage = message
	}

	returnReady := func() {
		result = ctrl.Result{}
		err = nil
		provisionStatus = string(metav1.ConditionTrue)
		provisionStatusReason = instanceStatusReasonReady
	}

	updateInstanceReadyCondition := func() {
		condition := metav1.Condition{
			Type:    instanceConditionReady,
			Status:  metav1.ConditionStatus(provisionStatus),
			Reason:  provisionStatusReason,
			Message: provisionStatusMessage,
		}
		apimeta.SetStatusCondition(&instance.Status.Conditions, condition)
		if len(phase) > 0 {
			instance.Status.Phase = phase
		} else if len(instance.Status.Phase) == 0 {
			instance.Status.Phase = dbaasv1alpha1.InstancePhaseUnknown
		}
		if e := r.Status().Update(ctx, &instance); e != nil {
			if errors.IsConflict(e) {
				logger.Info("Instance modified, retry reconciling")
				result = ctrl.Result{Requeue: true}
			} else {
				logger.Error(e, "Failed to update Instance status")
				if err == nil {
					err = e
				}
			}
		}
	}

	checkFinalizer := func() bool {
		if instance.ObjectMeta.DeletionTimestamp.IsZero() {
			if !controllerutil.ContainsFinalizer(&instance, instanceFinalizer) {
				phase = dbaasv1alpha1.InstancePhasePending
				controllerutil.AddFinalizer(&instance, instanceFinalizer)
				if e := r.Update(ctx, &instance); e != nil {
					if errors.IsConflict(e) {
						logger.Info("Instance modified, retry reconciling")
						returnUpdating()
						return true
					}
					logger.Error(e, "Failed to add finalizer to Instance")
					returnError(e, instanceStatusReasonBackendError, instanceStatusMessageUpdateError)
					return true
				}
				logger.Info("Finalizer added to Instance")
				returnNotReady(instanceStatusReasonUpdating, instanceStatusMessageUpdating)
				return true
			}
		} else {
			if controllerutil.ContainsFinalizer(&instance, instanceFinalizer) {
				phase = dbaasv1alpha1.InstancePhaseDeleting
				dbInstance := &rdsv1alpha1.DBInstance{}
				if e := r.Get(ctx, client.ObjectKey{Namespace: instance.Spec.InventoryRef.Namespace, Name: instance.Name}, dbInstance); e != nil {
					if !errors.IsNotFound(e) {
						logger.Error(e, "Failed to get DB Instance status")
						returnError(e, instanceStatusReasonBackendError, instanceStatusMessageGetError)
						return true
					}
				} else {
					if e := r.Delete(ctx, dbInstance); e != nil {
						logger.Error(e, "Failed to delete DB Instance")
						returnError(e, instanceStatusReasonBackendError, instanceStatusMessageDeleteError)
						return true
					}
					returnUpdating()
					return true
				}

				controllerutil.RemoveFinalizer(&instance, instanceFinalizer)
				if e := r.Update(ctx, &instance); e != nil {
					if errors.IsConflict(e) {
						logger.Info("Instance modified, retry reconciling")
						returnUpdating()
						return true
					}
					logger.Error(e, "Failed to remove finalizer from Instance")
					returnError(e, instanceStatusReasonBackendError, instanceStatusMessageUpdateError)
					return true
				}
				logger.Info("Finalizer removed from Instance")
				phase = dbaasv1alpha1.InstancePhaseDeleted
				returnNotReady(instanceStatusReasonUpdating, instanceStatusMessageUpdating)
				return true
			}

			// Stop reconciliation as the item is being deleted
			returnNotReady(instanceStatusReasonDeleting, instanceStatusMessageDeleting)
			return true
		}

		return false
	}

	createOrUpdateDBInstance := func() bool {
		dbInstance := &rdsv1alpha1.DBInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Name,
				Namespace: inventory.Namespace,
			},
		}

		if r, e := controllerutil.CreateOrUpdate(ctx, r.Client, dbInstance, func() error {
			if e := ophandler.SetOwnerAnnotations(&instance, dbInstance); e != nil {
				logger.Error(e, "Failed to set owner for DB Instance")
				returnError(e, instanceStatusReasonBackendError, e.Error())
				return e
			}
			if e := r.setDBInstanceSpec(ctx, dbInstance, &instance); e != nil {
				logger.Error(e, "Failed to set spec for DB Instance")
				returnError(e, instanceStatusReasonInputError, e.Error())
				return e
			}
			return nil
		}); e != nil {
			logger.Error(e, "Failed to create or update DB Instance")
			returnError(e, "", instanceStatusMessageCreateOrUpdateError)
			return true
		} else if r == controllerutil.OperationResultCreated {
			phase = dbaasv1alpha1.InstancePhaseCreating
			returnRequeue(instanceStatusReasonCreating, instanceStatusMessageCreating)
			return true
		} else if r == controllerutil.OperationResultUpdated {
			phase = dbaasv1alpha1.InstancePhaseUpdating
		}
		return false
	}

	syncDBInstanceStatus := func() bool {
		dbInstance := &rdsv1alpha1.DBInstance{}
		if e := r.Get(ctx, client.ObjectKey{Namespace: inventory.Namespace, Name: instance.Name}, dbInstance); e != nil {
			logger.Error(e, "Failed to get DB Instance status")
			if errors.IsNotFound(e) {
				returnError(e, instanceStatusReasonNotFound, instanceStatusMessageGetError)
			} else {
				returnError(e, instanceStatusReasonBackendError, instanceStatusMessageGetError)
			}
			return true
		}

		instance.Status.InstanceID = *dbInstance.Spec.DBInstanceIdentifier
		setDBInstancePhase(dbInstance, &instance)
		setDBInstanceStatus(dbInstance, &instance)
		regex := regexp.MustCompile("^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$")
		for _, condition := range dbInstance.Status.Conditions {
			c := metav1.Condition{
				Type:   string(condition.Type),
				Status: metav1.ConditionStatus(condition.Status),
			}
			if condition.LastTransitionTime != nil {
				c.LastTransitionTime = metav1.Time{Time: condition.LastTransitionTime.Time}
			}
			if condition.Reason != nil && len(*condition.Reason) > 0 {
				if match := regex.MatchString(*condition.Reason); match {
					c.Reason = *condition.Reason
					if condition.Message != nil {
						c.Message = *condition.Message
					}
				} else {
					c.Reason = instanceStatusReasonDBInstance
					if condition.Message != nil {
						c.Message = fmt.Sprintf("Reason: %s, Message: %s", *condition.Reason, *condition.Message)
					} else {
						c.Message = fmt.Sprintf("Reason: %s", *condition.Reason)
					}
				}
			} else {
				c.Reason = instanceStatusReasonDBInstance
				if condition.Message != nil {
					c.Message = *condition.Message
				}
			}
			apimeta.SetStatusCondition(&instance.Status.Conditions, c)
		}

		if e := r.Status().Update(ctx, &instance); e != nil {
			if errors.IsConflict(e) {
				logger.Info("Instance modified, retry reconciling")
				returnUpdating()
				return true
			}
			logger.Error(e, "Failed to sync Instance status")
			returnError(e, instanceStatusReasonBackendError, instanceStatusMessageUpdateError)
			return true
		}
		return false
	}

	if err = r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("RDS Instance resource not found, has been deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get RDS Instance")
		return ctrl.Result{}, err
	}

	defer updateInstanceReadyCondition()

	if checkFinalizer() {
		return
	}

	if e := r.Get(ctx, client.ObjectKey{Namespace: instance.Spec.InventoryRef.Namespace,
		Name: instance.Spec.InventoryRef.Name}, &inventory); e != nil {
		if errors.IsNotFound(e) {
			logger.Info("RDS Inventory resource not found, may have been deleted")
			returnError(e, instanceStatusReasonNotFound, instanceStatusMessageInventoryNotFound)
			return
		}
		logger.Error(e, "Failed to get RDS Inventory")
		returnError(e, instanceStatusReasonBackendError, instanceStatusMessageGetInventoryError)
		return
	}

	if condition := apimeta.FindStatusCondition(inventory.Status.Conditions, inventoryConditionReady); condition == nil ||
		condition.Status != metav1.ConditionTrue {
		logger.Info("RDS Inventory not ready")
		returnRequeue(instanceStatusReasonUnreachable, instanceStatusMessageInventoryNotReady)
		return
	}

	if createOrUpdateDBInstance() {
		return
	}

	if syncDBInstanceStatus() {
		return
	}

	switch instance.Status.Phase {
	case dbaasv1alpha1.InstancePhaseReady:
		returnReady()
	case dbaasv1alpha1.InstancePhaseFailed, dbaasv1alpha1.InstancePhaseDeleted:
		returnNotReady(instanceStatusReasonTerminated, string(instance.Status.Phase))
	case dbaasv1alpha1.InstancePhasePending, dbaasv1alpha1.InstancePhaseCreating,
		dbaasv1alpha1.InstancePhaseUpdating, dbaasv1alpha1.InstancePhaseDeleting:
		returnUpdating()
	case dbaasv1alpha1.InstancePhaseError, dbaasv1alpha1.InstancePhaseUnknown:
		returnRequeue(instanceStatusReasonBackendError, instanceStatusMessageError)
	default:
	}

	return
}

func (r *RDSInstanceReconciler) setDBInstanceSpec(ctx context.Context, dbInstance *rdsv1alpha1.DBInstance, rdsInstance *rdsdbaasv1alpha1.RDSInstance) error {
	if len(rdsInstance.Spec.CloudRegion) > 0 {
		dbInstance.Spec.AvailabilityZone = pointer.String(rdsInstance.Spec.CloudRegion)
	} else {
		dbInstance.Spec.AvailabilityZone = pointer.String(defaultAvailabilityZone)
	}

	if engine, ok := rdsInstance.Spec.OtherInstanceParams[engine]; ok {
		dbInstance.Spec.Engine = pointer.String(engine)
	} else {
		return fmt.Errorf(requiredParameterErrorTemplate, "Engine")
	}

	if engineVersion, ok := rdsInstance.Spec.OtherInstanceParams[engineVersion]; ok {
		dbInstance.Spec.EngineVersion = pointer.String(engineVersion)
	}

	if dbInstance.Spec.DBInstanceIdentifier == nil {
		dbInstance.Spec.DBInstanceIdentifier = pointer.String(fmt.Sprintf("rhoda-%s%s", getDBEngineAbbreviation(dbInstance.Spec.Engine), string(uuid.NewUUID())))
	}

	if dbInstanceClass, ok := rdsInstance.Spec.OtherInstanceParams[dbInstanceClass]; ok {
		dbInstance.Spec.DBInstanceClass = pointer.String(dbInstanceClass)
	} else {
		dbInstance.Spec.DBInstanceClass = pointer.String(defaultDBInstanceClass)
	}

	if storageType, ok := rdsInstance.Spec.OtherInstanceParams[storageType]; ok {
		dbInstance.Spec.StorageType = pointer.String(storageType)
	}

	if allocatedStorage, ok := rdsInstance.Spec.OtherInstanceParams[allocatedStorage]; ok {
		if i, e := strconv.ParseInt(allocatedStorage, 10, 64); e != nil {
			return fmt.Errorf(invalidParameterErrorTemplate, "AllocatedStorage")
		} else {
			dbInstance.Spec.AllocatedStorage = pointer.Int64(i)
		}
	} else {
		dbInstance.Spec.AllocatedStorage = pointer.Int64(defaultAllocatedStorage)
	}

	if iops, ok := rdsInstance.Spec.OtherInstanceParams[iops]; ok {
		if i, e := strconv.ParseInt(iops, 10, 64); e != nil {
			return fmt.Errorf(invalidParameterErrorTemplate, "IOPS")
		} else {
			dbInstance.Spec.IOPS = pointer.Int64(i)
		}
	}

	if maxAllocatedStorage, ok := rdsInstance.Spec.OtherInstanceParams[maxAllocatedStorage]; ok {
		if i, e := strconv.ParseInt(maxAllocatedStorage, 10, 64); e != nil {
			return fmt.Errorf(invalidParameterErrorTemplate, "MaxAllocatedStorage")
		} else {
			dbInstance.Spec.MaxAllocatedStorage = pointer.Int64(i)
		}
	}

	if dbSubnetGroupName, ok := rdsInstance.Spec.OtherInstanceParams[dbSubnetGroupName]; ok {
		dbInstance.Spec.DBSubnetGroupName = pointer.String(dbSubnetGroupName)
	}

	if publiclyAccessible, ok := rdsInstance.Spec.OtherInstanceParams[publiclyAccessible]; ok {
		if b, e := strconv.ParseBool(publiclyAccessible); e != nil {
			return fmt.Errorf(invalidParameterErrorTemplate, "PubliclyAccessible")
		} else {
			dbInstance.Spec.PubliclyAccessible = pointer.Bool(b)
		}
	} else {
		dbInstance.Spec.PubliclyAccessible = pointer.Bool(defaultPubliclyAccessible)
	}

	if vpcSecurityGroupIDs, ok := rdsInstance.Spec.OtherInstanceParams[vpcSecurityGroupIDs]; ok {
		sl := strings.Split(vpcSecurityGroupIDs, ",")
		var sgs []*string
		for _, s := range sl {
			st := s
			sgs = append(sgs, pointer.String(st))
		}
		dbInstance.Spec.VPCSecurityGroupIDs = sgs
	}

	if licenseModel, ok := rdsInstance.Spec.OtherInstanceParams[licenseModel]; ok {
		dbInstance.Spec.LicenseModel = pointer.String(licenseModel)
	}

	if _, e := setCredentials(ctx, r.Client, r.Scheme, dbInstance.GetName(), rdsInstance.Namespace, rdsInstance, rdsInstance.Kind,
		func(secretName string) {
			if dbInstance.Spec.MasterUsername == nil {
				dbInstance.Spec.MasterUsername = pointer.String(generateUsername(*dbInstance.Spec.Engine))
			}

			dbInstance.Spec.MasterUserPassword = &ackv1alpha1.SecretKeyReference{
				SecretReference: v1.SecretReference{
					Name:      secretName,
					Namespace: rdsInstance.Namespace,
				},
				Key: "password",
			}
		}); e != nil {
		return fmt.Errorf("failed to set credentials for DB instance")
	}

	dbName := generateDBName(*dbInstance.Spec.Engine)
	dbInstance.Spec.DBName = dbName

	return nil
}

func setDBInstancePhase(dbInstance *rdsv1alpha1.DBInstance, rdsInstance *rdsdbaasv1alpha1.RDSInstance) {
	var status string
	if dbInstance.Status.DBInstanceStatus != nil {
		status = *dbInstance.Status.DBInstanceStatus
	} else {
		status = ""
	}
	switch status {
	case "available":
		rdsInstance.Status.Phase = dbaasv1alpha1.InstancePhaseReady
	case "creating":
		rdsInstance.Status.Phase = dbaasv1alpha1.InstancePhaseCreating
	case "deleting":
		rdsInstance.Status.Phase = dbaasv1alpha1.InstancePhaseDeleting
	case "failed":
		rdsInstance.Status.Phase = dbaasv1alpha1.InstancePhaseFailed
	case "inaccessible-encryption-credentials-recoverable", "incompatible-parameters", "restore-error":
		rdsInstance.Status.Phase = dbaasv1alpha1.InstancePhaseError
	case "backing-up", "configuring-enhanced-monitoring", "configuring-iam-database-auth", "configuring-log-exports",
		"converting-to-vpc", "maintenance", "modifying", "moving-to-vpc", "rebooting", "resetting-master-credentials",
		"renaming", "starting", "stopping", "storage-optimization", "upgrading":
		rdsInstance.Status.Phase = dbaasv1alpha1.InstancePhaseUpdating
	case "inaccessible-encryption-credentials", "incompatible-network", "incompatible-option-group", "incompatible-restore",
		"insufficient-capacity", "stopped", "storage-full":
		rdsInstance.Status.Phase = dbaasv1alpha1.InstancePhaseUnknown
	default:
		rdsInstance.Status.Phase = dbaasv1alpha1.InstancePhaseUnknown
	}
}

func setDBInstanceStatus(dbInstance *rdsv1alpha1.DBInstance, rdsInstance *rdsdbaasv1alpha1.RDSInstance) {
	instanceStatus := parseDBInstanceStatus(dbInstance)
	rdsInstance.Status.InstanceInfo = instanceStatus
}

// SetupWithManager sets up the controller with the Manager.
func (r *RDSInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rdsdbaasv1alpha1.RDSInstance{}).
		Watches(
			&source.Kind{Type: &rdsv1alpha1.DBInstance{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				return getOwnerInstanceRequests(o)
			}),
		).
		Complete(r)
}

// Code from operator-lib: https://github.com/operator-framework/operator-lib/blob/d389ad4d93a46dba047b11161b755141fc853098/handler/enqueue_annotation.go#L121
func getOwnerInstanceRequests(object client.Object) []reconcile.Request {
	if typeString, ok := object.GetAnnotations()[ophandler.TypeAnnotation]; ok && typeString == rdsInstanceType {
		namespacedNameString, ok := object.GetAnnotations()[ophandler.NamespacedNameAnnotation]
		if !ok || strings.TrimSpace(namespacedNameString) == "" {
			return []reconcile.Request{}
		}
		nsn := parseNamespacedName(namespacedNameString)
		return []reconcile.Request{
			{
				NamespacedName: nsn,
			},
		}
	}
	return []reconcile.Request{}
}
