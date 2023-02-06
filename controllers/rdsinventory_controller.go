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
	_ "embed"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	dbaasv1beta1 "github.com/RHEcosystemAppEng/dbaas-operator/api/v1beta1"
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

	secretName    = "ack-rds-user-secrets" //#nosec G101
	configmapName = "ack-rds-user-config"

	ackDeploymentName = "ack-rds-controller"

	adoptedDBResourceLabelKey   = "rds.dbaas.redhat.com/adopted"
	adoptedDBResourceLabelValue = "true"

	inventoryConditionReady = "SpecSynced"

	inventoryStatusReasonSyncOK       = "SyncOK"
	inventoryStatusReasonInputError   = "InputError"
	inventoryStatusReasonBackendError = "BackendError"
	inventoryStatusReasonNotFound     = "NotFound"

	inventoryStatusMessageUpdateError              = "Failed to update Inventory"
	inventoryStatusMessageGetInstancesError        = "Failed to get Instances"
	inventoryStatusMessageGetClustersError         = "Failed to get clusters"
	inventoryStatusMessageDeleteInstancesError     = "Failed to delete Instances"
	inventoryStatusMessageDeleteClustersError      = "Failed to delete clusters"
	inventoryStatusMessageAdoptInstanceError       = "Failed to adopt DB Instance"
	inventoryStatusMessageAdoptClusterError        = "Failed to adopt DB Cluster"
	inventoryStatusMessageUpdateInstanceError      = "Failed to update DB Instance"
	inventoryStatusMessageUpdateClusterError       = "Failed to update DB Cluster"
	inventoryStatusMessageGetError                 = "Failed to get %s"
	inventoryStatusMessageDeleteError              = "Failed to delete %s"
	inventoryStatusMessageResetError               = "Failed to reset %s"
	inventoryStatusMessageCreateOrUpdateError      = "Failed to create or update %s"
	inventoryStatusMessageCredentialsInstanceError = "The AWS service account is not valid for accessing RDS DB instances"
	inventoryStatusMessageCredentialsClusterError  = "The AWS service account is not valid for accessing RDS DB clusters"
	inventoryStatusMessageInstallError             = "Failed to install %s for RDS controller"
	inventoryStatusMessageVerifyInstallError       = "Failed to verify %s ready for RDS controller"
	inventoryStatusMessageUninstallError           = "Failed to uninstall RDS controller"

	requiredCredentialErrorTemplate = "required credential %s is missing"
)

//go:embed yaml/rds/common/bases/services.k8s.aws_adoptedresources.yaml
var adoptedResourceCRDYaml []byte

//go:embed yaml/rds/common/bases/services.k8s.aws_fieldexports.yaml
var fieldExportCRDYaml []byte

// RDSInventoryReconciler reconciles a RDSInventory object
type RDSInventoryReconciler struct {
	client.Client
	Scheme                             *runtime.Scheme
	GetDescribeDBInstancesPaginatorAPI func(accessKey, secretKey, region string) controllersrds.DescribeDBInstancesPaginatorAPI
	GetModifyDBInstanceAPI             func(accessKey, secretKey, region string) controllersrds.ModifyDBInstanceAPI
	GetDescribeDBInstancesAPI          func(accessKey, secretKey, region string) controllersrds.DescribeDBInstancesAPI
	GetDescribeDBClustersPaginatorAPI  func(accessKey, secretKey, region string) controllersrds.DescribeDBClustersPaginatorAPI
	GetModifyDBClusterAPI              func(accessKey, secretKey, region string) controllersrds.ModifyDBClusterAPI
	GetDescribeDBClustersAPI           func(accessKey, secretKey, region string) controllersrds.DescribeDBClustersAPI
	ACKInstallNamespace                string
	WaitForRDSControllerRetries        int
	WaitForRDSControllerInterval       time.Duration
}

