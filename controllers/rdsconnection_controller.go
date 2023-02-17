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

type serviceInfo struct {
	passwordSecret   *ackv1alpha1.SecretKeyReference
	username         *string
	host             *string
	port             *int64
	engine           *string
	dbName           *string
	masterUserSecret *v1.Secret
}

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

	var connection rdsdbaasv1alpha1.RDSConnection
	var inventory rdsdbaasv1alpha1.RDSInventory

	if err = r.Get(ctx, req.NamespacedName, &connection); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("RDS Connection resource not found, has been deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Error fetching RDS Connection for reconcile")
		return ctrl.Result{}, err
	}

	defer func() {
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
	}()

	if e := r.Get(ctx, client.ObjectKey{Namespace: connection.Spec.InventoryRef.Namespace,
		Name: connection.Spec.InventoryRef.Name}, &inventory); e != nil {
		if errors.IsNotFound(e) {
			logger.Info("RDS Inventory resource not found, may have been deleted")
			r.setConnectionReadyCondition(&connection, string(metav1.ConditionFalse),
				connectionStatusReasonNotFound, connectionStatusMessageInventoryNotFound)
			return ctrl.Result{}, e
		}
		logger.Error(e, "Failed to get RDS Inventory")
		r.setConnectionReadyCondition(&connection, string(metav1.ConditionFalse),
			connectionStatusReasonBackendError, connectionStatusMessageGetInventoryError)
		return ctrl.Result{}, e
	}

	if condition := apimeta.FindStatusCondition(inventory.Status.Conditions, inventoryConditionReady); condition == nil || condition.Status != metav1.ConditionTrue {
		logger.Info("RDS Inventory not ready")
		r.setConnectionReadyCondition(&connection, string(metav1.ConditionFalse),
			connectionStatusReasonUnreachable, connectionStatusMessageInventoryNotReady)
		return ctrl.Result{Requeue: true}, nil
	}

	dbService, e := r.checkDBServiceStatus(ctx, &connection, &inventory)
	if e != nil {
		return ctrl.Result{}, e
	}

	info, e := r.checkDBConnectionStatus(ctx, &connection, dbService)
	if e != nil {
		return ctrl.Result{}, e
	}

	if r, e := r.syncConnectionStatus(ctx, &connection, info); r {
		return ctrl.Result{Requeue: true}, nil
	} else if e != nil {
		return ctrl.Result{}, e
	}

	r.setConnectionReadyCondition(&connection, string(metav1.ConditionTrue),
		connectionStatusReasonReady, "")
	return
}

func (r *RDSConnectionReconciler) setConnectionReadyCondition(connection *rdsdbaasv1alpha1.RDSConnection,
	bindingStatus, bindingStatusReason, bindingStatusMessage string) {
	condition := metav1.Condition{
		Type:    connectionConditionReady,
		Status:  metav1.ConditionStatus(bindingStatus),
		Reason:  bindingStatusReason,
		Message: bindingStatusMessage,
	}
	apimeta.SetStatusCondition(&connection.Status.Conditions, condition)
}

func (r *RDSConnectionReconciler) checkDBServiceStatus(ctx context.Context, connection *rdsdbaasv1alpha1.RDSConnection,
	inventory *rdsdbaasv1alpha1.RDSInventory) (client.Object, error) {
	logger := log.FromContext(ctx)

	var dbService client.Object

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
		r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
			connectionStatusReasonNotFound, connectionStatusMessageServiceNotFound)
		return nil, e
	}

	if connection.Spec.DatabaseServiceType != nil && *connection.Spec.DatabaseServiceType == clusterType {
		dbService = &rdsv1alpha1.DBCluster{}
	} else {
		dbService = &rdsv1alpha1.DBInstance{}
	}

	if e := r.Get(ctx, client.ObjectKey{Namespace: connection.Spec.InventoryRef.Namespace,
		Name: *serviceName}, dbService); e != nil {
		logger.Error(e, "Failed to get DB Service")
		r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
			connectionStatusReasonBackendError, connectionStatusMessageGetServiceError)
		return nil, e
	}

	switch s := dbService.(type) {
	case *rdsv1alpha1.DBCluster:
		if s.Status.Status == nil || *s.Status.Status != "available" {
			e := fmt.Errorf("cluster %s not ready", connection.Spec.DatabaseServiceID)
			logger.Error(e, "DB Cluster not ready")
			r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
				connectionStatusReasonUnreachable, connectionStatusMessageServiceNotReady)
			return nil, e
		}
	case *rdsv1alpha1.DBInstance:
		if s.Status.DBInstanceStatus == nil || *s.Status.DBInstanceStatus != "available" {
			e := fmt.Errorf("instance %s not ready", connection.Spec.DatabaseServiceID)
			logger.Error(e, "DB Instance not ready")
			r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
				connectionStatusReasonUnreachable, connectionStatusMessageServiceNotReady)
			return nil, e
		}
	default:
		e := fmt.Errorf("DB service %s not valid", connection.Spec.DatabaseServiceID)
		logger.Error(e, "DB service not valid")
		r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
			connectionStatusReasonUnreachable, connectionStatusMessageServiceNotValid)
		return nil, e
	}

	return dbService, nil
}

