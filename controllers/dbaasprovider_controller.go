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
	"os"
	"strconv"
	"time"

	v1 "k8s.io/api/apps/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	label "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dbaasoperator "github.com/RHEcosystemAppEng/dbaas-operator/api/v1alpha1"
)

const (
	providerKind   = "DBaaSProvider"
	providerCRName = "rds-registration"

	inventoryKind  = "RDSInventory"
	connectionKind = "RDSConnection"
	instanceKind   = "RDSInstance"

	relatedToLabelName  = "related-to"
	relatedToLabelValue = "dbaas-operator"
	typeLabelName       = "type"
	typeLabelValue      = "dbaas-provider-registration"

	provisionDocURL      = "https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/"
	provisionDescription = "Amazon Relational Database Service (Amazon RDS) is a web service that makes it easier to set up, operate, and scale a relational database in the AWS Cloud. It provides cost-efficient, resizable capacity for an industry-standard relational database and manages common database administration tasks."

	provider           = "Red Hat DBaaS / Amazon Relational Database Service"
	displayName        = "Amazon Relational Database Service"
	displayDescription = "Amazon Relational Database Service (RDS) is a collection of managed services that makes it simple to set up, operate, and scale databases in the cloud."
	iconData           = "PD94bWwgdmVyc2lvbj0iMS4wIiBlbmNvZGluZz0iVVRGLTgiPz4KPCEtLSBHZW5lcmF0b3I6IEFkb2JlIElsbHVzdHJhdG9yIDE5LjAuMSwgU1ZHIEV4cG9ydCBQbHVnLUluIC4gU1ZHIFZlcnNpb246IDYuMDAgQnVpbGQgMCkgIC0tPgo8c3ZnIHZlcnNpb249IjEuMSIgaWQ9IkxheWVyXzEiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIgeG1sbnM6eGxpbms9Imh0dHA6Ly93d3cudzMub3JnLzE5OTkveGxpbmsiIHg9IjBweCIgeT0iMHB4IiB2aWV3Qm94PSIwIDAgMzA0IDE4MiIgc3R5bGU9ImVuYWJsZS1iYWNrZ3JvdW5kOm5ldyAwIDAgMzA0IDE4MjsiIHhtbDpzcGFjZT0icHJlc2VydmUiPgo8c3R5bGUgdHlwZT0idGV4dC9jc3MiPgoJLnN0MHtmaWxsOiMyNTJGM0U7fQoJLnN0MXtmaWxsLXJ1bGU6ZXZlbm9kZDtjbGlwLXJ1bGU6ZXZlbm9kZDtmaWxsOiNGRjk5MDA7fQo8L3N0eWxlPgo8Zz4KCTxwYXRoIGNsYXNzPSJzdDAiIGQ9Ik04Ni40LDY2LjRjMCwzLjcsMC40LDYuNywxLjEsOC45YzAuOCwyLjIsMS44LDQuNiwzLjIsNy4yYzAuNSwwLjgsMC43LDEuNiwwLjcsMi4zYzAsMS0wLjYsMi0xLjksM2wtNi4zLDQuMiAgIGMtMC45LDAuNi0xLjgsMC45LTIuNiwwLjljLTEsMC0yLTAuNS0zLTEuNEM3Ni4yLDkwLDc1LDg4LjQsNzQsODYuOGMtMS0xLjctMi0zLjYtMy4xLTUuOWMtNy44LDkuMi0xNy42LDEzLjgtMjkuNCwxMy44ICAgYy04LjQsMC0xNS4xLTIuNC0yMC03LjJjLTQuOS00LjgtNy40LTExLjItNy40LTE5LjJjMC04LjUsMy0xNS40LDkuMS0yMC42YzYuMS01LjIsMTQuMi03LjgsMjQuNS03LjhjMy40LDAsNi45LDAuMywxMC42LDAuOCAgIGMzLjcsMC41LDcuNSwxLjMsMTEuNSwyLjJ2LTcuM2MwLTcuNi0xLjYtMTIuOS00LjctMTZjLTMuMi0zLjEtOC42LTQuNi0xNi4zLTQuNmMtMy41LDAtNy4xLDAuNC0xMC44LDEuM2MtMy43LDAuOS03LjMsMi0xMC44LDMuNCAgIGMtMS42LDAuNy0yLjgsMS4xLTMuNSwxLjNjLTAuNywwLjItMS4yLDAuMy0xLjYsMC4zYy0xLjQsMC0yLjEtMS0yLjEtMy4xdi00LjljMC0xLjYsMC4yLTIuOCwwLjctMy41YzAuNS0wLjcsMS40LTEuNCwyLjgtMi4xICAgYzMuNS0xLjgsNy43LTMuMywxMi42LTQuNWM0LjktMS4zLDEwLjEtMS45LDE1LjYtMS45YzExLjksMCwyMC42LDIuNywyNi4yLDguMWM1LjUsNS40LDguMywxMy42LDguMywyNC42VjY2LjR6IE00NS44LDgxLjYgICBjMy4zLDAsNi43LTAuNiwxMC4zLTEuOGMzLjYtMS4yLDYuOC0zLjQsOS41LTYuNGMxLjYtMS45LDIuOC00LDMuNC02LjRjMC42LTIuNCwxLTUuMywxLTguN3YtNC4yYy0yLjktMC43LTYtMS4zLTkuMi0xLjcgICBjLTMuMi0wLjQtNi4zLTAuNi05LjQtMC42Yy02LjcsMC0xMS42LDEuMy0xNC45LDRjLTMuMywyLjctNC45LDYuNS00LjksMTEuNWMwLDQuNywxLjIsOC4yLDMuNywxMC42ICAgQzM3LjcsODAuNCw0MS4yLDgxLjYsNDUuOCw4MS42eiBNMTI2LjEsOTIuNGMtMS44LDAtMy0wLjMtMy44LTFjLTAuOC0wLjYtMS41LTItMi4xLTMuOUw5Ni43LDEwLjJjLTAuNi0yLTAuOS0zLjMtMC45LTQgICBjMC0xLjYsMC44LTIuNSwyLjQtMi41aDkuOGMxLjksMCwzLjIsMC4zLDMuOSwxYzAuOCwwLjYsMS40LDIsMiwzLjlsMTYuOCw2Ni4ybDE1LjYtNjYuMmMwLjUtMiwxLjEtMy4zLDEuOS0zLjljMC44LTAuNiwyLjItMSw0LTEgICBoOGMxLjksMCwzLjIsMC4zLDQsMWMwLjgsMC42LDEuNSwyLDEuOSwzLjlsMTUuOCw2N2wxNy4zLTY3YzAuNi0yLDEuMy0zLjMsMi0zLjljMC44LTAuNiwyLjEtMSwzLjktMWg5LjNjMS42LDAsMi41LDAuOCwyLjUsMi41ICAgYzAsMC41LTAuMSwxLTAuMiwxLjZjLTAuMSwwLjYtMC4zLDEuNC0wLjcsMi41bC0yNC4xLDc3LjNjLTAuNiwyLTEuMywzLjMtMi4xLDMuOWMtMC44LDAuNi0yLjEsMS0zLjgsMWgtOC42Yy0xLjksMC0zLjItMC4zLTQtMSAgIGMtMC44LTAuNy0xLjUtMi0xLjktNEwxNTYsMjNsLTE1LjQsNjQuNGMtMC41LDItMS4xLDMuMy0xLjksNGMtMC44LDAuNy0yLjIsMS00LDFIMTI2LjF6IE0yNTQuNiw5NS4xYy01LjIsMC0xMC40LTAuNi0xNS40LTEuOCAgIGMtNS0xLjItOC45LTIuNS0xMS41LTRjLTEuNi0wLjktMi43LTEuOS0zLjEtMi44Yy0wLjQtMC45LTAuNi0xLjktMC42LTIuOHYtNS4xYzAtMi4xLDAuOC0zLjEsMi4zLTMuMWMwLjYsMCwxLjIsMC4xLDEuOCwwLjMgICBjMC42LDAuMiwxLjUsMC42LDIuNSwxYzMuNCwxLjUsNy4xLDIuNywxMSwzLjVjNCwwLjgsNy45LDEuMiwxMS45LDEuMmM2LjMsMCwxMS4yLTEuMSwxNC42LTMuM2MzLjQtMi4yLDUuMi01LjQsNS4yLTkuNSAgIGMwLTIuOC0wLjktNS4xLTIuNy03Yy0xLjgtMS45LTUuMi0zLjYtMTAuMS01LjJMMjQ2LDUyYy03LjMtMi4zLTEyLjctNS43LTE2LTEwLjJjLTMuMy00LjQtNS05LjMtNS0xNC41YzAtNC4yLDAuOS03LjksMi43LTExLjEgICBjMS44LTMuMiw0LjItNiw3LjItOC4yYzMtMi4zLDYuNC00LDEwLjQtNS4yYzQtMS4yLDguMi0xLjcsMTIuNi0xLjdjMi4yLDAsNC41LDAuMSw2LjcsMC40YzIuMywwLjMsNC40LDAuNyw2LjUsMS4xICAgYzIsMC41LDMuOSwxLDUuNywxLjZjMS44LDAuNiwzLjIsMS4yLDQuMiwxLjhjMS40LDAuOCwyLjQsMS42LDMsMi41YzAuNiwwLjgsMC45LDEuOSwwLjksMy4zdjQuN2MwLDIuMS0wLjgsMy4yLTIuMywzLjIgICBjLTAuOCwwLTIuMS0wLjQtMy44LTEuMmMtNS43LTIuNi0xMi4xLTMuOS0xOS4yLTMuOWMtNS43LDAtMTAuMiwwLjktMTMuMywyLjhjLTMuMSwxLjktNC43LDQuOC00LjcsOC45YzAsMi44LDEsNS4yLDMsNy4xICAgYzIsMS45LDUuNywzLjgsMTEsNS41bDE0LjIsNC41YzcuMiwyLjMsMTIuNCw1LjUsMTUuNSw5LjZjMy4xLDQuMSw0LjYsOC44LDQuNiwxNGMwLDQuMy0wLjksOC4yLTIuNiwxMS42ICAgYy0xLjgsMy40LTQuMiw2LjQtNy4zLDguOGMtMy4xLDIuNS02LjgsNC4zLTExLjEsNS42QzI2NC40LDk0LjQsMjU5LjcsOTUuMSwyNTQuNiw5NS4xeiIvPgoJPGc+CgkJPHBhdGggY2xhc3M9InN0MSIgZD0iTTI3My41LDE0My43Yy0zMi45LDI0LjMtODAuNywzNy4yLTEyMS44LDM3LjJjLTU3LjYsMC0xMDkuNS0yMS4zLTE0OC43LTU2LjdjLTMuMS0yLjgtMC4zLTYuNiwzLjQtNC40ICAgIGM0Mi40LDI0LjYsOTQuNywzOS41LDE0OC44LDM5LjVjMzYuNSwwLDc2LjYtNy42LDExMy41LTIzLjJDMjc0LjIsMTMzLjYsMjc4LjksMTM5LjcsMjczLjUsMTQzLjd6Ii8+CgkJPHBhdGggY2xhc3M9InN0MSIgZD0iTTI4Ny4yLDEyOC4xYy00LjItNS40LTI3LjgtMi42LTM4LjUtMS4zYy0zLjIsMC40LTMuNy0yLjQtMC44LTQuNWMxOC44LTEzLjIsNDkuNy05LjQsNTMuMy01ICAgIGMzLjYsNC41LTEsMzUuNC0xOC42LDUwLjJjLTIuNywyLjMtNS4zLDEuMS00LjEtMS45QzI4Mi41LDE1NS43LDI5MS40LDEzMy40LDI4Ny4yLDEyOC4xeiIvPgoJPC9nPgo8L2c+Cjwvc3ZnPg=="
	mediaType          = "image/svg+xml"
)

