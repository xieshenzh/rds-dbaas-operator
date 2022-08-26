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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
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
	controllersrds "github.com/RHEcosystemAppEng/rds-dbaas-operator/controllers/rds"
	rdsv1alpha1 "github.com/aws-controllers-k8s/rds-controller/apis/v1alpha1"
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypesv2 "github.com/aws/aws-sdk-go-v2/service/rds/types"
	ophandler "github.com/operator-framework/operator-lib/handler"
)

const (
	rdsInventoryType = "RDSInventory.dbaas.redhat.com"

	inventoryFinalizer = "rds.dbaas.redhat.com/inventory"

	awsAccessKeyID              = "AWS_ACCESS_KEY_ID"
	awsSecretAccessKey          = "AWS_SECRET_ACCESS_KEY" //#nosec G101
	awsRegion                   = "AWS_REGION"
	ackResourceTags             = "ACK_RESOURCE_TAGS"
	ackLogLevel                 = "ACK_LOG_LEVEL"
	ackEnableDevelopmentLogging = "ACK_ENABLE_DEVELOPMENT_LOGGING"
	ackWatchNamespace           = "ACK_WATCH_NAMESPACE"
	awsEndpointUrl              = "AWS_ENDPOINT_URL"

	awsAccessKeyIDHelpText     = "The AWS Access Key ID is the value associated with a user's security credentials within the IAM console."
	awsSecretAccessKeyHelpText = "The AWS Secret Access Key is the value associated with a user's security credentials within the IAM console."
	awsRegionHelpText          = "The geographical region where your AWS resources reside."
	ackResourceTagsHelpText    = "Optionally, you can set key:value pair tags on resources managed by the service controller."
	ackLogLevelHelpText        = "Optionally, you can set the logging level on the RDS controller for OpenShift Database Access. The default value is \"info\". The only valid values are \"info\", and \"debug\"."

	secretName    = "ack-rds-user-secrets" //#nosec G101
	configmapName = "ack-rds-user-config"

	adoptedResourceCRDFile = "services.k8s.aws_adoptedresources.yaml"
	fieldExportCRDFile     = "services.k8s.aws_fieldexports.yaml"
	ackDeploymentName      = "ack-rds-controller"

	adpotedDBInstanceLabelKey   = "rds.dbaas.redhat.com/adopted"
	adpotedDBInstanceLabelValue = "true"

	inventoryConditionReady = "SpecSynced"

	inventoryStatusReasonSyncOK       = "SyncOK"
	inventoryStatusReasonInputError   = "InputError"
	inventoryStatusReasonBackendError = "BackendError"
	inventoryStatusReasonNotFound     = "NotFound"

	inventoryStatusMessageUpdateError         = "Failed to update Inventory"
	inventoryStatusMessageGetInstancesError   = "Failed to get Instances"
	inventoryStatusMessageAdoptInstanceError  = "Failed to adopt DB Instance"
	inventoryStatusMessageUpdateInstanceError = "Failed to update DB Instance"
	inventoryStatusMessageGetError            = "Failed to get %s"
	inventoryStatusMessageDeleteError         = "Failed to delete %s"
	inventoryStatusMessageCreateOrUpdateError = "Failed to create or update %s"
	inventoryStatusMessageCredentialsError    = "The AWS service account is not valid for accessing RDS DB instances"
	inventoryStatusMessageInstallError        = "Failed to install %s for RDS controller"
	inventoryStatusMessageVerifyInstallError  = "Failed to verify %s ready for RDS controller"
	inventoryStatusMessageUninstallError      = "Failed to uninstall RDS controller"

	requiredCredentialErrorTemplate = "required credential %s is missing"
)

// RDSInventoryReconciler reconciles a RDSInventory object
type RDSInventoryReconciler struct {
	client.Client
	Scheme                             *runtime.Scheme
	GetDescribeDBInstancesPaginatorAPI func(accessKey, secretKey, region string) controllersrds.DescribeDBInstancesPaginatorAPI
	GetModifyDBInstanceAPI             func(accessKey, secretKey, region string) controllersrds.ModifyDBInstanceAPI
	GetDescribeDBInstancesAPI          func(accessKey, secretKey, region string) controllersrds.DescribeDBInstancesAPI
	ACKInstallNamespace                string
	RDSCRDFilePath                     string
	WaitForRDSControllerRetries        int
	WaitForRDSControllerInterval       time.Duration
}

