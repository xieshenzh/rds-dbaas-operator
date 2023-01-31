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
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	dbaasv1beta1 "github.com/RHEcosystemAppEng/dbaas-operator/api/v1beta1"
	rdsdbaasv1alpha1 "github.com/RHEcosystemAppEng/rds-dbaas-operator/api/v1alpha1"
	rdsv1alpha1 "github.com/aws-controllers-k8s/rds-controller/apis/v1alpha1"
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
)

const (
	databaseServiceIDKey = ".spec.databaseServiceID"

	databaseProvider = "Red Hat DBaaS / Amazon Relational Database Service (RDS)"

	connectionConditionReady = "ReadyForBinding"

	connectionStatusReasonReady        = "Ready"
	connectionStatusReasonUpdating     = "Updating"
	connectionStatusReasonBackendError = "BackendError"
	connectionStatusReasonInputError   = "InputError"
	connectionStatusReasonNotFound     = "NotFound"
	connectionStatusReasonUnreachable  = "Unreachable"

	connectionStatusMessageUpdateError       = "Failed to update Connection"
	connectionStatusMessageUpdating          = "Updating Connection"
	connectionStatusMessageSecretError       = "Failed to create or update secret"
	connectionStatusMessageConfigMapError    = "Failed to create or update configmap"
	connectionStatusMessageServiceNotFound   = "Database service not found"
	connectionStatusMessageServiceNotReady   = "Database service not ready"
	connectionStatusMessageServiceNotValid   = "Database service not valid"
	connectionStatusMessageGetServiceError   = "Failed to get Database service"
	connectionStatusMessagePasswordNotFound  = "Password not found"
	connectionStatusMessagePasswordInvalid   = "Password invalid"
	connectionStatusMessageUsernameNotFound  = "Username not found"
	connectionStatusMessageEndpointNotFound  = "Endpoint not found"
	connectionStatusMessageGetPasswordError  = "Failed to get secret for password" //#nosec G101
	connectionStatusMessageInventoryNotFound = "Inventory not found"
	connectionStatusMessageInventoryNotReady = "Inventory not ready"
	connectionStatusMessageGetInventoryError = "Failed to get Inventory"
)

