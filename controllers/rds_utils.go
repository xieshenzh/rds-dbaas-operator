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
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1beta1 "github.com/RHEcosystemAppEng/dbaas-operator/api/v1beta1"
	rdsv1alpha1 "github.com/aws-controllers-k8s/rds-controller/apis/v1alpha1"
)

const (
	rdsClusterKind = "DBCluster"

	clusterType = "cluster"
)

func parseNamespacedName(namespacedNameString string) types.NamespacedName {
	values := strings.SplitN(namespacedNameString, "/", 2)

	switch len(values) {
	case 1:
		return types.NamespacedName{Name: values[0]}
	default:
		return types.NamespacedName{Namespace: values[0], Name: values[1]}
	}
}

func setCredentials(ctx context.Context, cli client.Client, scheme *runtime.Scheme, name string,
	namespace string, owner metav1.Object, kind string, setSpec func(string)) (*v1.Secret, error) {
	logger := log.FromContext(ctx)

	secretName := fmt.Sprintf("%s-credentials", name)
	secret := &v1.Secret{}
	if e := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, secret); e != nil {
		if errors.IsNotFound(e) {
			secret.Name = secretName
			secret.Namespace = namespace
			secret.ObjectMeta.Labels = createSecretLabels()
			if annotations := createSecretAnnotations(owner, kind); annotations != nil {
				secret.ObjectMeta.Annotations = annotations
			}
			if owner != nil {
				if e := ctrl.SetControllerReference(owner, secret, scheme); e != nil {
					logger.Error(e, "Failed to set owner reference for credential Secret")
					return nil, e
				}
			}
			secret.Data = map[string][]byte{
				"password": []byte(generatePassword()),
			}
			if e := cli.Create(ctx, secret); e != nil {
				logger.Error(e, "Failed to create credential secret")
				return nil, e
			}
		} else {
			logger.Error(e, "Failed to retrieve credential secret")
			return nil, e
		}
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	if _, ok := secret.Data["password"]; !ok {
		secret.Data["password"] = []byte(generatePassword())
		if e := cli.Update(ctx, secret); e != nil {
			logger.Error(e, "Failed to update credential secret")
			return nil, e
		}
	}

	setSpec(secretName)

	return secret, nil
}

func createSecretLabels() map[string]string {
	return map[string]string{
		dbaasv1beta1.TypeLabelKey: dbaasv1beta1.TypeLabelValue,
	}
}

func createSecretAnnotations(owner metav1.Object, kind string) map[string]string {
	if owner != nil {
		return map[string]string{
			"managed-by":      "rds-dbaas-operator",
			"owner":           owner.GetName(),
			"owner.kind":      kind,
			"owner.namespace": owner.GetNamespace(),
		}
	}
	return nil
}