//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsinventories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsinventories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsinventories/finalizers,verbs=update
//+kubebuilder:rbac:groups=rds.services.k8s.aws,resources=dbinstances,verbs=get;list;watch;update;delete
//+kubebuilder:rbac:groups=rds.services.k8s.aws,resources=dbinstances/finalizers,verbs=update
//+kubebuilder:rbac:groups=rds.services.k8s.aws,resources=dbclusters,verbs=get;list;watch;update;delete
//+kubebuilder:rbac:groups=rds.services.k8s.aws,resources=dbclusters/finalizers,verbs=update
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

				if e := r.createOrUpdateSecret(ctx, r.Client, nil); e != nil {
					returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageResetError, "Secret"))
					return true
				}
				if e := r.createOrUpdateConfigMap(ctx, r.Client, nil); e != nil {
					returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageResetError, "ConfigMap"))
					return true
				}

				if e := r.stopRDSController(ctx, r.Client, false); e != nil {
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUninstallError)
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
		instanceInput := &rds.DescribeDBInstancesInput{
			MaxRecords: pointer.Int32(20),
		}
		if _, e := describeDBInstances.DescribeDBInstances(ctx, instanceInput); e != nil {
			logger.Error(e, "Failed to read the DB instances with the AWS service account")
			returnError(e, inventoryStatusReasonInputError, inventoryStatusMessageCredentialsInstanceError)
			return true
		}

		describeDBClusters := r.GetDescribeDBClustersAPI(accessKey, secretKey, region)
		clusterInput := &rds.DescribeDBClustersInput{
			MaxRecords: pointer.Int32(20),
		}
		if _, e := describeDBClusters.DescribeDBClusters(ctx, clusterInput); e != nil {
			logger.Error(e, "Failed to read the DB clusters with the AWS service account")
			returnError(e, inventoryStatusReasonInputError, inventoryStatusMessageCredentialsClusterError)
			return true
		}

		return false
	}

	installRDSController := func() bool {
		if e := r.createOrUpdateSecret(ctx, r.Client, &credentialsRef); e != nil {
			logger.Error(e, "Failed to create or update secret for Inventory")
			returnError(e, inventoryStatusReasonBackendError, fmt.Sprintf(inventoryStatusMessageCreateOrUpdateError, "Secret"))
			return true
		}
		if e := r.createOrUpdateConfigMap(ctx, r.Client, &credentialsRef); e != nil {
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

			dbInstanceMap := make(map[string]string, len(clusterDBInstanceList.Items))
			for _, dbInstance := range clusterDBInstanceList.Items {
				if dbInstance.Spec.DBInstanceIdentifier != nil &&
					dbInstance.Status.ACKResourceMetadata != nil && dbInstance.Status.ACKResourceMetadata.ARN != nil {
					dbInstanceMap[string(*dbInstance.Status.ACKResourceMetadata.ARN)] = *dbInstance.Spec.DBInstanceIdentifier
				}
			}

			adoptedResourceList := &ackv1alpha1.AdoptedResourceList{}
			if e := r.List(ctx, adoptedResourceList, client.InNamespace(inventory.Namespace)); e != nil {
				logger.Error(e, "Failed to read adopted DB Instances of the Inventory in the cluster")
				returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetInstancesError)
				return true, false
			}
			adoptedDBInstanceMap := make(map[string]ackv1alpha1.AdoptedResource, len(adoptedResourceList.Items))
			for _, adoptedDBInstance := range adoptedResourceList.Items {
				if adoptedDBInstance.Spec.AWS != nil && adoptedDBInstance.Spec.AWS.ARN != nil {
					adoptedDBInstanceMap[string(*adoptedDBInstance.Spec.AWS.ARN)] = adoptedDBInstance
				}
			}

			adoptingResource := false
			for i := range awsDBInstances {
				dbInstance := awsDBInstances[i]

				if dbInstance.DBInstanceArn == nil {
					continue
				}
				awsDBInstanceMap[*dbInstance.DBInstanceArn] = dbInstance

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

				if _, ok := dbInstanceMap[*dbInstance.DBInstanceArn]; ok {
					continue
				}

				if adoptedDBInstance, ok := adoptedDBInstanceMap[*dbInstance.DBInstanceArn]; ok {
					// Wait for the DB instances that are being adopted
					if adoptedDBInstance.Status.Conditions == nil {
						adoptingResource = true
					} else {
						adopted := false
						for j := range adoptedDBInstance.Status.Conditions {
							condition := adoptedDBInstance.Status.Conditions[j]
							if condition != nil && condition.Type == ackv1alpha1.ConditionTypeAdopted &&
								condition.Status == v1.ConditionTrue {
								adopted = true
								break
							}
						}
						if !adopted {
							adoptingResource = true
						}
					}
					continue
				}

				adoptingResource = true
				logger.Info("Adopting DB Instance", "DB Instance Identifier", *dbInstance.DBInstanceIdentifier, "ARN", *dbInstance.DBInstanceArn)

				adoptedDBInstance := createAdoptedResource(dbInstance.DBInstanceIdentifier, dbInstance.DBInstanceArn, dbInstance.Engine, rdsInstanceKind, &inventory)
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

			if adoptingResource {
				logger.Info("Adopting DB Instance")
				returnRequeueSyncReset()
				return true, false
			}
		}

		adoptedDBInstanceList := &rdsv1alpha1.DBInstanceList{}
		if e := r.List(ctx, adoptedDBInstanceList, client.InNamespace(inventory.Namespace),
			client.MatchingLabels(map[string]string{adoptedDBResourceLabelKey: adoptedDBResourceLabelValue})); e != nil {
			logger.Error(e, "Failed to read adopted DB Instances of the Inventory that are created by the operator")
			returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetInstancesError)
			return true, false
		}

		adoptedResourceList := &ackv1alpha1.AdoptedResourceList{}
		if e := r.List(ctx, adoptedResourceList, client.InNamespace(inventory.Namespace),
			client.MatchingLabels(map[string]string{adoptedDBResourceLabelKey: adoptedDBResourceLabelValue})); e != nil {
			logger.Error(e, "Failed to read adopted Resources of the Inventory that are created by the operator")
			returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetInstancesError)
			return true, false
		}
		adoptedDBInstanceMap := make(map[string]ackv1alpha1.AdoptedResource, len(adoptedResourceList.Items))
		for _, adoptedDBInstance := range adoptedResourceList.Items {
			if adoptedDBInstance.Spec.AWS != nil && adoptedDBInstance.Spec.AWS.ARN != nil {
				adoptedDBInstanceMap[string(*adoptedDBInstance.Spec.AWS.ARN)] = adoptedDBInstance
			}
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
			awsDBInstance, awsOk := awsDBInstanceMap[string(*adoptedDBInstance.Status.ACKResourceMetadata.ARN)]
			if !awsOk {
				if e := r.Delete(ctx, &adoptedDBInstance); e != nil {
					if !errors.IsNotFound(e) {
						logger.Error(e, "Failed to delete obsolete adopted DB Instance", "DB Instance", adoptedDBInstance)
						returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageDeleteInstancesError)
						return true, false
					}
				}
				if instance, ok := adoptedDBInstanceMap[string(*adoptedDBInstance.Status.ACKResourceMetadata.ARN)]; ok {
					if e := r.Delete(ctx, &instance); e != nil {
						if !errors.IsNotFound(e) {
							logger.Error(e, "Failed to delete adopting resource", "DB Instance", adoptedDBInstance)
							returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageDeleteInstancesError)
							return true, false
						}
					}
				}
				logger.Info("Deleted adopted DB Instance that is obsolete", "DB Cluster", adoptedDBInstance)
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
					logger.Info("DB Instance is not available to reset credentials", "DB Instance Identifier", *adoptedDBInstance.Spec.DBInstanceIdentifier)
					continue
				}
				s, e := setCredentials(ctx, r.Client, r.Scheme, adoptedDBInstance.GetName(), inventory.Namespace, &adoptedDBInstance, adoptedDBInstance.Kind,
					func(secretName string) {
						if adoptedDBInstance.Spec.MasterUsername == nil {
							adoptedDBInstance.Spec.MasterUsername = pointer.String(generateUsername(*adoptedDBInstance.Spec.Engine))
						}

						adoptedDBInstance.Spec.MasterUserPassword = &ackv1alpha1.SecretKeyReference{
							SecretReference: v1.SecretReference{
								Name:      secretName,
								Namespace: inventory.Namespace,
							},
							Key: "password",
						}
					})
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

	adoptDBClusters := func() (bool, bool) {
		var awsDBClusters []rdstypesv2.DBCluster
		describeDBClustersPaginator := r.GetDescribeDBClustersPaginatorAPI(accessKey, secretKey, region)
		for describeDBClustersPaginator.HasMorePages() {
			if output, e := describeDBClustersPaginator.NextPage(ctx); e != nil {
				logger.Error(e, "Failed to read DB clusters of the Inventory from AWS")
				returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetClustersError)
				return true, false
			} else if output != nil {
				awsDBClusters = append(awsDBClusters, output.DBClusters...)
			}
		}

		awsDBClusterMap := make(map[string]rdstypesv2.DBCluster, len(awsDBClusters))
		if len(awsDBClusters) > 0 {
			// query all db clusters in cluster
			clusterDBClusterList := &rdsv1alpha1.DBClusterList{}
			if e := r.List(ctx, clusterDBClusterList, client.InNamespace(inventory.Namespace)); e != nil {
				logger.Error(e, "Failed to read DB Clusters of the Inventory in the cluster")
				returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetClustersError)
				return true, false
			}

			dbClusterMap := make(map[string]string, len(clusterDBClusterList.Items))
			for _, dbCluster := range clusterDBClusterList.Items {
				if dbCluster.Spec.DBClusterIdentifier != nil &&
					dbCluster.Status.ACKResourceMetadata != nil && dbCluster.Status.ACKResourceMetadata.ARN != nil {
					dbClusterMap[string(*dbCluster.Status.ACKResourceMetadata.ARN)] = *dbCluster.Spec.DBClusterIdentifier
				}
			}

			adoptedResourceList := &ackv1alpha1.AdoptedResourceList{}
			if e := r.List(ctx, adoptedResourceList, client.InNamespace(inventory.Namespace)); e != nil {
				logger.Error(e, "Failed to read adopted DB Clusters of the Inventory in the cluster")
				returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetClustersError)
				return true, false
			}
			adoptedDBClusterMap := make(map[string]ackv1alpha1.AdoptedResource, len(adoptedResourceList.Items))
			for _, adoptedDBCluster := range adoptedResourceList.Items {
				if adoptedDBCluster.Spec.AWS != nil && adoptedDBCluster.Spec.AWS.ARN != nil {
					adoptedDBClusterMap[string(*adoptedDBCluster.Spec.AWS.ARN)] = adoptedDBCluster
				}
			}

			adoptingResource := false
			for i := range awsDBClusters {
				dbCluster := awsDBClusters[i]

				if dbCluster.DBClusterArn == nil {
					continue
				}
				awsDBClusterMap[*dbCluster.DBClusterArn] = dbCluster

				if dbCluster.Engine == nil {
					continue
				} else {
					switch *dbCluster.Engine {
					case mariadb, oracleSe2, oracleSe2Cdb, oracleEe, oracleEeCdb, customOracleEe, sqlserverEe, sqlserverSe,
						sqlserverEx, sqlserverWeb, customSqlserverEe, customSqlserverSe, customSqlserverWeb:
						continue
					default:
					}
				}
				if dbCluster.Status != nil && *dbCluster.Status == "deleting" {
					continue
				}

				if _, ok := dbClusterMap[*dbCluster.DBClusterArn]; ok {
					continue
				}

				if adoptedDBCluster, ok := adoptedDBClusterMap[*dbCluster.DBClusterArn]; ok {
					// Wait for the DB clusters that are being adopted
					if adoptedDBCluster.Status.Conditions == nil {
						adoptingResource = true
					} else {
						adopted := false
						for j := range adoptedDBCluster.Status.Conditions {
							condition := adoptedDBCluster.Status.Conditions[j]
							if condition != nil && condition.Type == ackv1alpha1.ConditionTypeAdopted &&
								condition.Status == v1.ConditionTrue {
								adopted = true
								break
							}
						}
						if !adopted {
							adoptingResource = true
						}
					}
					continue
				}

				adoptingResource = true
				logger.Info("Adopting DB Cluster", "DB Cluster Identifier", *dbCluster.DBClusterIdentifier, "ARN", *dbCluster.DBClusterArn)

				adoptedDBCluster := createAdoptedResource(dbCluster.DBClusterIdentifier, dbCluster.DBClusterArn, dbCluster.Engine, rdsClusterKind, &inventory)
				if e := ophandler.SetOwnerAnnotations(&inventory, adoptedDBCluster); e != nil {
					logger.Error(e, "Failed to create adopted DB Cluster in the cluster")
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageAdoptClusterError)
					return true, false
				}
				if e := r.Create(ctx, adoptedDBCluster); e != nil {
					logger.Error(e, "Failed to create adopted DB Cluster in the cluster")
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageAdoptClusterError)
					return true, false
				}
			}

			if adoptingResource {
				logger.Info("Adopting DB Cluster")
				returnRequeueSyncReset()
				return true, false
			}
		}

		adoptedDBClusterList := &rdsv1alpha1.DBClusterList{}
		if e := r.List(ctx, adoptedDBClusterList, client.InNamespace(inventory.Namespace),
			client.MatchingLabels(map[string]string{adoptedDBResourceLabelKey: adoptedDBResourceLabelValue})); e != nil {
			logger.Error(e, "Failed to read adopted DB Clusters of the Inventory that are created by the operator")
			returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetClustersError)
			return true, false
		}

		adoptedResourceList := &ackv1alpha1.AdoptedResourceList{}
		if e := r.List(ctx, adoptedResourceList, client.InNamespace(inventory.Namespace),
			client.MatchingLabels(map[string]string{adoptedDBResourceLabelKey: adoptedDBResourceLabelValue})); e != nil {
			logger.Error(e, "Failed to read adopted Resources of the Inventory that are created by the operator")
			returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetClustersError)
			return true, false
		}
		adoptedDBClusterMap := make(map[string]ackv1alpha1.AdoptedResource, len(adoptedResourceList.Items))
		for _, adoptedDBCluster := range adoptedResourceList.Items {
			if adoptedDBCluster.Spec.AWS != nil && adoptedDBCluster.Spec.AWS.ARN != nil {
				adoptedDBClusterMap[string(*adoptedDBCluster.Spec.AWS.ARN)] = adoptedDBCluster
			}
		}

		modifyDBCluster := r.GetModifyDBClusterAPI(accessKey, secretKey, region)
		waitForAdoptedResource := false
		for i := range adoptedDBClusterList.Items {
			adoptedDBCluster := adoptedDBClusterList.Items[i]
			if adoptedDBCluster.Spec.Engine == nil {
				continue
			} else {
				switch *adoptedDBCluster.Spec.Engine {
				case mariadb, oracleSe2, oracleSe2Cdb, oracleEe, oracleEeCdb, customOracleEe, sqlserverEe, sqlserverSe,
					sqlserverEx, sqlserverWeb, customSqlserverEe, customSqlserverSe, customSqlserverWeb:
					continue
				default:
				}
			}
			if adoptedDBCluster.Status.Status != nil && *adoptedDBCluster.Status.Status == "deleting" {
				continue
			}
			if adoptedDBCluster.Status.ACKResourceMetadata == nil || adoptedDBCluster.Status.ACKResourceMetadata.ARN == nil {
				continue
			}
			awsDBCluster, awsOk := awsDBClusterMap[string(*adoptedDBCluster.Status.ACKResourceMetadata.ARN)]
			if !awsOk {
				if e := r.Delete(ctx, &adoptedDBCluster); e != nil {
					if !errors.IsNotFound(e) {
						logger.Error(e, "Failed to delete obsolete adopted DB Cluster", "DB Cluster", adoptedDBCluster)
						returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageDeleteClustersError)
						return true, false
					}
				}
				if cluster, ok := adoptedDBClusterMap[string(*adoptedDBCluster.Status.ACKResourceMetadata.ARN)]; ok {
					if e := r.Delete(ctx, &cluster); e != nil {
						if !errors.IsNotFound(e) {
							logger.Error(e, "Failed to delete adopting resource", "DB Cluster", adoptedDBCluster)
							returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageDeleteClustersError)
							return true, false
						}
					}
				}
				logger.Info("Deleted adopted DB Cluster that is obsolete", "DB Cluster", adoptedDBCluster)
				continue
			}

			if adoptedDBCluster.Spec.MasterUsername == nil || adoptedDBCluster.Spec.DatabaseName == nil {
				update := false
				if adoptedDBCluster.Spec.MasterUsername == nil && awsDBCluster.MasterUsername != nil {
					adoptedDBCluster.Spec.MasterUsername = pointer.String(*awsDBCluster.MasterUsername)
					update = true
				}
				if adoptedDBCluster.Spec.DatabaseName == nil && awsDBCluster.DatabaseName != nil {
					adoptedDBCluster.Spec.DatabaseName = pointer.String(*awsDBCluster.DatabaseName)
					update = true
				}
				if update {
					if e := r.Update(ctx, &adoptedDBCluster); e != nil {
						if errors.IsConflict(e) {
							logger.Info("Adopted DB Cluster modified, retry reconciling")
							returnRequeueSyncReset()
							return true, false
						}
						logger.Error(e, "Failed to update connection info of the adopted DB Cluster", "DB Cluster", adoptedDBCluster)
						returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateClusterError)
						return true, false
					}
				}
			}

			if adoptedDBCluster.Spec.MasterUserPassword == nil {
				if adoptedDBCluster.Status.Status == nil || *adoptedDBCluster.Status.Status != "available" {
					waitForAdoptedResource = true
					logger.Info("DB Cluster is not available to reset credentials", "DB Cluster Identifier", *adoptedDBCluster.Spec.DBClusterIdentifier)
					continue
				}
				s, e := setCredentials(ctx, r.Client, r.Scheme, adoptedDBCluster.GetName(), inventory.Namespace, &adoptedDBCluster, adoptedDBCluster.Kind,
					func(secretName string) {
						if adoptedDBCluster.Spec.MasterUsername == nil {
							adoptedDBCluster.Spec.MasterUsername = pointer.String(generateUsername(*adoptedDBCluster.Spec.Engine))
						}

						adoptedDBCluster.Spec.MasterUserPassword = &ackv1alpha1.SecretKeyReference{
							SecretReference: v1.SecretReference{
								Name:      secretName,
								Namespace: inventory.Namespace,
							},
							Key: "password",
						}
					})
				if e != nil {
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateClusterError)
					return true, false
				}
				password := s.Data["password"]
				input := &rds.ModifyDBClusterInput{
					DBClusterIdentifier: adoptedDBCluster.Spec.DBClusterIdentifier,
					MasterUserPassword:  pointer.String(string(password)),
					ApplyImmediately:    true,
				}
				if _, e := modifyDBCluster.ModifyDBCluster(ctx, input); e != nil {
					logger.Error(e, "Failed to update credentials of the adopted DB Cluster", "DB Cluster", adoptedDBCluster)
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateClusterError)
					return true, false
				}
				if e := r.Update(ctx, &adoptedDBCluster); e != nil {
					if errors.IsConflict(e) {
						logger.Info("Adopted DB Cluster modified, retry reconciling")
						returnRequeueSyncReset()
						return true, false
					}
					logger.Error(e, "Failed to update credentials of the adopted DB Cluster", "DB Cluster", adoptedDBCluster)
					returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateClusterError)
					return true, false
				}
			}
		}
		if waitForAdoptedResource {
			logger.Info("DB Cluster being adopted is not available, retry reconciling")
			return false, true
		}
		return false, false
	}

	syncDBInstancesStatus := func() (bool, []dbaasv1beta1.DatabaseService) {
		awsDBInstanceIdentifiers := map[string]string{}
		describeDBInstancesPaginator := r.GetDescribeDBInstancesPaginatorAPI(accessKey, secretKey, region)
		for describeDBInstancesPaginator.HasMorePages() {
			if output, e := describeDBInstancesPaginator.NextPage(ctx); e != nil {
				logger.Error(e, "Failed to read DB Instances of the Inventory from AWS")
				returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetInstancesError)
				return true, nil
			} else if output != nil {
				for _, instance := range output.DBInstances {
					if instance.DBInstanceIdentifier != nil && instance.DBInstanceArn != nil {
						awsDBInstanceIdentifiers[*instance.DBInstanceArn] = *instance.DBInstanceIdentifier
					}
				}
			}
		}

		dbInstanceList := &rdsv1alpha1.DBInstanceList{}
		if e := r.List(ctx, dbInstanceList, client.InNamespace(inventory.Namespace)); e != nil {
			logger.Error(e, "Failed to read DB Instances of the Inventory in the cluster")
			returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetInstancesError)
			return true, nil
		}

		serviceType := dbaasv1beta1.DatabaseServiceType(instanceType)
		var services []dbaasv1beta1.DatabaseService
		for i := range dbInstanceList.Items {
			dbInstance := dbInstanceList.Items[i]
			if dbInstance.Spec.DBInstanceIdentifier == nil ||
				dbInstance.Status.ACKResourceMetadata == nil || dbInstance.Status.ACKResourceMetadata.ARN == nil {
				continue
			}
			if _, ok := awsDBInstanceIdentifiers[string(*dbInstance.Status.ACKResourceMetadata.ARN)]; !ok {
				continue
			}
			service := dbaasv1beta1.DatabaseService{
				ServiceID:   *dbInstance.Spec.DBInstanceIdentifier,
				ServiceName: dbInstance.Name,
				ServiceType: &serviceType,
				ServiceInfo: parseDBInstanceStatus(&dbInstance),
			}
			services = append(services, service)
		}

		return false, services
	}

	syncDBClustersStatus := func() (bool, []dbaasv1beta1.DatabaseService) {
		awsDBClusterIdentifiers := map[string]string{}
		describeDBClustersPaginator := r.GetDescribeDBClustersPaginatorAPI(accessKey, secretKey, region)
		for describeDBClustersPaginator.HasMorePages() {
			if output, e := describeDBClustersPaginator.NextPage(ctx); e != nil {
				logger.Error(e, "Failed to read DB Clusters of the Inventory from AWS")
				returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetClustersError)
				return true, nil
			} else if output != nil {
				for _, cluster := range output.DBClusters {
					if cluster.DBClusterIdentifier != nil && cluster.DBClusterArn != nil {
						awsDBClusterIdentifiers[*cluster.DBClusterArn] = *cluster.DBClusterIdentifier
					}
				}
			}
		}

		dbClusterList := &rdsv1alpha1.DBClusterList{}
		if e := r.List(ctx, dbClusterList, client.InNamespace(inventory.Namespace)); e != nil {
			logger.Error(e, "Failed to read DB Clusters of the Inventory in the cluster")
			returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageGetClustersError)
			return true, nil
		}

		serviceType := dbaasv1beta1.DatabaseServiceType(clusterType)
		var services []dbaasv1beta1.DatabaseService
		for i := range dbClusterList.Items {
			dbCluster := dbClusterList.Items[i]
			if dbCluster.Spec.DBClusterIdentifier == nil ||
				dbCluster.Status.ACKResourceMetadata == nil || dbCluster.Status.ACKResourceMetadata.ARN == nil {
				continue
			}
			if _, ok := awsDBClusterIdentifiers[string(*dbCluster.Status.ACKResourceMetadata.ARN)]; !ok {
				continue
			}
			service := dbaasv1beta1.DatabaseService{
				ServiceID:   *dbCluster.Spec.DBClusterIdentifier,
				ServiceName: dbCluster.Name,
				ServiceType: &serviceType,
				ServiceInfo: parseDBClusterStatus(&dbCluster),
			}
			services = append(services, service)
		}

		return false, services
	}

	if len(req.NamespacedName.Name) == 0 && req.NamespacedName.Namespace == r.ACKInstallNamespace {
		inventoryList := &rdsdbaasv1alpha1.RDSInventoryList{}
		if err := r.List(ctx, inventoryList); err != nil {
			logger.Error(err, "Failed to check Inventories for ACK controller update")
			return ctrl.Result{}, err
		}
		if len(inventoryList.Items) == 0 {
			logger.Info("Deployment of the ACk controller is updated when no Inventory exists, stop the ACK controller")
			if err := r.stopRDSController(ctx, r.Client, false); err != nil {
				logger.Error(err, "Failed to stop the ACK controller")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
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

	rtc, rqc := adoptDBClusters()

	rti, rqi := adoptDBInstances()

	if rtc || rti {
		return
	}

	var services []dbaasv1beta1.DatabaseService
	if rt, sv := syncDBClustersStatus(); rt {
		return
	} else {
		services = append(services, sv...)
	}

	if rt, sv := syncDBInstancesStatus(); rt {
		return
	} else {
		services = append(services, sv...)
	}

	inventory.Status.DatabaseServices = services
	if e := r.Status().Update(ctx, &inventory); e != nil {
		if errors.IsConflict(e) {
			logger.Info("Inventory modified, retry reconciling")
			returnRequeueSyncReset()
			return
		}
		logger.Error(e, "Failed to update Inventory status")
		returnError(e, inventoryStatusReasonBackendError, inventoryStatusMessageUpdateError)
		return
	}

	if rqi || rqc {
		returnReadyRequeue()
	} else {
		returnReady()
	}
	return
}

func (r *RDSInventoryReconciler) createOrUpdateSecret(ctx context.Context, cli client.Client, credentialsRef *v1.Secret) error {
	deployment := &appsv1.Deployment{}
	if e := cli.Get(ctx, client.ObjectKey{Namespace: r.ACKInstallNamespace, Name: ackDeploymentName}, deployment); e != nil {
		return e
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: r.ACKInstallNamespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, cli, secret, func() error {
		secret.ObjectMeta.Labels = buildDBaaSLabels()
		secret.ObjectMeta.Annotations = r.buildDBaaSAnnotations(&secret.ObjectMeta)
		if len(deployment.GetObjectKind().GroupVersionKind().GroupKind().Kind) > 0 {
			if e := ophandler.SetOwnerAnnotations(deployment, secret); e != nil {
				return e
			}
		}
		if credentialsRef != nil {
			secret.Data = map[string][]byte{
				awsAccessKeyID:     credentialsRef.Data[awsAccessKeyID],
				awsSecretAccessKey: credentialsRef.Data[awsSecretAccessKey],
			}
		} else {
			secret.Data = map[string][]byte{
				awsAccessKeyID:     []byte("dummy"),
				awsSecretAccessKey: []byte("dummy"),
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *RDSInventoryReconciler) createOrUpdateConfigMap(ctx context.Context, cli client.Client, credentialsRef *v1.Secret) error {
	deployment := &appsv1.Deployment{}
	if e := cli.Get(ctx, client.ObjectKey{Namespace: r.ACKInstallNamespace, Name: ackDeploymentName}, deployment); e != nil {
		return e
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: r.ACKInstallNamespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, cli, cm, func() error {
		cm.ObjectMeta.Labels = buildDBaaSLabels()
		cm.ObjectMeta.Annotations = r.buildDBaaSAnnotations(&cm.ObjectMeta)
		if len(deployment.GetObjectKind().GroupVersionKind().GroupKind().Kind) > 0 {
			if e := ophandler.SetOwnerAnnotations(deployment, cm); e != nil {
				return e
			}
		}
		cm.Data = map[string]string{
			awsEndpointUrl:              "",
			ackEnableDevelopmentLogging: "false",
			ackWatchNamespace:           "",
			ackLogLevel:                 "info",
			ackResourceTags:             "rhoda",
		}
		if credentialsRef != nil {
			cm.Data[awsRegion] = string(credentialsRef.Data[awsRegion])
			if l, ok := credentialsRef.Data[ackLogLevel]; ok {
				cm.Data[ackLogLevel] = string(l)
			}
			if t, ok := credentialsRef.Data[ackResourceTags]; ok {
				cm.Data[ackResourceTags] = string(t)
			}
		} else {
			cm.Data[awsRegion] = "dummy"
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func buildDBaaSLabels() map[string]string {
	return map[string]string{
		dbaasv1beta1.TypeLabelKey: dbaasv1beta1.TypeLabelValue,
	}
}

func (r *RDSInventoryReconciler) buildDBaaSAnnotations(obj *metav1.ObjectMeta) map[string]string {
	annotations := map[string]string{}
	if obj.Annotations != nil {
		for key, value := range obj.Annotations {
			annotations[key] = value
		}
	}
	annotations["managed-by"] = "rds-dbaas-operator"
	annotations["owner"] = ackDeploymentName
	annotations["owner.kind"] = "Deployment"
	annotations["owner.namespace"] = r.ACKInstallNamespace
	return annotations
}

func (r *RDSInventoryReconciler) installCRD(ctx context.Context, cli client.Client, file []byte) error {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := unmarshalYaml(file, crd); err != nil {
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

func createAdoptedResource(resourceIdentifier *string, resourceArn *string, engine *string, resourceKind string,
	inventory *rdsdbaasv1alpha1.RDSInventory) *ackv1alpha1.AdoptedResource {
	arn := ackv1alpha1.AWSResourceName(*resourceArn)
	return &ackv1alpha1.AdoptedResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: inventory.Namespace,
			Name:      fmt.Sprintf("rhoda-adopted-%s%s-%s", getDBEngineAbbreviation(engine), strings.ToLower(*resourceIdentifier), uuid.NewUUID()),
			Annotations: map[string]string{
				"managed-by":      "rds-dbaas-operator",
				"owner":           inventory.Name,
				"owner.kind":      inventory.Kind,
				"owner.namespace": inventory.Namespace,
			},
			Labels: map[string]string{
				adoptedDBResourceLabelKey: adoptedDBResourceLabelValue,
			},
		},
		Spec: ackv1alpha1.AdoptedResourceSpec{
			Kubernetes: &ackv1alpha1.ResourceWithMetadata{
				GroupKind: metav1.GroupKind{
					Group: rdsv1alpha1.GroupVersion.Group,
					Kind:  resourceKind,
				},
				Metadata: &ackv1alpha1.PartialObjectMeta{
					Namespace: inventory.Namespace,
					Labels: map[string]string{
						adoptedDBResourceLabelKey: adoptedDBResourceLabelValue,
					},
				},
			},
			AWS: &ackv1alpha1.AWSIdentifiers{
				NameOrID: *resourceIdentifier,
				ARN:      &arn,
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

	if err := r.installCRD(ctx, cli, adoptedResourceCRDYaml); err != nil {
		return err
	}
	if err := r.installCRD(ctx, cli, fieldExportCRDYaml); err != nil {
		return err
	}

	if err := r.createOrUpdateSecret(ctx, cli, nil); err != nil {
		return err
	}
	if err := r.createOrUpdateConfigMap(ctx, cli, nil); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&rdsdbaasv1alpha1.RDSInventory{}).
		Watches(
			&source.Kind{Type: &rdsv1alpha1.DBInstance{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				return getRDSObjectInventoryRequests(o, mgr)
			}),
		).
		Watches(
			&source.Kind{Type: &rdsv1alpha1.DBCluster{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				return getRDSObjectInventoryRequests(o, mgr)
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

func getRDSObjectInventoryRequests(object client.Object, mgr ctrl.Manager) []reconcile.Request {
	ctx := context.Background()
	logger := log.FromContext(ctx)
	cli := mgr.GetClient()

	inventoryList := &rdsdbaasv1alpha1.RDSInventoryList{}
	if e := cli.List(ctx, inventoryList, client.InNamespace(object.GetNamespace())); e != nil {
		logger.Error(e, "Failed to get Inventories for RDS object update", "RDS Object", object.GetName())
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

		if len(inventoryList.Items) > 0 {
			for _, i := range inventoryList.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: i.Namespace,
						Name:      i.Name,
					},
				})
			}
		} else {
			// Deployment of the ACK controller is updated but no Inventory exists, scale down the Deployment
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      "",
				},
			})
		}
	}
	return requests
}