// RDSConnectionReconciler reconciles a RDSConnection object
type RDSConnectionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsconnections,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsconnections/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.redhat.com,resources=rdsconnections/finalizers,verbs=update
//+kubebuilder:rbac:groups=rds.services.k8s.aws,resources=dbinstances,verbs=get;list;watch
//+kubebuilder:rbac:groups=rds.services.k8s.aws,resources=dbclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets;configmaps,verbs=get;list;watch;create;delete;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *RDSConnectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	var bindingStatus, bindingStatusReason, bindingStatusMessage string

	var connection rdsdbaasv1alpha1.RDSConnection
	var inventory rdsdbaasv1alpha1.RDSInventory
	var dbService client.Object

	var passwordSecret *ackv1alpha1.SecretKeyReference
	var username *string
	var host *string
	var port *int64
	var engine *string
	var dbName *string

	var masterUserSecret v1.Secret

	returnError := func(e error, reason, message string) {
		result = ctrl.Result{}
		err = e
		bindingStatus = string(metav1.ConditionFalse)
		bindingStatusReason = reason
		bindingStatusMessage = message
	}

	returnRequeue := func(reason, message string) {
		result = ctrl.Result{Requeue: true}
		err = nil
		bindingStatus = string(metav1.ConditionFalse)
		bindingStatusReason = reason
		bindingStatusMessage = message
	}

	returnReady := func() {
		result = ctrl.Result{}
		err = nil
		bindingStatus = string(metav1.ConditionTrue)
		bindingStatusReason = connectionStatusReasonReady
	}

	updateConnectionReadyCondition := func() {
		condition := metav1.Condition{
			Type:    connectionConditionReady,
			Status:  metav1.ConditionStatus(bindingStatus),
			Reason:  bindingStatusReason,
			Message: bindingStatusMessage,
		}
		apimeta.SetStatusCondition(&connection.Status.Conditions, condition)
		if e := r.Status().Update(ctx, &connection); e != nil {
			if errors.IsConflict(e) {
				logger.Info("Connection modified, retry reconciling")
				result = ctrl.Result{Requeue: true}
			} else {
				logger.Error(e, "Failed to update Connection status")
				if err == nil {
					err = e
				}
			}
		}
	}

	checkDBServiceStatus := func() bool {
		var serviceName *string
		var cType string
		if connection.Spec.DatabaseServiceType != nil {
			cType = string(*connection.Spec.DatabaseServiceType)
		} else {
			cType = instanceType
		}
		for _, ds := range inventory.Status.DatabaseServices {
			if ds.ServiceID == connection.Spec.DatabaseServiceID {
				var sType string
				if ds.ServiceType != nil {
					sType = string(*ds.ServiceType)
				} else {
					sType = instanceType
				}
				if cType == sType {
					serviceName = &ds.ServiceName
					break
				}
			}
		}
		if serviceName == nil {
			var e error
			if connection.Spec.DatabaseServiceType != nil {
				e = fmt.Errorf("database service %s type %v not found", connection.Spec.DatabaseServiceID, *connection.Spec.DatabaseServiceType)
			} else {
				e = fmt.Errorf("database service %s not found", connection.Spec.DatabaseServiceID)
			}
			logger.Error(e, "DB Service not found from Inventory")
			returnError(e, connectionStatusReasonNotFound, connectionStatusMessageServiceNotFound)
			return true
		}

		if connection.Spec.DatabaseServiceType != nil && *connection.Spec.DatabaseServiceType == clusterType {
			dbService = &rdsv1alpha1.DBCluster{}
		} else {
			dbService = &rdsv1alpha1.DBInstance{}
		}

		if e := r.Get(ctx, client.ObjectKey{Namespace: connection.Spec.InventoryRef.Namespace,
			Name: *serviceName}, dbService); e != nil {
			logger.Error(e, "Failed to get DB Service")
			returnError(e, connectionStatusReasonBackendError, connectionStatusMessageGetServiceError)
			return true
		}

		switch s := dbService.(type) {
		case *rdsv1alpha1.DBCluster:
			if s.Status.Status == nil || *s.Status.Status != "available" {
				e := fmt.Errorf("cluster %s not ready", connection.Spec.DatabaseServiceID)
				logger.Error(e, "DB Cluster not ready")
				returnError(e, connectionStatusReasonUnreachable, connectionStatusMessageServiceNotReady)
				return true
			}
		case *rdsv1alpha1.DBInstance:
			if s.Status.DBInstanceStatus == nil || *s.Status.DBInstanceStatus != "available" {
				e := fmt.Errorf("instance %s not ready", connection.Spec.DatabaseServiceID)
				logger.Error(e, "DB Instance not ready")
				returnError(e, connectionStatusReasonUnreachable, connectionStatusMessageServiceNotReady)
				return true
			}
		default:
			e := fmt.Errorf("DB service %s not valid", connection.Spec.DatabaseServiceID)
			logger.Error(e, "DB service not valid")
			returnError(e, connectionStatusReasonUnreachable, connectionStatusMessageServiceNotValid)
			return true
		}

		return false
	}

	checkDBConnectionStatus := func() bool {
		switch s := dbService.(type) {
		case *rdsv1alpha1.DBCluster:
			engine = s.Spec.Engine
			passwordSecret = s.Spec.MasterUserPassword
			username = s.Spec.MasterUsername
			host = s.Status.Endpoint
			port = s.Spec.Port
			dbName = s.Spec.DatabaseName
		case *rdsv1alpha1.DBInstance:
			engine = s.Spec.Engine
			passwordSecret = s.Spec.MasterUserPassword
			username = s.Spec.MasterUsername
			if s.Status.Endpoint != nil {
				host = s.Status.Endpoint.Address
				port = s.Status.Endpoint.Port
			} else {
				host = nil
				port = nil
			}
			dbName = s.Spec.DBName
		}

		if passwordSecret == nil {
			e := fmt.Errorf("service %s master password not set", connection.Spec.DatabaseServiceID)
			logger.Error(e, "DB Service master password not set")
			returnError(e, connectionStatusReasonInputError, connectionStatusMessagePasswordNotFound)
			return true
		}

		if e := r.Get(ctx, client.ObjectKey{Namespace: passwordSecret.Namespace, Name: passwordSecret.Name}, &masterUserSecret); e != nil {
			logger.Error(e, "Failed to get secret for DB Service master password")
			if errors.IsNotFound(e) {
				returnError(e, connectionStatusReasonNotFound, connectionStatusMessageGetPasswordError)
			} else {
				returnError(e, connectionStatusReasonBackendError, connectionStatusMessageGetPasswordError)
			}
			return true
		}
		if v, ok := masterUserSecret.Data[passwordSecret.Key]; !ok || len(v) == 0 {
			e := fmt.Errorf("service %s master password key not set", connection.Spec.DatabaseServiceID)
			logger.Error(e, "DB Service master password key not set")
			returnError(e, connectionStatusReasonInputError, connectionStatusMessagePasswordInvalid)
			return true
		}
		if username == nil {
			e := fmt.Errorf("service %s master username not set", connection.Spec.DatabaseServiceID)
			logger.Error(e, "DB Service master username not set")
			returnError(e, connectionStatusReasonInputError, connectionStatusMessageUsernameNotFound)
			return true
		}

		if host == nil || port == nil {
			e := fmt.Errorf("service %s endpoint not found", connection.Spec.DatabaseServiceID)
			logger.Error(e, "DB Service endpoint not found")
			returnError(e, connectionStatusReasonUnreachable, connectionStatusMessageEndpointNotFound)
			return true
		}
		return false
	}

	syncConnectionStatus := func() bool {
		userSecret, e := r.createOrUpdateSecret(ctx, &connection, username, masterUserSecret.Data[passwordSecret.Key])
		if e != nil {
			logger.Error(e, "Failed to create or update secret for Connection")
			returnError(e, connectionStatusReasonBackendError, connectionStatusMessageSecretError)
			return true
		}

		dbConfigMap, e := r.createOrUpdateConfigMap(ctx, &connection, engine, dbName, host, port)
		if e != nil {
			logger.Error(e, "Failed to create or update configmap for Connection")
			returnError(e, connectionStatusReasonBackendError, connectionStatusMessageConfigMapError)
			return true
		}

		connection.Status.CredentialsRef = &v1.LocalObjectReference{Name: userSecret.Name}
		connection.Status.ConnectionInfoRef = &v1.LocalObjectReference{Name: dbConfigMap.Name}
		if e := r.Status().Update(ctx, &connection); e != nil {
			if errors.IsConflict(e) {
				logger.Info("Connection modified, retry reconciling")
				returnRequeue(connectionStatusReasonUpdating, connectionStatusMessageUpdating)
				return true
			}
			logger.Error(e, "Failed to update Connection status")
			returnError(e, connectionStatusReasonBackendError, connectionStatusMessageUpdateError)
			return true
		}
		return false
	}

	if err = r.Get(ctx, req.NamespacedName, &connection); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("RDS Connection resource not found, has been deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Error fetching RDS Connection for reconcile")
		return ctrl.Result{}, err
	}

	defer updateConnectionReadyCondition()

	if e := r.Get(ctx, client.ObjectKey{Namespace: connection.Spec.InventoryRef.Namespace,
		Name: connection.Spec.InventoryRef.Name}, &inventory); e != nil {
		if errors.IsNotFound(e) {
			logger.Info("RDS Inventory resource not found, may have been deleted")
			returnError(e, connectionStatusReasonNotFound, connectionStatusMessageInventoryNotFound)
			return
		}
		logger.Error(e, "Failed to get RDS Inventory")
		returnError(e, connectionStatusReasonBackendError, connectionStatusMessageGetInventoryError)
		return
	}

	if condition := apimeta.FindStatusCondition(inventory.Status.Conditions, inventoryConditionReady); condition == nil || condition.Status != metav1.ConditionTrue {
		logger.Info("RDS Inventory not ready")
		returnRequeue(connectionStatusReasonUnreachable, connectionStatusMessageInventoryNotReady)
		return
	}

	if checkDBServiceStatus() {
		return
	}

	if checkDBConnectionStatus() {
		return
	}

	if syncConnectionStatus() {
		return
	}

	returnReady()
	return
}