func parseDBInstanceStatus(dbInstance *rdsv1alpha1.DBInstance) map[string]string {
	instanceStatus := map[string]string{}
	if dbInstance.Spec.Engine != nil {
		instanceStatus["engine"] = *dbInstance.Spec.Engine
	}
	if dbInstance.Spec.EngineVersion != nil {
		instanceStatus["engineVersion"] = *dbInstance.Spec.EngineVersion
	}
	if dbInstance.Status.ACKResourceMetadata != nil {
		if dbInstance.Status.ACKResourceMetadata.ARN != nil {
			instanceStatus["ackResourceMetadata.arn"] = string(*dbInstance.Status.ACKResourceMetadata.ARN)
		}
		if dbInstance.Status.ACKResourceMetadata.OwnerAccountID != nil {
			instanceStatus["ackResourceMetadata.ownerAccountID"] = string(*dbInstance.Status.ACKResourceMetadata.OwnerAccountID)
		}
		if dbInstance.Status.ACKResourceMetadata.Region != nil {
			instanceStatus["ackResourceMetadata.region"] = string(*dbInstance.Status.ACKResourceMetadata.Region)
		}
	}
	if dbInstance.Status.ActivityStreamEngineNativeAuditFieldsIncluded != nil {
		instanceStatus["activityStreamEngineNativeAuditFieldsIncluded"] = strconv.FormatBool(*dbInstance.Status.ActivityStreamEngineNativeAuditFieldsIncluded)
	}
	if dbInstance.Status.ActivityStreamKinesisStreamName != nil {
		instanceStatus["activityStreamKinesisStreamName"] = *dbInstance.Status.ActivityStreamKinesisStreamName
	}
	if dbInstance.Status.ActivityStreamKMSKeyID != nil {
		instanceStatus["activityStreamKMSKeyID"] = *dbInstance.Status.ActivityStreamKMSKeyID
	}
	if dbInstance.Status.ActivityStreamMode != nil {
		instanceStatus["activityStreamMode"] = *dbInstance.Status.ActivityStreamMode
	}
	if dbInstance.Status.ActivityStreamPolicyStatus != nil {
		instanceStatus["activityStreamPolicyStatus"] = *dbInstance.Status.ActivityStreamPolicyStatus
	}
	if dbInstance.Status.ActivityStreamStatus != nil {
		instanceStatus["activityStreamStatus"] = *dbInstance.Status.ActivityStreamStatus
	}
	if dbInstance.Status.AssociatedRoles != nil {
		for i, r := range dbInstance.Status.AssociatedRoles {
			if r != nil {
				if r.FeatureName != nil {
					instanceStatus[fmt.Sprintf("associatedRoles[%d].featureName", i)] = *r.FeatureName
				}
				if r.RoleARN != nil {
					instanceStatus[fmt.Sprintf("associatedRoles[%d].roleARN", i)] = *r.RoleARN
				}
				if r.Status != nil {
					instanceStatus[fmt.Sprintf("associatedRoles[%d].status", i)] = *r.Status
				}
			}
		}
	}
	if dbInstance.Status.AutomaticRestartTime != nil {
		instanceStatus["automaticRestartTime"] = dbInstance.Status.AutomaticRestartTime.String()
	}
	if dbInstance.Status.AutomationMode != nil {
		instanceStatus["automationMode"] = *dbInstance.Status.AutomationMode
	}
	if dbInstance.Status.AWSBackupRecoveryPointARN != nil {
		instanceStatus["awsBackupRecoveryPointARN"] = *dbInstance.Status.AWSBackupRecoveryPointARN
	}
	if dbInstance.Status.CACertificateIdentifier != nil {
		instanceStatus["caCertificateIdentifier"] = *dbInstance.Status.CACertificateIdentifier
	}
	if dbInstance.Status.CustomerOwnedIPEnabled != nil {
		instanceStatus["customerOwnedIPEnabled"] = strconv.FormatBool(*dbInstance.Status.CustomerOwnedIPEnabled)
	}
	if dbInstance.Status.DBInstanceAutomatedBackupsReplications != nil {
		for i, r := range dbInstance.Status.DBInstanceAutomatedBackupsReplications {
			if r != nil && r.DBInstanceAutomatedBackupsARN != nil {
				instanceStatus[fmt.Sprintf("dbInstanceAutomatedBackupsReplications[%d].dbInstanceAutomatedBackupsARN", i)] = *r.DBInstanceAutomatedBackupsARN
			}
		}
	}
	if dbInstance.Status.DBInstanceStatus != nil {
		instanceStatus["dbInstanceStatus"] = *dbInstance.Status.DBInstanceStatus
	}
	if dbInstance.Status.DBParameterGroups != nil {
		for i, g := range dbInstance.Status.DBParameterGroups {
			if g != nil {
				if g.DBParameterGroupName != nil {
					instanceStatus[fmt.Sprintf("dbParameterGroups[%d].dbParameterGroupName", i)] = *g.DBParameterGroupName
				}
				if g.ParameterApplyStatus != nil {
					instanceStatus[fmt.Sprintf("dbParameterGroups[%d].parameterApplyStatus", i)] = *g.ParameterApplyStatus
				}
			}
		}
	}
	if dbInstance.Status.DBSubnetGroup != nil {
		if dbInstance.Status.DBSubnetGroup.DBSubnetGroupARN != nil {
			instanceStatus["dbSubnetGroup.dbSubnetGroupARN"] = *dbInstance.Status.DBSubnetGroup.DBSubnetGroupARN
		}
		if dbInstance.Status.DBSubnetGroup.DBSubnetGroupDescription != nil {
			instanceStatus["dbSubnetGroup.dbSubnetGroupDescription"] = *dbInstance.Status.DBSubnetGroup.DBSubnetGroupDescription
		}
		if dbInstance.Status.DBSubnetGroup.DBSubnetGroupName != nil {
			instanceStatus["dbSubnetGroup.dbSubnetGroupName"] = *dbInstance.Status.DBSubnetGroup.DBSubnetGroupName
		}
		if dbInstance.Status.DBSubnetGroup.SubnetGroupStatus != nil {
			instanceStatus["dbSubnetGroup.subnetGroupStatus"] = *dbInstance.Status.DBSubnetGroup.SubnetGroupStatus
		}
		if dbInstance.Status.DBSubnetGroup.Subnets != nil {
			for i, s := range dbInstance.Status.DBSubnetGroup.Subnets {
				if s != nil {
					if s.SubnetAvailabilityZone != nil {
						if s.SubnetAvailabilityZone.Name != nil {
							instanceStatus[fmt.Sprintf("dbSubnetGroup.subnets[%d].subnetAvailabilityZone.name", i)] = *s.SubnetAvailabilityZone.Name
						}
					}
					if s.SubnetIdentifier != nil {
						instanceStatus[fmt.Sprintf("dbSubnetGroup.subnets[%d].subnetIdentifier", i)] = *s.SubnetIdentifier
					}
					if s.SubnetOutpost != nil {
						if s.SubnetOutpost.ARN != nil {
							instanceStatus[fmt.Sprintf("dbSubnetGroup.subnets[%d].subnetOutpost.arn", i)] = *s.SubnetOutpost.ARN
						}
					}
					if s.SubnetStatus != nil {
						instanceStatus[fmt.Sprintf("dbSubnetGroup.subnets[%d].subnetStatus", i)] = *s.SubnetStatus
					}
				}
			}
		}
		if dbInstance.Status.DBSubnetGroup.VPCID != nil {
			instanceStatus["dbSubnetGroup.vpcID"] = *dbInstance.Status.DBSubnetGroup.VPCID
		}
	}
	if dbInstance.Status.DBInstancePort != nil {
		instanceStatus["dbInstancePort"] = strconv.FormatInt(*dbInstance.Status.DBInstancePort, 10)
	}
	if dbInstance.Status.DBIResourceID != nil {
		instanceStatus["dbiResourceID"] = *dbInstance.Status.DBIResourceID
	}
	if dbInstance.Status.DomainMemberships != nil {
		for i, m := range dbInstance.Status.DomainMemberships {
			if m != nil {
				if m.Domain != nil {
					instanceStatus[fmt.Sprintf("domainMemberships[%d].domain", i)] = *m.Domain
				}
				if m.FQDN != nil {
					instanceStatus[fmt.Sprintf("domainMemberships[%d].fQDN", i)] = *m.FQDN
				}
				if m.IAMRoleName != nil {
					instanceStatus[fmt.Sprintf("domainMemberships[%d].iamRoleName", i)] = *m.IAMRoleName
				}
				if m.Status != nil {
					instanceStatus[fmt.Sprintf("domainMemberships[%d].status", i)] = *m.Status
				}
			}
		}
	}
	if dbInstance.Status.EnabledCloudwatchLogsExports != nil {
		for i, e := range dbInstance.Status.EnabledCloudwatchLogsExports {
			if e != nil {
				instanceStatus[fmt.Sprintf("enabledCloudwatchLogsExports[%d]", i)] = *e
			}
		}
	}
	if dbInstance.Status.Endpoint != nil {
		if dbInstance.Status.Endpoint.Address != nil {
			instanceStatus["endpoint.address"] = *dbInstance.Status.Endpoint.Address
		}
		if dbInstance.Status.Endpoint.HostedZoneID != nil {
			instanceStatus["endpoint.hostedZoneID"] = *dbInstance.Status.Endpoint.HostedZoneID
		}
		if dbInstance.Status.Endpoint.Port != nil {
			instanceStatus["endpoint.port"] = strconv.FormatInt(*dbInstance.Status.Endpoint.Port, 10)
		}
	}
	if dbInstance.Status.EnhancedMonitoringResourceARN != nil {
		instanceStatus["enhancedMonitoringResourceARN"] = *dbInstance.Status.EnhancedMonitoringResourceARN
	}
	if dbInstance.Status.IAMDatabaseAuthenticationEnabled != nil {
		instanceStatus["iamDatabaseAuthenticationEnabled"] = strconv.FormatBool(*dbInstance.Status.IAMDatabaseAuthenticationEnabled)
	}
	if dbInstance.Status.InstanceCreateTime != nil {
		instanceStatus["instanceCreateTime"] = dbInstance.Status.InstanceCreateTime.String()
	}
	if dbInstance.Status.LatestRestorableTime != nil {
		instanceStatus["latestRestorableTime"] = dbInstance.Status.LatestRestorableTime.String()
	}
	if dbInstance.Status.ListenerEndpoint != nil {
		if dbInstance.Status.ListenerEndpoint.Address != nil {
			instanceStatus["listenerEndpoint.address"] = *dbInstance.Status.ListenerEndpoint.Address
		}
		if dbInstance.Status.ListenerEndpoint.HostedZoneID != nil {
			instanceStatus["listenerEndpoint.hostedZoneID"] = *dbInstance.Status.ListenerEndpoint.HostedZoneID
		}
		if dbInstance.Status.ListenerEndpoint.Port != nil {
			instanceStatus["listenerEndpoint.port"] = strconv.FormatInt(*dbInstance.Status.ListenerEndpoint.Port, 10)
		}
	}
	if dbInstance.Status.OptionGroupMemberships != nil {
		for i, m := range dbInstance.Status.OptionGroupMemberships {
			if m != nil {
				if m.OptionGroupName != nil {
					instanceStatus[fmt.Sprintf("optionGroupMemberships[%d].optionGroupName", i)] = *m.OptionGroupName
				}
				if m.Status != nil {
					instanceStatus[fmt.Sprintf("optionGroupMemberships[%d].status", i)] = *m.Status
				}
			}
		}
	}
	if dbInstance.Status.PendingModifiedValues != nil {
		if dbInstance.Status.PendingModifiedValues.AllocatedStorage != nil {
			instanceStatus["pendingModifiedValues.allocatedStorage"] = strconv.FormatInt(*dbInstance.Status.PendingModifiedValues.AllocatedStorage, 10)
		}
		if dbInstance.Status.PendingModifiedValues.AutomationMode != nil {
			instanceStatus["pendingModifiedValues.automationMode"] = *dbInstance.Status.PendingModifiedValues.AutomationMode
		}
		if dbInstance.Status.PendingModifiedValues.BackupRetentionPeriod != nil {
			instanceStatus["pendingModifiedValues.backupRetentionPeriod"] = strconv.FormatInt(*dbInstance.Status.PendingModifiedValues.BackupRetentionPeriod, 10)
		}
		if dbInstance.Status.PendingModifiedValues.CACertificateIdentifier != nil {
			instanceStatus["pendingModifiedValues.caCertificateIdentifier"] = *dbInstance.Status.PendingModifiedValues.CACertificateIdentifier
		}
		if dbInstance.Status.PendingModifiedValues.DBInstanceClass != nil {
			instanceStatus["pendingModifiedValues.dbInstanceClass"] = *dbInstance.Status.PendingModifiedValues.DBInstanceClass
		}
		if dbInstance.Status.PendingModifiedValues.DBInstanceIdentifier != nil {
			instanceStatus["pendingModifiedValues.dbInstanceIdentifier"] = *dbInstance.Status.PendingModifiedValues.DBInstanceIdentifier
		}
		if dbInstance.Status.PendingModifiedValues.DBSubnetGroupName != nil {
			instanceStatus["pendingModifiedValues.dbSubnetGroupName"] = *dbInstance.Status.PendingModifiedValues.DBSubnetGroupName
		}
		if dbInstance.Status.PendingModifiedValues.EngineVersion != nil {
			instanceStatus["pendingModifiedValues.engineVersion"] = *dbInstance.Status.PendingModifiedValues.EngineVersion
		}
		if dbInstance.Status.PendingModifiedValues.IAMDatabaseAuthenticationEnabled != nil {
			instanceStatus["pendingModifiedValues.iamDatabaseAuthenticationEnabled"] = strconv.FormatBool(*dbInstance.Status.PendingModifiedValues.IAMDatabaseAuthenticationEnabled)
		}
		if dbInstance.Status.PendingModifiedValues.IOPS != nil {
			instanceStatus["pendingModifiedValues.iops"] = strconv.FormatInt(*dbInstance.Status.PendingModifiedValues.IOPS, 10)
		}
		if dbInstance.Status.PendingModifiedValues.LicenseModel != nil {
			instanceStatus["pendingModifiedValues.licenseModel"] = *dbInstance.Status.PendingModifiedValues.LicenseModel
		}
		if dbInstance.Status.PendingModifiedValues.MasterUserPassword != nil {
			instanceStatus["pendingModifiedValues.masterUserPassword"] = *dbInstance.Status.PendingModifiedValues.MasterUserPassword
		}
		if dbInstance.Status.PendingModifiedValues.MultiAZ != nil {
			instanceStatus["pendingModifiedValues.multiAZ"] = strconv.FormatBool(*dbInstance.Status.PendingModifiedValues.MultiAZ)
		}
		if dbInstance.Status.PendingModifiedValues.PendingCloudwatchLogsExports != nil {
			if dbInstance.Status.PendingModifiedValues.PendingCloudwatchLogsExports.LogTypesToDisable != nil {
				for i, d := range dbInstance.Status.PendingModifiedValues.PendingCloudwatchLogsExports.LogTypesToDisable {
					if d != nil {
						instanceStatus[fmt.Sprintf("pendingModifiedValues.pendingCloudwatchLogsExports.logTypesToDisable[%d]", i)] = *d
					}
				}
			}
			if dbInstance.Status.PendingModifiedValues.PendingCloudwatchLogsExports.LogTypesToEnable != nil {
				for i, e := range dbInstance.Status.PendingModifiedValues.PendingCloudwatchLogsExports.LogTypesToEnable {
					if e != nil {
						instanceStatus[fmt.Sprintf("pendingModifiedValues.pendingCloudwatchLogsExports.logTypesToEnable[%d]", i)] = *e
					}
				}
			}
		}
		if dbInstance.Status.PendingModifiedValues.Port != nil {
			instanceStatus["pendingModifiedValues.port"] = strconv.FormatInt(*dbInstance.Status.PendingModifiedValues.Port, 10)
		}
		if dbInstance.Status.PendingModifiedValues.ProcessorFeatures != nil {
			for i, f := range dbInstance.Status.PendingModifiedValues.ProcessorFeatures {
				if f != nil {
					if f.Name != nil {
						instanceStatus[fmt.Sprintf("pendingModifiedValues.ProcessorFeature[%d].name", i)] = *f.Name
					}
					if f.Value != nil {
						instanceStatus[fmt.Sprintf("pendingModifiedValues.ProcessorFeature[%d].value", i)] = *f.Value
					}
				}
			}
		}
		if dbInstance.Status.PendingModifiedValues.ResumeFullAutomationModeTime != nil {
			instanceStatus["pendingModifiedValues.resumeFullAutomationModeTime"] = dbInstance.Status.PendingModifiedValues.ResumeFullAutomationModeTime.String()
		}
		if dbInstance.Status.PendingModifiedValues.StorageType != nil {
			instanceStatus["pendingModifiedValues.storageType"] = *dbInstance.Status.PendingModifiedValues.StorageType
		}
	}
	if dbInstance.Status.ReadReplicaDBClusterIdentifiers != nil {
		for i, id := range dbInstance.Status.ReadReplicaDBClusterIdentifiers {
			if id != nil {
				instanceStatus[fmt.Sprintf("readReplicaDBClusterIdentifiers[%d]", i)] = *id
			}
		}
	}
	if dbInstance.Status.ReadReplicaDBInstanceIdentifiers != nil {
		for i, id := range dbInstance.Status.ReadReplicaDBInstanceIdentifiers {
			if id != nil {
				instanceStatus[fmt.Sprintf("readReplicaDBInstanceIdentifiers[%d]", i)] = *id
			}
		}
	}
	if dbInstance.Status.ReadReplicaSourceDBInstanceIdentifier != nil {
		instanceStatus["readReplicaSourceDBInstanceIdentifier"] = *dbInstance.Status.ReadReplicaSourceDBInstanceIdentifier
	}
	if dbInstance.Status.ResumeFullAutomationModeTime != nil {
		instanceStatus["resumeFullAutomationModeTime"] = dbInstance.Status.ResumeFullAutomationModeTime.String()
	}
	if dbInstance.Status.SecondaryAvailabilityZone != nil {
		instanceStatus["secondaryAvailabilityZone"] = *dbInstance.Status.SecondaryAvailabilityZone
	}
	if dbInstance.Status.StatusInfos != nil {
		for i, info := range dbInstance.Status.StatusInfos {
			if info != nil {
				if info.Message != nil {
					instanceStatus[fmt.Sprintf("statusInfos[%d].message", i)] = *info.Message
				}
				if info.Normal != nil {
					instanceStatus[fmt.Sprintf("statusInfos[%d].normal", i)] = strconv.FormatBool(*info.Normal)
				}
				if info.Status != nil {
					instanceStatus[fmt.Sprintf("statusInfos[%d].status", i)] = *info.Status
				}
				if info.StatusType != nil {
					instanceStatus[fmt.Sprintf("statusInfos[%d].statusType", i)] = *info.StatusType
				}
			}
		}
	}
	if dbInstance.Status.VPCSecurityGroups != nil {
		for i, g := range dbInstance.Status.VPCSecurityGroups {
			if g != nil {
				if g.Status != nil {
					instanceStatus[fmt.Sprintf("vpcSecurityGroups[%d].status", i)] = *g.Status
				}
				if g.VPCSecurityGroupID != nil {
					instanceStatus[fmt.Sprintf("vpcSecurityGroups[%d].vpcSecurityGroupID", i)] = *g.VPCSecurityGroupID
				}
			}
		}
	}
	return instanceStatus
}