func (r *RDSConnectionReconciler) checkDBConnectionStatus(ctx context.Context, connection *rdsdbaasv1alpha1.RDSConnection,
	dbService client.Object) (*serviceInfo, error) {
	logger := log.FromContext(ctx)

	info := &serviceInfo{}
	switch s := dbService.(type) {
	case *rdsv1alpha1.DBCluster:
		info.engine = s.Spec.Engine
		info.passwordSecret = s.Spec.MasterUserPassword
		info.username = s.Spec.MasterUsername
		info.host = s.Status.Endpoint
		info.port = s.Spec.Port
		info.dbName = s.Spec.DatabaseName
	case *rdsv1alpha1.DBInstance:
		info.engine = s.Spec.Engine
		info.passwordSecret = s.Spec.MasterUserPassword
		info.username = s.Spec.MasterUsername
		if s.Status.Endpoint != nil {
			info.host = s.Status.Endpoint.Address
			info.port = s.Status.Endpoint.Port
		} else {
			info.host = nil
			info.port = nil
		}
		info.dbName = s.Spec.DBName
	}

	if info.passwordSecret == nil {
		e := fmt.Errorf("service %s master password not set", connection.Spec.DatabaseServiceID)
		logger.Error(e, "DB Service master password not set")
		r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
			connectionStatusReasonInputError, connectionStatusMessagePasswordNotFound)
		return nil, e
	}

	info.masterUserSecret = &v1.Secret{}
	if e := r.Get(ctx, client.ObjectKey{Namespace: info.passwordSecret.Namespace, Name: info.passwordSecret.Name}, info.masterUserSecret); e != nil {
		logger.Error(e, "Failed to get secret for DB Service master password")
		if errors.IsNotFound(e) {
			r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
				connectionStatusReasonNotFound, connectionStatusMessageGetPasswordError)
		} else {
			r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
				connectionStatusReasonBackendError, connectionStatusMessageGetPasswordError)
		}
		return nil, e
	}
	if v, ok := info.masterUserSecret.Data[info.passwordSecret.Key]; !ok || len(v) == 0 {
		e := fmt.Errorf("service %s master password key not set", connection.Spec.DatabaseServiceID)
		logger.Error(e, "DB Service master password key not set")
		r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
			connectionStatusReasonInputError, connectionStatusMessagePasswordInvalid)
		return nil, e
	}
	if info.username == nil {
		e := fmt.Errorf("service %s master username not set", connection.Spec.DatabaseServiceID)
		logger.Error(e, "DB Service master username not set")
		r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
			connectionStatusReasonInputError, connectionStatusMessageUsernameNotFound)
		return nil, e
	}

	if info.host == nil || info.port == nil {
		e := fmt.Errorf("service %s endpoint not found", connection.Spec.DatabaseServiceID)
		logger.Error(e, "DB Service endpoint not found")
		r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
			connectionStatusReasonUnreachable, connectionStatusMessageEndpointNotFound)
		return nil, e
	}
	return info, nil
}

func (r *RDSConnectionReconciler) syncConnectionStatus(ctx context.Context, connection *rdsdbaasv1alpha1.RDSConnection,
	info *serviceInfo) (bool, error) {
	logger := log.FromContext(ctx)

	userSecret, e := r.createOrUpdateSecret(ctx, connection, info.username, info.masterUserSecret.Data[info.passwordSecret.Key])
	if e != nil {
		logger.Error(e, "Failed to create or update secret for Connection")
		r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
			connectionStatusReasonBackendError, connectionStatusMessageSecretError)
		return false, e
	}

	dbConfigMap, e := r.createOrUpdateConfigMap(ctx, connection, info.engine, info.dbName, info.host, info.port)
	if e != nil {
		logger.Error(e, "Failed to create or update configmap for Connection")
		r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
			connectionStatusReasonBackendError, connectionStatusMessageConfigMapError)
		return false, e
	}

	connection.Status.CredentialsRef = &v1.LocalObjectReference{Name: userSecret.Name}
	connection.Status.ConnectionInfoRef = &v1.LocalObjectReference{Name: dbConfigMap.Name}
	if e := r.Status().Update(ctx, connection); e != nil {
		if errors.IsConflict(e) {
			logger.Info("Connection modified, retry reconciling")
			r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
				connectionStatusReasonUpdating, connectionStatusMessageUpdating)
			return true, nil
		}
		logger.Error(e, "Failed to update Connection status")
		r.setConnectionReadyCondition(connection, string(metav1.ConditionFalse),
			connectionStatusReasonBackendError, connectionStatusMessageUpdateError)
		return false, e
	}
	return false, nil
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