func (r *RDSConnectionReconciler) createOrUpdateSecret(ctx context.Context, connection *rdsdbaasv1alpha1.RDSConnection,
	username *string, password []byte) (*v1.Secret, error) {
	secretName := fmt.Sprintf("%s-credentials", connection.Name)
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: connection.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		secret.ObjectMeta.Labels = buildConnectionLabels()
		secret.ObjectMeta.Annotations = buildConnectionAnnotations(connection, &secret.ObjectMeta)
		if err := ctrl.SetControllerReference(connection, secret, r.Scheme); err != nil {
			return err
		}
		setSecret(secret, username, password)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func setSecret(secret *v1.Secret, username *string, password []byte) {
	data := map[string][]byte{
		"username": []byte(*username),
		"password": password,
	}
	secret.Data = data
}

func (r *RDSConnectionReconciler) createOrUpdateConfigMap(ctx context.Context, connection *rdsdbaasv1alpha1.RDSConnection,
	engine *string, dbName *string, host *string, port *int64) (*v1.ConfigMap, error) {
	cmName := fmt.Sprintf("%s-configs", connection.Name)
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: connection.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.ObjectMeta.Labels = buildConnectionLabels()
		cm.ObjectMeta.Annotations = buildConnectionAnnotations(connection, &cm.ObjectMeta)
		if err := ctrl.SetControllerReference(connection, cm, r.Scheme); err != nil {
			return err
		}
		setConfigMap(cm, engine, dbName, host, port)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cm, nil
}

func setConfigMap(cm *v1.ConfigMap, engine *string, dbName *string, host *string, port *int64) {
	dataMap := map[string]string{
		"type":     generateBindingType(*engine),
		"provider": databaseProvider,
		"host":     *host,
	}

	if dbName != nil {
		dataMap["database"] = *dbName
	} else if engine != nil {
		if dbn := getDefaultDBName(*engine); dbn != nil {
			dataMap["database"] = *dbn
		}
	}

	if port != nil {
		dataMap["port"] = strconv.FormatInt(*port, 10)
	} else if engine != nil {
		if p := getDefaultDBPort(*engine); p != nil {
			dataMap["port"] = strconv.FormatInt(*p, 10)
		}
	}

	cm.Data = dataMap
}

func buildConnectionLabels() map[string]string {
	return map[string]string{
		dbaasv1beta1.TypeLabelKey: dbaasv1beta1.TypeLabelValue,
	}
}

func buildConnectionAnnotations(connection *rdsdbaasv1alpha1.RDSConnection, obj *metav1.ObjectMeta) map[string]string {
	annotations := map[string]string{}
	if obj.Annotations != nil {
		for key, value := range obj.Annotations {
			annotations[key] = value
		}
	}
	annotations["managed-by"] = "rds-dbaas-operator"
	annotations["owner"] = connection.Name
	annotations["owner.kind"] = connection.Kind
	annotations["owner.namespace"] = connection.Namespace
	return annotations
}

// SetupWithManager sets up the controller with the Manager.
func (r *RDSConnectionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&rdsdbaasv1alpha1.RDSConnection{}).
		Watches(
			&source.Kind{Type: &rdsv1alpha1.DBInstance{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				return getInstanceConnectionRequests(o, mgr)
			}),
		).
		Watches(
			&source.Kind{Type: &rdsv1alpha1.DBCluster{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				return getInstanceConnectionRequests(o, mgr)
			}),
		).
		Complete(r); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &rdsdbaasv1alpha1.RDSConnection{}, databaseServiceIDKey, func(rawObj client.Object) []string {
		connection := rawObj.(*rdsdbaasv1alpha1.RDSConnection)
		instanceID := connection.Spec.DatabaseServiceID
		return []string{instanceID}
	}); err != nil {
		return err
	}

	return nil
}