func parseDBClusterStatus(dbCluster *rdsv1alpha1.DBCluster) map[string]string {
	clusterStatus := map[string]string{}
	if dbCluster.Spec.Engine != nil {
		clusterStatus["engine"] = *dbCluster.Spec.Engine
	}
	if dbCluster.Spec.EngineVersion != nil {
		clusterStatus["engineVersion"] = *dbCluster.Spec.EngineVersion
	}
	if dbCluster.Status.ACKResourceMetadata != nil {
		if dbCluster.Status.ACKResourceMetadata.ARN != nil {
			clusterStatus["ackResourceMetadata.arn"] = string(*dbCluster.Status.ACKResourceMetadata.ARN)
		}
		if dbCluster.Status.ACKResourceMetadata.OwnerAccountID != nil {
			clusterStatus["ackResourceMetadata.ownerAccountID"] = string(*dbCluster.Status.ACKResourceMetadata.OwnerAccountID)
		}
		if dbCluster.Status.ACKResourceMetadata.Region != nil {
			clusterStatus["ackResourceMetadata.region"] = string(*dbCluster.Status.ACKResourceMetadata.Region)
		}
	}
	if dbCluster.Status.ActivityStreamKinesisStreamName != nil {
		clusterStatus["activityStreamKinesisStreamName"] = *dbCluster.Status.ActivityStreamKinesisStreamName
	}
	if dbCluster.Status.ActivityStreamKMSKeyID != nil {
		clusterStatus["activityStreamKMSKeyID"] = *dbCluster.Status.ActivityStreamKMSKeyID
	}
	if dbCluster.Status.ActivityStreamMode != nil {
		clusterStatus["activityStreamMode"] = *dbCluster.Status.ActivityStreamMode
	}
	if dbCluster.Status.ActivityStreamStatus != nil {
		clusterStatus["activityStreamStatus"] = *dbCluster.Status.ActivityStreamStatus
	}
	if dbCluster.Status.AssociatedRoles != nil {
		for i, r := range dbCluster.Status.AssociatedRoles {
			if r != nil {
				if r.FeatureName != nil {
					clusterStatus[fmt.Sprintf("associatedRoles[%d].featureName", i)] = *r.FeatureName
				}
				if r.RoleARN != nil {
					clusterStatus[fmt.Sprintf("associatedRoles[%d].roleARN", i)] = *r.RoleARN
				}
				if r.Status != nil {
					clusterStatus[fmt.Sprintf("associatedRoles[%d].status", i)] = *r.Status
				}
			}
		}
	}
	if dbCluster.Status.AutomaticRestartTime != nil {
		clusterStatus["automaticRestartTime"] = dbCluster.Status.AutomaticRestartTime.String()
	}
	if dbCluster.Status.BacktrackConsumedChangeRecords != nil {
		clusterStatus["backtrackConsumedChangeRecords"] = strconv.FormatInt(*dbCluster.Status.BacktrackConsumedChangeRecords, 10)
	}
	if dbCluster.Status.Capacity != nil {
		clusterStatus["capacity"] = strconv.FormatInt(*dbCluster.Status.Capacity, 10)
	}
	if dbCluster.Status.CloneGroupID != nil {
		clusterStatus["cloneGroupID"] = *dbCluster.Status.CloneGroupID
	}
	if dbCluster.Status.ClusterCreateTime != nil {
		clusterStatus["clusterCreateTime"] = dbCluster.Status.ClusterCreateTime.String()
	}
	if dbCluster.Status.CrossAccountClone != nil {
		clusterStatus["crossAccountClone"] = strconv.FormatBool(*dbCluster.Status.CrossAccountClone)
	}
	if dbCluster.Status.CustomEndpoints != nil {
		for i, e := range dbCluster.Status.CustomEndpoints {
			if e != nil {
				clusterStatus[fmt.Sprintf("customEndpoints[%d]", i)] = *e
			}
		}
	}
	if dbCluster.Status.DBClusterMembers != nil {
		for i, m := range dbCluster.Status.DBClusterMembers {
			if m != nil {
				if m.DBClusterParameterGroupStatus != nil {
					clusterStatus[fmt.Sprintf("dbClusterMembers[%d].dbClusterParameterGroupStatus", i)] = *m.DBClusterParameterGroupStatus
				}
				if m.DBInstanceIdentifier != nil {
					clusterStatus[fmt.Sprintf("dbClusterMembers[%d].dbInstanceIdentifier", i)] = *m.DBInstanceIdentifier
				}
				if m.IsClusterWriter != nil {
					clusterStatus[fmt.Sprintf("dbClusterMembers[%d].isClusterWriter", i)] = strconv.FormatBool(*m.IsClusterWriter)
				}
				if m.PromotionTier != nil {
					clusterStatus[fmt.Sprintf("dbClusterMembers[%d].promotionTier", i)] = strconv.FormatInt(*m.PromotionTier, 10)
				}
			}
		}
	}
	if dbCluster.Status.DBClusterOptionGroupMemberships != nil {
		for i, m := range dbCluster.Status.DBClusterOptionGroupMemberships {
			if m != nil {
				if m.DBClusterOptionGroupName != nil {
					clusterStatus[fmt.Sprintf("dbClusterOptionGroupMemberships[%d].dbClusterOptionGroupName", i)] = *m.DBClusterOptionGroupName
				}
				if m.Status != nil {
					clusterStatus[fmt.Sprintf("dbClusterOptionGroupMemberships[%d].status", i)] = *m.Status
				}
			}
		}
	}
	if dbCluster.Status.DBClusterParameterGroup != nil {
		clusterStatus["dbClusterParameterGroup"] = *dbCluster.Status.DBClusterParameterGroup
	}
	if dbCluster.Status.DBSubnetGroup != nil {
		clusterStatus["dbSubnetGroup"] = *dbCluster.Status.DBSubnetGroup
	}
	if dbCluster.Status.DBClusterResourceID != nil {
		clusterStatus["dbClusterResourceID"] = *dbCluster.Status.DBClusterResourceID
	}
	if dbCluster.Status.DomainMemberships != nil {
		for i, m := range dbCluster.Status.DomainMemberships {
			if m != nil {
				if m.Domain != nil {
					clusterStatus[fmt.Sprintf("domainMemberships[%d].domain", i)] = *m.Domain
				}
				if m.FQDN != nil {
					clusterStatus[fmt.Sprintf("domainMemberships[%d].fQDN", i)] = *m.FQDN
				}
				if m.IAMRoleName != nil {
					clusterStatus[fmt.Sprintf("domainMemberships[%d].iamRoleName", i)] = *m.IAMRoleName
				}
				if m.Status != nil {
					clusterStatus[fmt.Sprintf("domainMemberships[%d].status", i)] = *m.Status
				}
			}
		}
	}
	if dbCluster.Status.EarliestBacktrackTime != nil {
		clusterStatus["earliestBacktrackTime"] = dbCluster.Status.EarliestBacktrackTime.String()
	}
	if dbCluster.Status.EarliestRestorableTime != nil {
		clusterStatus["earliestRestorableTime"] = dbCluster.Status.EarliestRestorableTime.String()
	}
	if dbCluster.Status.EnabledCloudwatchLogsExports != nil {
		for i, e := range dbCluster.Status.EnabledCloudwatchLogsExports {
			if e != nil {
				clusterStatus[fmt.Sprintf("enabledCloudwatchLogsExports[%d]", i)] = *e
			}
		}
	}
	if dbCluster.Status.Endpoint != nil {
		clusterStatus["endpoint"] = *dbCluster.Status.Endpoint
	}
	if dbCluster.Status.GlobalWriteForwardingRequested != nil {
		clusterStatus["globalWriteForwardingRequested"] = strconv.FormatBool(*dbCluster.Status.GlobalWriteForwardingRequested)
	}
	if dbCluster.Status.GlobalWriteForwardingStatus != nil {
		clusterStatus["globalWriteForwardingStatus"] = *dbCluster.Status.GlobalWriteForwardingStatus
	}
	if dbCluster.Status.HostedZoneID != nil {
		clusterStatus["hostedZoneID"] = *dbCluster.Status.HostedZoneID
	}
	if dbCluster.Status.HTTPEndpointEnabled != nil {
		clusterStatus["httpEndpointEnabled"] = strconv.FormatBool(*dbCluster.Status.HTTPEndpointEnabled)
	}
	if dbCluster.Status.IAMDatabaseAuthenticationEnabled != nil {
		clusterStatus["iamDatabaseAuthenticationEnabled"] = strconv.FormatBool(*dbCluster.Status.IAMDatabaseAuthenticationEnabled)
	}
	if dbCluster.Status.LatestRestorableTime != nil {
		clusterStatus["latestRestorableTime"] = dbCluster.Status.LatestRestorableTime.String()
	}
	if dbCluster.Status.MultiAZ != nil {
		clusterStatus["multiAZ"] = strconv.FormatBool(*dbCluster.Status.MultiAZ)
	}
	if dbCluster.Status.PendingModifiedValues != nil {
		if dbCluster.Status.PendingModifiedValues.DBClusterIdentifier != nil {
			clusterStatus["pendingModifiedValues.dbClusterIdentifier"] = *dbCluster.Status.PendingModifiedValues.DBClusterIdentifier
		}
		if dbCluster.Status.PendingModifiedValues.EngineVersion != nil {
			clusterStatus["pendingModifiedValues.engineVersion"] = *dbCluster.Status.PendingModifiedValues.EngineVersion
		}
		if dbCluster.Status.PendingModifiedValues.IAMDatabaseAuthenticationEnabled != nil {
			clusterStatus["pendingModifiedValues.iamDatabaseAuthenticationEnabled"] = strconv.FormatBool(*dbCluster.Status.PendingModifiedValues.IAMDatabaseAuthenticationEnabled)
		}
		if dbCluster.Status.PendingModifiedValues.MasterUserPassword != nil {
			clusterStatus["pendingModifiedValues.masterUserPassword"] = *dbCluster.Status.PendingModifiedValues.MasterUserPassword
		}
		if dbCluster.Status.PendingModifiedValues.PendingCloudwatchLogsExports != nil {
			if dbCluster.Status.PendingModifiedValues.PendingCloudwatchLogsExports.LogTypesToDisable != nil {
				for i, d := range dbCluster.Status.PendingModifiedValues.PendingCloudwatchLogsExports.LogTypesToDisable {
					if d != nil {
						clusterStatus[fmt.Sprintf("pendingModifiedValues.pendingCloudwatchLogsExports.logTypesToDisable[%d]", i)] = *d
					}
				}
			}
			if dbCluster.Status.PendingModifiedValues.PendingCloudwatchLogsExports.LogTypesToEnable != nil {
				for i, e := range dbCluster.Status.PendingModifiedValues.PendingCloudwatchLogsExports.LogTypesToEnable {
					if e != nil {
						clusterStatus[fmt.Sprintf("pendingModifiedValues.pendingCloudwatchLogsExports.logTypesToEnable[%d]", i)] = *e
					}
				}
			}
		}
	}
	if dbCluster.Status.PercentProgress != nil {
		clusterStatus["percentProgress"] = *dbCluster.Status.PercentProgress
	}
	if dbCluster.Status.PerformanceInsightsEnabled != nil {
		clusterStatus["performanceInsightsEnabled"] = strconv.FormatBool(*dbCluster.Status.PerformanceInsightsEnabled)
	}
	if dbCluster.Status.ReadReplicaIdentifiers != nil {
		for i, r := range dbCluster.Status.ReadReplicaIdentifiers {
			if r != nil {
				clusterStatus[fmt.Sprintf("readReplicaIdentifiers[%d]", i)] = *r
			}
		}
	}
	if dbCluster.Status.ReaderEndpoint != nil {
		clusterStatus["readerEndpoint"] = *dbCluster.Status.ReaderEndpoint
	}
	if dbCluster.Status.Status != nil {
		clusterStatus["status"] = *dbCluster.Status.Status
	}
	if dbCluster.Status.TagList != nil {
		for i, t := range dbCluster.Status.TagList {
			if t != nil {
				if t.Key != nil {
					clusterStatus[fmt.Sprintf("tagList[%d].key", i)] = *t.Key
				}
				if t.Value != nil {
					clusterStatus[fmt.Sprintf("tagList[%d].value", i)] = *t.Value
				}
			}
		}
	}
	if dbCluster.Status.VPCSecurityGroups != nil {
		for i, g := range dbCluster.Status.VPCSecurityGroups {
			if g != nil {
				if g.Status != nil {
					clusterStatus[fmt.Sprintf("vpcSecurityGroups[%d].status", i)] = *g.Status
				}
				if g.VPCSecurityGroupID != nil {
					clusterStatus[fmt.Sprintf("vpcSecurityGroups[%d].vpcSecurityGroupID", i)] = *g.VPCSecurityGroupID
				}
			}
		}
	}
	return clusterStatus
}

func unmarshalYaml(file []byte, obj interface{}) error {
	if err := yaml.Unmarshal(file, obj); err != nil {
		return err
	}
	return nil
}