var labels = map[string]string{relatedToLabelName: relatedToLabelValue, typeLabelName: typeLabelValue}

type DBaaSProviderReconciler struct {
	client.Client
	*runtime.Scheme
	Clientset                *kubernetes.Clientset
	operatorNameVersion      string
	operatorInstallNamespace string
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;create;update;delete;watch
// +kubebuilder:rbac:groups=dbaas.redhat.com,resources=dbaasproviders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dbaas.redhat.com,resources=dbaasproviders/status,verbs=get;update;patch

func (r *DBaaSProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "DBaaSProvider", req.NamespacedName, "during", "DBaaSProvider Reconciler")

	// due to predicate filtering, we'll only reconcile this operator's own deployment when it's seen the first time
	// meaning we have a reconcile entry-point on operator start-up, so now we can create a cluster-scoped resource
	// owned by the operator's ClusterRole to ensure cleanup on uninstall

	dep := &v1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, dep); err != nil {
		if errors.IsNotFound(err) {
			// CR deleted since request queued, child objects getting GC'd, no requeue
			logger.Info("deployment not found, deleted, no requeue")
			return ctrl.Result{}, nil
		}
		// error fetching deployment, requeue and try again
		logger.Error(err, "error fetching Deployment CR")
		return ctrl.Result{}, err
	}

	isCrdInstalled, err := r.checkCrdInstalled(dbaasoperator.GroupVersion.String(), providerKind)
	if err != nil {
		logger.Error(err, "error discovering GVK")
		return ctrl.Result{}, err
	}
	if !isCrdInstalled {
		logger.Info("CRD not found, requeueing with rate limiter")
		// returning with 'Requeue: true' will invoke our custom rate limiter seen in SetupWithManager below
		return ctrl.Result{Requeue: true}, nil
	}

	instance := &dbaasoperator.DBaaSProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name: providerCRName,
		},
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(instance), instance); err != nil {
		if errors.IsNotFound(err) {
			// CR deleted since request queued, child objects getting GC'd, no requeue
			logger.Info("resource not found, creating now")

			// RDS controller registration custom resource isn't present,so create now with ClusterRole owner for GC
			opts := &client.ListOptions{
				LabelSelector: label.SelectorFromSet(map[string]string{
					"olm.owner":      r.operatorNameVersion,
					"olm.owner.kind": "ClusterServiceVersion",
				}),
			}
			clusterRoleList := &rbac.ClusterRoleList{}
			if err := r.List(context.Background(), clusterRoleList, opts); err != nil {
				logger.Error(err, "unable to list ClusterRoles to seek potential operand owners")
				return ctrl.Result{}, err
			}

			if len(clusterRoleList.Items) < 1 {
				err := errors.NewNotFound(
					schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "ClusterRole"}, "potentialOwner")
				logger.Error(err, "could not find ClusterRole owned by CSV to inherit operand")
				return ctrl.Result{}, err
			}

			instance = bridgeProviderCR(clusterRoleList)
			if err := r.Create(ctx, instance); err != nil {
				logger.Error(err, "error while creating new cluster-scoped resource")
				return ctrl.Result{}, err
			} else {
				logger.Info("cluster-scoped resource created")
				return ctrl.Result{}, nil
			}
		}
		// error fetching the resource, requeue and try again
		logger.Error(err, "error fetching the resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// bridgeProviderCR CR for RDS registration
func bridgeProviderCR(clusterRoleList *rbac.ClusterRoleList) *dbaasoperator.DBaaSProvider {
	instance := &dbaasoperator.DBaaSProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name: providerCRName,
			OwnerReferences: []metav1.OwnerReference{
				{

					APIVersion:         "rbac.authorization.k8s.io/v1",
					Kind:               "ClusterRole",
					UID:                clusterRoleList.Items[0].GetUID(),
					Name:               clusterRoleList.Items[0].Name,
					Controller:         pointer.BoolPtr(true),
					BlockOwnerDeletion: pointer.BoolPtr(false),
				},
			},
			Labels: labels,
		},

		Spec: dbaasoperator.DBaaSProviderSpec{
			Provider: dbaasoperator.DatabaseProvider{
				Name:               provider,
				DisplayName:        displayName,
				DisplayDescription: displayDescription,
				Icon: dbaasoperator.ProviderIcon{
					Data:      iconData,
					MediaType: mediaType,
				},
			},
			InventoryKind:  inventoryKind,
			ConnectionKind: connectionKind,
			InstanceKind:   instanceKind,
			CredentialFields: []dbaasoperator.CredentialField{
				{
					Key:         awsAccessKeyID,
					DisplayName: "AWS Access Key ID",
					Type:        "maskedstring",
					Required:    true,
					HelpText:    awsAccessKeyIDHelpText,
				},
				{
					Key:         awsSecretAccessKey,
					DisplayName: "AWS Secret Access Key",
					Type:        "maskedstring",
					Required:    true,
					HelpText:    awsSecretAccessKeyHelpText,
				},
				{
					Key:         awsRegion,
					DisplayName: "AWS Region",
					Type:        "string",
					Required:    true,
					HelpText:    awsRegionHelpText,
				},
				{
					Key:         ackResourceTags,
					DisplayName: "ACK Resource Tags",
					Type:        "string",
					Required:    false,
					HelpText:    ackResourceTagsHelpText,
				},
				{
					Key:         ackLogLevel,
					DisplayName: "ACK Log Level",
					Type:        "string",
					Required:    false,
					HelpText:    ackLogLevelHelpText,
				},
			},
			AllowsFreeTrial:              true,
			ExternalProvisionURL:         provisionDocURL,
			ExternalProvisionDescription: provisionDescription,
			InstanceParameterSpecs: []dbaasoperator.InstanceParameterSpec{
				{
					Name:        engine,
					DisplayName: "Engine Type",
					Type:        "string",
					Required:    true,
				},
				{
					Name:        engineVersion,
					DisplayName: "Engine Version",
					Type:        "string",
					Required:    false,
				},
				{
					Name:         dbInstanceClass,
					DisplayName:  "DB instance class",
					Type:         "string",
					Required:     true,
					DefaultValue: defaultDBInstanceClass,
				},
				{
					Name:         storageType,
					DisplayName:  "Storage type",
					Type:         "string",
					Required:     false,
					DefaultValue: "gp2",
				},
				{
					Name:         allocatedStorage,
					DisplayName:  "Allocated storage",
					Type:         "int",
					Required:     true,
					DefaultValue: strconv.Itoa(defaultAllocatedStorage),
				},
				{
					Name:        iops,
					DisplayName: "Provisioned IOPS",
					Type:        "int",
					Required:    false,
				},
				{
					Name:        maxAllocatedStorage,
					DisplayName: "Maximum storage threshold",
					Type:        "int",
					Required:    false,
				},
				{
					Name:        dbSubnetGroupName,
					DisplayName: "Subnet group",
					Type:        "string",
					Required:    false,
				},
				{
					Name:         publiclyAccessible,
					DisplayName:  "Public access",
					Type:         "bool",
					Required:     false,
					DefaultValue: strconv.FormatBool(defaultPubliclyAccessible),
				},
				{
					Name:        vpcSecurityGroupIDs,
					DisplayName: "VPC security groups",
					Type:        "string",
					Required:    false,
				},
				{
					Name:        licenseModel,
					DisplayName: "License Model",
					Type:        "string",
					Required:    false,
				},
			},
		},
	}
	return instance
}