func getInstanceConnectionRequests(object client.Object, mgr ctrl.Manager) []reconcile.Request {
	ctx := context.Background()
	logger := log.FromContext(ctx)
	cli := mgr.GetClient()

	connectionList := &rdsdbaasv1alpha1.RDSConnectionList{}
	var namespace string
	switch s := object.(type) {
	case *rdsv1alpha1.DBCluster:
		if e := cli.List(ctx, connectionList, client.MatchingFields{databaseServiceIDKey: *s.Spec.DBClusterIdentifier}); e != nil {
			logger.Error(e, "Failed to get Connections for DB Cluster update", "DBCluster ID", s.Spec.DBClusterIdentifier)
			return nil
		}
		namespace = s.Namespace
	case *rdsv1alpha1.DBInstance:
		if e := cli.List(ctx, connectionList, client.MatchingFields{databaseServiceIDKey: *s.Spec.DBInstanceIdentifier}); e != nil {
			logger.Error(e, "Failed to get Connections for DB Instance update", "DBInstance ID", s.Spec.DBInstanceIdentifier)
			return nil
		}
		namespace = s.Namespace
	}

	var requests []reconcile.Request
	for _, c := range connectionList.Items {
		match := false
		if len(c.Spec.InventoryRef.Namespace) > 0 {
			if c.Spec.InventoryRef.Namespace == namespace {
				match = true
			}
		} else {
			if c.Namespace == namespace {
				match = true
			}
		}
		if match {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: c.Namespace,
					Name:      c.Name,
				},
			})
		}
	}
	return requests
}