//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsinventories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsinventories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsinventories/finalizers,verbs=update
//+kubebuilder:rbac:groups=rds.services.k8s.aws,resources=dbinstances,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=rds.services.k8s.aws,resources=dbinstances/finalizers,verbs=update
//+kubebuilder:rbac:groups=services.k8s.aws,resources=adoptedresources,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *RDSInventoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	var syncStatus, syncStatusReason, syncStatusMessage string
	var syncReset bool

	var inventory rdsdbaasv1alpha1.RDSInventory
	var credentialsRef v1.Secret

	var accessKey, secretKey, region string

	returnRequeueSyncReset := func() {
		result = ctrl.Result{Requeue: true}
		err = nil
		syncReset = true
	}

	returnError := func(e error, reason, message string) {
		result = ctrl.Result{}
		err = e
		syncStatus = string(metav1.ConditionFalse)
		syncStatusReason = reason
		syncStatusMessage = message
	}

	returnSyncReset := func() {
		result = ctrl.Result{}
		err = nil
		syncReset = true
	}

	returnReady := func() {
		result = ctrl.Result{}
		err = nil
		syncStatus = string(metav1.ConditionTrue)
		syncStatusReason = inventoryStatusReasonSyncOK
	}

	returnReadyRequeue := func() {
		result = ctrl.Result{Requeue: true}
		err = nil
		syncStatus = string(metav1.ConditionTrue)
		syncStatusReason = inventoryStatusReasonSyncOK
	}

	updateInventoryReadyCondition := func() {
		if syncReset {
			apimeta.RemoveStatusCondition(&inventory.Status.Conditions, inventoryConditionReady)
		} else {
			condition := metav1.Condition{
				Type:    inventoryConditionReady,
				Status:  metav1.ConditionStatus(syncStatus),
				Reason:  syncStatusReason,
				Message: syncStatusMessage,
			}
			apimeta.SetStatusCondition(&inventory.Status.Conditions, condition)
		}
		if e := r.Status().Update(ctx, &inventory); e != nil {
			if errors.IsConflict(e) {
				logger.Info("Inventory modified, retry reconciling")
				result = ctrl.Result{Requeue: true}
			} else {
				logger.Error(e, "Failed to update Inventory status")
				if err == nil {
					err = e
				}
			}
		}
	}

	checkFinalizer := func() bool {
		if inventory.ObjectMeta.DeletionTimestamp.IsZero() {
			if !controllerutil.ContainsFinalizer(&inventory, inventoryFinalizer) {
				controllerutil.AddFinalizer(&inventory, inventoryFinalizer)
				if e := r.Update(ctx, &inventory); e != nil {
					if errors.IsConflict(e) {
						logger.Info("Inventory modified, retry reconciling")
						returnRequeueSyncReset()
						return true
					}
					logger.Error(e, "Failed to add finalizer to Inventory")
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateError)
					return true
				}
				logger.Info("Finalizer added to Inventory")
				returnSyncReset()
				return true
			}
		} else {
			if controllerutil.ContainsFinalizer(&inventory, inventoryFinalizer) {
				adoptedResourceList := &ackv1alpha1.AdoptedResourceList{}
				if e := r.List(ctx, adoptedResourceList, client.InNamespace(inventory.Namespace)); e != nil {
					returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageDeleteError, "AdoptedResource"))
					return true
				}
				deletingAdoptedResource := false
				for i := range adoptedResourceList.Items {
					adoptedResource := adoptedResourceList.Items[i]
					if typeString, ok := adoptedResource.GetAnnotations()[ophandler.TypeAnnotation]; ok && typeString == rdsInventoryType {
						namespacedNameString, ok := adoptedResource.GetAnnotations()[ophandler.NamespacedNameAnnotation]
						if !ok || strings.TrimSpace(namespacedNameString) == "" {
							continue
						}
						nsn := parseNamespacedName(namespacedNameString)
						if nsn.Name == inventory.Name && nsn.Namespace == inventory.Namespace {
							if adoptedResource.ObjectMeta.DeletionTimestamp.IsZero() {
								if e := r.Delete(ctx, &adoptedResource); e != nil {
									returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageDeleteError, "AdoptedResource"))
									return true
								}
							}
							deletingAdoptedResource = true
						}
					}
				}
				if deletingAdoptedResource {
					returnRequeueSyncReset()
					return true
				}

				if e := r.stopRDSController(ctx, r.Client, false); e != nil {
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUninstallError)
					return true
				}

				deletingRDSConfig := false
				secret := &v1.Secret{}
				if e := r.Get(ctx, client.ObjectKey{Namespace: r.ACKInstallNamespace, Name: secretName}, secret); e != nil {
					if !errors.IsNotFound(e) {
						returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageGetError, "Secret"))
						return true
					}
				} else {
					if e := r.Delete(ctx, secret); e != nil {
						returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageDeleteError, "Secret"))
						return true
					}
					deletingRDSConfig = true
				}
				configmap := &v1.ConfigMap{}
				if e := r.Get(ctx, client.ObjectKey{Namespace: r.ACKInstallNamespace, Name: configmapName}, configmap); e != nil {
					if !errors.IsNotFound(e) {
						returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageGetError, "ConfigMap"))
						return true
					}
				} else {
					if e := r.Delete(ctx, configmap); e != nil {
						returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageDeleteError, "ConfigMap"))
						return true
					}
					deletingRDSConfig = true
				}
				if deletingRDSConfig {
					returnRequeueSyncReset()
					return true
				}

				controllerutil.RemoveFinalizer(&inventory, inventoryFinalizer)
				if e := r.Update(ctx, &inventory); e != nil {
					if errors.IsConflict(e) {
						logger.Info("Inventory modified, retry reconciling")
						returnRequeueSyncReset()
						return true
					}
					logger.Error(e, "Failed to remove finalizer from Inventory")
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateError)
					return true
				}
				logger.Info("Finalizer removed from Inventory")
				returnSyncReset()
				return true
			}
			// Stop reconciliation as the item is being deleted
			returnSyncReset()
			return true
		}
		return false
	}

	validateAWSParameter := func() bool {
		if e := r.Get(ctx, client.ObjectKey{Namespace: inventory.Namespace,
			Name: inventory.Spec.CredentialsRef.Name}, &credentialsRef); e != nil {
			logger.Error(e, "Failed to get credentials reference for Inventory")
			if errors.IsNotFound(e) {
				returnError(e, inventoryStatusReasonNotFound, fmt.Sprintf(inventoryStatusMessageGetError, "Credential"))
			}
			returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageGetError, "Credential"))
			return true
		}

		if ak, ok := credentialsRef.Data[awsAccessKeyID]; !ok || len(ak) == 0 {
			e := fmt.Errorf(requiredCredentialErrorTemplate, awsAccessKeyID)
			returnError(e, inventoryStatusReasonInputError, e.Error())
			return true
		} else {
			accessKey = string(ak)
		}
		if sk, ok := credentialsRef.Data[awsSecretAccessKey]; !ok || len(sk) == 0 {
			e := fmt.Errorf(requiredCredentialErrorTemplate, awsSecretAccessKey)
			returnError(e, inventoryStatusReasonInputError, e.Error())
			return true
		} else {
			secretKey = string(sk)
		}
		if r, ok := credentialsRef.Data[awsRegion]; !ok || len(r) == 0 {
			e := fmt.Errorf(requiredCredentialErrorTemplate, awsRegion)
			returnError(e, inventoryStatusReasonInputError, e.Error())
			return true
		} else {
			region = string(r)
		}

		describeDBInstances := r.GetDescribeDBInstancesAPI(accessKey, secretKey, region)
		input := &rds.DescribeDBInstancesInput{
			MaxRecords: pointer.Int32(20),
		}
		if _, e := describeDBInstances.DescribeDBInstances(ctx, input); e != nil {
			logger.Error(e, "Failed to read the DB instances with the AWS service account")
			returnError(e, inventoryStatusReasonInputError, inventoryStatusMessageCredentialsError)
			return true
		}

		return false
	}

	installRDSController := func() bool {
		if e := r.createOrUpdateSecret(ctx, &inventory, &credentialsRef); e != nil {
			logger.Error(e, "Failed to create or update secret for Inventory")
			returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageCreateOrUpdateError, "Secret"))
			return true
		}
		if e := r.createOrUpdateConfigMap(ctx, &inventory, &credentialsRef); e != nil {
			logger.Error(e, "Failed to create or update configmap for Inventory")
			returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageCreateOrUpdateError, "ConfigMap"))
			return true
		}

		if e := r.startRDSController(ctx); e != nil {
			logger.Error(e, "Failed to start RDS controller")
			returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageInstallError, "Operator Deployment"))
			return true
		}

		if r, e := r.waitForRDSController(ctx); e != nil {
			logger.Error(e, "Failed to check operator Deployment for RDS controller installation")
			returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageVerifyInstallError, "Operator Deployment"))
			return true
		} else if !r {
			returnRequeueSyncReset()
			return true
		}
		return false
	}

	adoptDBInstances := func() (bool, bool) {
		var awsDBInstances []rdstypesv2.DBInstance
		describeDBInstancesPaginator := r.GetDescribeDBInstancesPaginatorAPI(accessKey, secretKey, region)
		for describeDBInstancesPaginator.HasMorePages() {
			if output, e := describeDBInstancesPaginator.NextPage(ctx); e != nil {
				logger.Error(e, "Failed to read DB Instances of the Inventory from AWS")
				returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetInstancesError)
				return true, false
			} else if output != nil {
				awsDBInstances = append(awsDBInstances, output.DBInstances...)
			}
		}

		awsDBInstanceMap := make(map[string]rdstypesv2.DBInstance, len(awsDBInstances))
		if len(awsDBInstances) > 0 {
			// query all db instances in cluster
			clusterDBInstanceList := &rdsv1alpha1.DBInstanceList{}
			if e := r.List(ctx, clusterDBInstanceList, client.InNamespace(inventory.Namespace)); e != nil {
				logger.Error(e, "Failed to read DB Instances of the Inventory in the cluster")
				returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetInstancesError)
				return true, false
			}

			dbInstanceMap := make(map[string]rdsv1alpha1.DBInstance, len(clusterDBInstanceList.Items))
			for _, dbInstance := range clusterDBInstanceList.Items {
				dbInstanceMap[*dbInstance.Spec.DBInstanceIdentifier] = dbInstance
			}

			adoptedResourceList := &ackv1alpha1.AdoptedResourceList{}
			if e := r.List(ctx, adoptedResourceList, client.InNamespace(inventory.Namespace)); e != nil {
				logger.Error(e, "Failed to read adopted DB Instances of the Inventory in the cluster")
				returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetInstancesError)
				return true, false
			}
			adoptedDBInstanceMap := make(map[string]ackv1alpha1.AdoptedResource, len(adoptedResourceList.Items))
			for _, adoptedDBInstance := range adoptedResourceList.Items {
				adoptedDBInstanceMap[adoptedDBInstance.Spec.AWS.NameOrID] = adoptedDBInstance
			}

			adoptingResource := false
			for i := range awsDBInstances {
				dbInstance := awsDBInstances[i]
				awsDBInstanceMap[*dbInstance.DBInstanceIdentifier] = dbInstance
				if dbInstance.Engine == nil {
					continue
				} else {
					switch *dbInstance.Engine {
					case aurora, auroraMysql, auroraPostgresql, customOracleEe, customSqlserverEe, customSqlserverSe, customSqlserverWeb:
						continue
					default:
					}
				}
				if dbInstance.DBClusterIdentifier != nil {
					continue
				}
				if dbInstance.DBInstanceStatus != nil && *dbInstance.DBInstanceStatus == "deleting" {
					continue
				}
				if _, ok := dbInstanceMap[*dbInstance.DBInstanceIdentifier]; !ok {
					adoptingResource = true
					if _, ok := adoptedDBInstanceMap[*dbInstance.DBInstanceIdentifier]; !ok {
						adoptedDBInstance := createAdoptedResource(&dbInstance, &inventory)
						if e := ophandler.SetOwnerAnnotations(&inventory, adoptedDBInstance); e != nil {
							logger.Error(e, "Failed to create adopted DB Instance in the cluster")
							returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageAdoptInstanceError)
							return true, false
						}
						if e := r.Create(ctx, adoptedDBInstance); e != nil {
							logger.Error(e, "Failed to create adopted DB Instance in the cluster")
							returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageAdoptInstanceError)
							return true, false
						}
					}
				}
			}
			if adoptingResource {
				logger.Info("DB Instance adopted")
				returnRequeueSyncReset()
				return true, false
			}
		}

		adoptedDBInstanceList := &rdsv1alpha1.DBInstanceList{}
		if e := r.List(ctx, adoptedDBInstanceList, client.InNamespace(inventory.Namespace),
			client.MatchingLabels(map[string]string{adpotedDBInstanceLabelKey: adpotedDBInstanceLabelValue})); e != nil {
			logger.Error(e, "Failed to read adopted DB Instances of the Inventory that are created by the operator")
			returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetInstancesError)
			return true, false
		}
		modifyDBInstance := r.GetModifyDBInstanceAPI(accessKey, secretKey, region)
		waitForAdoptedResource := false
		for i := range adoptedDBInstanceList.Items {
			adoptedDBInstance := adoptedDBInstanceList.Items[i]
			if adoptedDBInstance.Spec.Engine == nil {
				continue
			} else {
				switch *adoptedDBInstance.Spec.Engine {
				case aurora, auroraMysql, auroraPostgresql, customOracleEe, customSqlserverEe, customSqlserverSe, customSqlserverWeb:
					continue
				default:
				}
			}
			if adoptedDBInstance.Spec.DBClusterIdentifier != nil {
				continue
			}
			if adoptedDBInstance.Status.DBInstanceStatus != nil && *adoptedDBInstance.Status.DBInstanceStatus == "deleting" {
				continue
			}
			if adoptedDBInstance.Status.ACKResourceMetadata == nil || adoptedDBInstance.Status.ACKResourceMetadata.ARN == nil {
				continue
			}
			awsDBInstance, awsOk := awsDBInstanceMap[*adoptedDBInstance.Spec.DBInstanceIdentifier]
			if !awsOk {
				continue
			} else if awsDBInstance.DBInstanceArn == nil ||
				*awsDBInstance.DBInstanceArn != string(*adoptedDBInstance.Status.ACKResourceMetadata.ARN) {
				continue
			}

			if adoptedDBInstance.Spec.MasterUsername == nil || adoptedDBInstance.Spec.DBName == nil {
				update := false
				if adoptedDBInstance.Spec.MasterUsername == nil && awsDBInstance.MasterUsername != nil {
					adoptedDBInstance.Spec.MasterUsername = pointer.String(*awsDBInstance.MasterUsername)
					update = true
				}
				if adoptedDBInstance.Spec.DBName == nil && awsDBInstance.DBName != nil {
					adoptedDBInstance.Spec.DBName = pointer.String(*awsDBInstance.DBName)
					update = true
				}
				if update {
					if e := r.Update(ctx, &adoptedDBInstance); e != nil {
						if errors.IsConflict(e) {
							logger.Info("Adopted DB Instance modified, retry reconciling")
							returnRequeueSyncReset()
							return true, false
						}
						logger.Error(e, "Failed to update connection info of the adopted DB Instance", "DB Instance", adoptedDBInstance)
						returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateInstanceError)
						return true, false
					}
				}
			}

			if adoptedDBInstance.Spec.MasterUserPassword == nil {
				if adoptedDBInstance.Status.DBInstanceStatus == nil || *adoptedDBInstance.Status.DBInstanceStatus != "available" {
					waitForAdoptedResource = true
					continue
				}
				s, e := setCredentials(ctx, r.Client, r.Scheme, &adoptedDBInstance, inventory.Namespace, &adoptedDBInstance, adoptedDBInstance.Kind)
				if e != nil {
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateInstanceError)
					return true, false
				}
				password := s.Data["password"]
				input := &rds.ModifyDBInstanceInput{
					DBInstanceIdentifier: adoptedDBInstance.Spec.DBInstanceIdentifier,
					MasterUserPassword:   pointer.String(string(password)),
					ApplyImmediately:     true,
				}
				if _, e := modifyDBInstance.ModifyDBInstance(ctx, input); e != nil {
					logger.Error(e, "Failed to update credentials of the adopted DB Instance", "DB Instance", adoptedDBInstance)
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateInstanceError)
					return true, false
				}
				if e := r.Update(ctx, &adoptedDBInstance); e != nil {
					if errors.IsConflict(e) {
						logger.Info("Adopted DB Instance modified, retry reconciling")
						returnRequeueSyncReset()
						return true, false
					}
					logger.Error(e, "Failed to update credentials of the adopted DB Instance", "DB Instance", adoptedDBInstance)
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateInstanceError)
					return true, false
				}
			}
		}
		if waitForAdoptedResource {
			logger.Info("DB Instance being adopted is not available, retry reconciling")
			return false, true
		}
		return false, false
	}

	syncDBInstancesStatus := func() bool {
		awsDBInstanceIdentifiers := map[string]string{}
		describeDBInstancesPaginator := r.GetDescribeDBInstancesPaginatorAPI(accessKey, secretKey, region)
		for describeDBInstancesPaginator.HasMorePages() {
			if output, e := describeDBInstancesPaginator.NextPage(ctx); e != nil {
				logger.Error(e, "Failed to read DB Instances of the Inventory from AWS")
				returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetInstancesError)
				return true
			} else if output != nil {
				for _, instance := range output.DBInstances {
					if instance.DBInstanceIdentifier != nil && instance.DBInstanceArn != nil {
						awsDBInstanceIdentifiers[*instance.DBInstanceIdentifier] = *instance.DBInstanceArn
					}
				}
			}
		}

		dbInstanceList := &rdsv1alpha1.DBInstanceList{}
		if e := r.List(ctx, dbInstanceList, client.InNamespace(inventory.Namespace)); e != nil {
			logger.Error(e, "Failed to read DB Instances of the Inventory in the cluster")
			returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetInstancesError)
			return true
		}

		var instances []dbaasv1alpha1.Instance
		for i := range dbInstanceList.Items {
			dbInstance := dbInstanceList.Items[i]
			if dbInstance.Spec.DBInstanceIdentifier == nil ||
				dbInstance.Status.ACKResourceMetadata == nil || dbInstance.Status.ACKResourceMetadata.ARN == nil {
				continue
			}
			if arn, ok := awsDBInstanceIdentifiers[*dbInstance.Spec.DBInstanceIdentifier]; !ok {
				continue
			} else if arn != string(*dbInstance.Status.ACKResourceMetadata.ARN) {
				continue
			}
			instance := dbaasv1alpha1.Instance{
				InstanceID:   *dbInstance.Spec.DBInstanceIdentifier,
				Name:         dbInstance.Name,
				InstanceInfo: parseDBInstanceStatus(&dbInstance),
			}
			instances = append(instances, instance)
		}
		inventory.Status.Instances = instances

		if e := r.Status().Update(ctx, &inventory); e != nil {
			if errors.IsConflict(e) {
				logger.Info("Inventory modified, retry reconciling")
				returnRequeueSyncReset()
				return true
			}
			logger.Error(e, "Failed to update Inventory status")
			returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateError)
			return true
		}

		return false
	}

	if err = r.Get(ctx, req.NamespacedName, &inventory); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("RDS Inventory resource not found, has been deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Error fetching RDS Inventory for reconcile")
		return ctrl.Result{}, err
	}

	defer updateInventoryReadyCondition()

	if checkFinalizer() {
		return
	}

	if validateAWSParameter() {
		return
	}

	if installRDSController() {
		return
	}

	rt, rq := adoptDBInstances()
	if rt {
		return
	}

	if syncDBInstancesStatus() {
		return
	}

	if rq {
		returnReadyRequeue()
	} else {
		returnReady()
	}
	return
}