// CheckCrdInstalled checks whether dbaas provider CRD, has been created yet
func (r *DBaaSProviderReconciler) checkCrdInstalled(groupVersion, kind string) (bool, error) {
	resources, err := r.Clientset.Discovery().ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	for _, r := range resources.APIResources {
		if r.Kind == kind {
			return true, nil
		}
	}
	return false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DBaaSProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := log.FromContext(context.Background(), "DBaaSProvider", "Manager", "during", "DBaaSProviderReconciler setup")

	// envVar set in controller-manager's Deployment YAML
	if operatorInstallNamespace, found := os.LookupEnv("INSTALL_NAMESPACE"); !found {
		err := fmt.Errorf("INSTALL_NAMESPACE must be set")
		logger.Error(err, "error fetching envVar")
		return err
	} else {
		r.operatorInstallNamespace = operatorInstallNamespace
	}

	// envVar set for all operators
	if operatorNameEnvVar, found := os.LookupEnv("OPERATOR_CONDITION_NAME"); !found {
		err := fmt.Errorf("OPERATOR_CONDITION_NAME must be set")
		logger.Error(err, "error fetching envVar")
		return err
	} else {
		r.operatorNameVersion = operatorNameEnvVar
	}

	customRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(30*time.Second, 30*time.Minute)

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{RateLimiter: customRateLimiter}).
		For(&v1.Deployment{}).
		WithEventFilter(r.ignoreOtherDeployments()).
		Complete(r)
}

//ignoreOtherDeployments  only on a 'create' event is issued for the deployment
func (r *DBaaSProviderReconciler) ignoreOtherDeployments() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return r.evaluatePredicateObject(e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func (r *DBaaSProviderReconciler) evaluatePredicateObject(obj client.Object) bool {
	lbls := obj.GetLabels()
	if obj.GetNamespace() == r.operatorInstallNamespace {
		if val, keyFound := lbls["olm.owner.kind"]; keyFound {
			if val == "ClusterServiceVersion" {
				if val, keyFound := lbls["olm.owner"]; keyFound {
					return val == r.operatorNameVersion
				}
			}
		}
	}
	return false
}