func (r *RDSInventoryReconciler) createOrUpdateSecret(ctx context.Context, inventory *rdsdbaasv1alpha1.RDSInventory,
	credentialsRef *v1.Secret) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: r.ACKInstallNamespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		secret.ObjectMeta.Labels = buildInventoryLabels()
		secret.ObjectMeta.Annotations = buildInventoryAnnotations(inventory, &secret.ObjectMeta)
		if e := ophandler.SetOwnerAnnotations(inventory, secret); e != nil {
			return e
		}
		secret.Data = map[string][]byte{
			awsAccessKeyID:     credentialsRef.Data[awsAccessKeyID],
			awsSecretAccessKey: credentialsRef.Data[awsSecretAccessKey],
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *RDSInventoryReconciler) createOrUpdateConfigMap(ctx context.Context, inventory *rdsdbaasv1alpha1.RDSInventory,
	credentialsRef *v1.Secret) error {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: r.ACKInstallNamespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.ObjectMeta.Labels = buildInventoryLabels()
		cm.ObjectMeta.Annotations = buildInventoryAnnotations(inventory, &cm.ObjectMeta)
		if e := ophandler.SetOwnerAnnotations(inventory, cm); e != nil {
			return e
		}
		cm.Data = map[string]string{
			awsRegion:                   string(credentialsRef.Data[awsRegion]),
			awsEndpointUrl:              "",
			ackEnableDevelopmentLogging: "false",
			ackWatchNamespace:           "",
			ackLogLevel:                 "info",
			ackResourceTags:             "rhoda",
		}
		if l, ok := credentialsRef.Data[ackLogLevel]; ok {
			cm.Data[ackLogLevel] = string(l)
		}
		if t, ok := credentialsRef.Data[ackResourceTags]; ok {
			cm.Data[ackResourceTags] = string(t)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func buildInventoryLabels() map[string]string {
	return map[string]string{
		dbaasv1alpha1.TypeLabelKey: dbaasv1alpha1.TypeLabelValue,
	}
}

func buildInventoryAnnotations(inventory *rdsdbaasv1alpha1.RDSInventory, obj *metav1.ObjectMeta) map[string]string {
	annotations := map[string]string{}
	if obj.Annotations != nil {
		for key, value := range obj.Annotations {
			annotations[key] = value
		}
	}
	annotations["managed-by"] = "rds-dbaas-operator"
	annotations["owner"] = inventory.Name
	annotations["owner.kind"] = inventory.Kind
	annotations["owner.namespace"] = inventory.Namespace
	return annotations
}

func (r *RDSInventoryReconciler) installCRD(ctx context.Context, cli client.Client, file string) error {
	crd, err := r.readCRDFile(file)
	if err != nil {
		return err
	}
	c := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: crd.Name,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, cli, c, func() error {
		c.Spec = crd.Spec
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (r *RDSInventoryReconciler) readCRDFile(file string) (*apiextensionsv1.CustomResourceDefinition, error) {
	d, err := ioutil.ReadFile(filepath.Clean(file))
	if err != nil {
		return nil, err
	}
	jsonData, err := yaml.ToJSON(d)
	if err != nil {
		return nil, err
	}
	csv := &apiextensionsv1.CustomResourceDefinition{}
	if err := json.Unmarshal(jsonData, csv); err != nil {
		return nil, err
	}

	return csv, nil
}

func (r *RDSInventoryReconciler) waitForRDSController(ctx context.Context) (bool, error) {
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.ACKInstallNamespace, Name: ackDeploymentName}, deployment); err != nil {
		return false, err
	}
	if deployment.Status.ReadyReplicas > 0 {
		return true, nil
	}
	return false, nil
}

func (r *RDSInventoryReconciler) stopRDSController(ctx context.Context, cli client.Client, wait bool) error {
	logger := log.FromContext(ctx)

	deployment := &appsv1.Deployment{}
	waitCounter := 0
	for {
		if err := cli.Get(ctx, client.ObjectKey{Namespace: r.ACKInstallNamespace, Name: ackDeploymentName}, deployment); err != nil {
			if errors.IsNotFound(err) {
				if wait && waitCounter < r.WaitForRDSControllerRetries {
					logger.Info("Wait for the installation of the RDS controller")
					time.Sleep(r.WaitForRDSControllerInterval)
					waitCounter++
					continue
				} else {
					return err
				}
			}
			return err
		}
		break
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas > 0 {
		deployment.Spec.Replicas = pointer.Int32(0)
		if err := cli.Update(ctx, deployment); err != nil {
			return err
		}
	}
	return nil
}

func (r *RDSInventoryReconciler) startRDSController(ctx context.Context) error {
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.ACKInstallNamespace, Name: ackDeploymentName}, deployment); err != nil {
		return err
	}
	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 1 {
		deployment.Spec.Replicas = pointer.Int32(1)
		if err := r.Update(ctx, deployment); err != nil {
			return err
		}
	}
	return nil
}

func createAdoptedResource(dbInstance *rdstypesv2.DBInstance, inventory *rdsdbaasv1alpha1.RDSInventory) *ackv1alpha1.AdoptedResource {
	return &ackv1alpha1.AdoptedResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: inventory.Namespace,
			Name:      fmt.Sprintf("rhoda-adopted-db-instance-%s", strings.ToLower(*dbInstance.DBInstanceIdentifier)),
			Annotations: map[string]string{
				"managed-by":      "rds-dbaas-operator",
				"owner":           inventory.Name,
				"owner.kind":      inventory.Kind,
				"owner.namespace": inventory.Namespace,
			},
		},
		Spec: ackv1alpha1.AdoptedResourceSpec{
			Kubernetes: &ackv1alpha1.ResourceWithMetadata{
				GroupKind: metav1.GroupKind{
					Group: rdsv1alpha1.GroupVersion.Group,
					Kind:  rdsInstanceKind,
				},
				Metadata: &ackv1alpha1.PartialObjectMeta{
					Namespace: inventory.Namespace,
					Labels: map[string]string{
						adpotedDBInstanceLabelKey: adpotedDBInstanceLabelValue,
					},
				},
			},
			AWS: &ackv1alpha1.AWSIdentifiers{
				NameOrID: *dbInstance.DBInstanceIdentifier,
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RDSInventoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()

	kubeConfig := mgr.GetConfig()
	cli, err := client.New(kubeConfig, client.Options{
		Scheme: mgr.GetScheme(),
	})
	if err != nil {
		return err
	}
	if err := r.stopRDSController(ctx, cli, true); err != nil {
		return err
	}

	if err := r.installCRD(ctx, cli, filepath.Join(r.RDSCRDFilePath, adoptedResourceCRDFile)); err != nil {
		return err
	}
	if err := r.installCRD(ctx, cli, filepath.Join(r.RDSCRDFilePath, fieldExportCRDFile)); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&rdsdbaasv1alpha1.RDSInventory{}).
		Watches(
			&source.Kind{Type: &rdsv1alpha1.DBInstance{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				return getInstanceInventoryRequests(o, mgr)
			}),
		).
		Watches(
			&source.Kind{Type: &appsv1.Deployment{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				return getACKDeploymentInventoryRequests(o, r.ACKInstallNamespace, mgr)
			}),
		).
		Complete(r)
}

func getInstanceInventoryRequests(object client.Object, mgr ctrl.Manager) []reconcile.Request {
	ctx := context.Background()
	logger := log.FromContext(ctx)
	cli := mgr.GetClient()

	dbInstance := object.(*rdsv1alpha1.DBInstance)
	inventoryList := &rdsdbaasv1alpha1.RDSInventoryList{}
	if e := cli.List(ctx, inventoryList, client.InNamespace(dbInstance.Namespace)); e != nil {
		logger.Error(e, "Failed to get Inventories for DB Instance update", "DBInstance ID", dbInstance.Spec.DBInstanceIdentifier)
		return nil
	}

	var requests []reconcile.Request
	for _, i := range inventoryList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: i.Namespace,
				Name:      i.Name,
			},
		})
	}
	return requests
}

func getACKDeploymentInventoryRequests(object client.Object, namespace string, mgr ctrl.Manager) []reconcile.Request {
	ctx := context.Background()
	logger := log.FromContext(ctx)
	cli := mgr.GetClient()

	deployment := object.(*appsv1.Deployment)

	var requests []reconcile.Request
	if deployment.Name == ackDeploymentName && deployment.Namespace == namespace {
		inventoryList := &rdsdbaasv1alpha1.RDSInventoryList{}
		if e := cli.List(ctx, inventoryList); e != nil {
			logger.Error(e, "Failed to get Inventories for ACK controller update")
			return nil
		}

		for _, i := range inventoryList.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: i.Namespace,
					Name:      i.Name,
				},
			})
		}
	}
	return requests
}
