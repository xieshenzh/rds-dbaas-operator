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

package controllers_test

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/google/uuid"

	dbaasv1alpha1 "github.com/RHEcosystemAppEng/dbaas-operator/api/v1alpha1"
	rdsdbaasv1alpha1 "github.com/RHEcosystemAppEng/rds-dbaas-operator/api/v1alpha1"
	"github.com/RHEcosystemAppEng/rds-dbaas-operator/controllers/rds/test"
	rdsv1alpha1 "github.com/aws-controllers-k8s/rds-controller/apis/v1alpha1"
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ophandler "github.com/operator-framework/operator-lib/handler"
)

var _ = Describe("RDSInstanceController", func() {
	Context("when Instance is created", func() {
		instanceName := "rds-instance-instance-controller"
		inventoryName := "rds-inventory-instance-controller"

		instance := &rdsdbaasv1alpha1.RDSInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instanceName,
				Namespace: testNamespace,
			},
			Spec: dbaasv1alpha1.DBaaSInstanceSpec{
				InventoryRef: dbaasv1alpha1.NamespacedName{
					Name:      inventoryName,
					Namespace: testNamespace,
				},
				Name:          instanceName,
				CloudProvider: "AWS",
				CloudRegion:   "us-east-1a",
				OtherInstanceParams: map[string]string{
					"Engine":              "postgres",
					"DBInstanceClass":     "db.t3.micro",
					"AllocatedStorage":    "20",
					"EngineVersion":       "13.2",
					"StorageType":         "gp2",
					"IOPS":                "1000",
					"MaxAllocatedStorage": "50",
					"DBSubnetGroupName":   "default",
					"PubliclyAccessible":  "false",
					"VPCSecurityGroupIDs": "default",
					"LicenseModel":        "license-included",
				},
			},
		}
		BeforeEach(assertResourceCreation(instance))
		AfterEach(assertResourceDeletion(instance))

		Context("when Inventory is not created", func() {
			It("should make Instance in error status", func() {
				ins := &rdsdbaasv1alpha1.RDSInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      instanceName,
						Namespace: testNamespace,
					},
				}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ins), ins); err != nil {
						return false
					}
					condition := apimeta.FindStatusCondition(ins.Status.Conditions, "ProvisionReady")
					if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "NotFound" {
						return false
					}
					return true
				}, timeout).Should(BeTrue())
			})
		})

		Context("when Inventory is created", func() {
			credentialName := "credentials-ref-instance-controller"

			inventory := &rdsdbaasv1alpha1.RDSInventory{
				ObjectMeta: metav1.ObjectMeta{
					Name:      inventoryName,
					Namespace: testNamespace,
				},
				Spec: dbaasv1alpha1.DBaaSInventorySpec{
					CredentialsRef: &dbaasv1alpha1.LocalObjectReference{
						Name: credentialName,
					},
				},
			}
			BeforeEach(assertResourceCreation(inventory))
			AfterEach(assertResourceDeletion(inventory))

			Context("when Inventory is not ready", func() {
				It("should make Instance in error status", func() {
					ins := &rdsdbaasv1alpha1.RDSInstance{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instanceName,
							Namespace: testNamespace,
						},
					}
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ins), ins); err != nil {
							return false
						}
						condition := apimeta.FindStatusCondition(ins.Status.Conditions, "ProvisionReady")
						if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "Unreachable" {
							return false
						}
						return true
					}, timeout).Should(BeTrue())
				})
			})

			Context("when Inventory is ready", func() {
				accessKey := "AKIAIOSFODNN7EXAMPLE" + test.InstanceControllerTestAccessKeySuffix
				secretKey := "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
				region := "us-east-1"

				credential := &v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      credentialName,
						Namespace: testNamespace,
					},
					Data: map[string][]byte{
						"AWS_ACCESS_KEY_ID":     []byte(accessKey),
						"AWS_SECRET_ACCESS_KEY": []byte(secretKey), //#nosec G101
						"AWS_REGION":            []byte(region),
					},
				}
				BeforeEach(assertResourceCreation(credential))
				AfterEach(assertResourceDeletion(credential))

				Context("when engine is not set for the instance", func() {
					instanceEngine := &rdsdbaasv1alpha1.RDSInstance{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instanceName + "-engine",
							Namespace: testNamespace,
						},
						Spec: dbaasv1alpha1.DBaaSInstanceSpec{
							InventoryRef: dbaasv1alpha1.NamespacedName{
								Name:      inventoryName,
								Namespace: testNamespace,
							},
							Name:          instanceName + "-engine",
							CloudProvider: "AWS",
							CloudRegion:   "us-east-1a",
							OtherInstanceParams: map[string]string{
								"DBInstanceClass":  "db.t3.micro",
								"AllocatedStorage": "20",
							},
						},
					}
					BeforeEach(assertResourceCreation(instanceEngine))
					AfterEach(assertResourceDeletion(instanceEngine))

					It("should make Instance in error status", func() {
						ins := &rdsdbaasv1alpha1.RDSInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      instanceName + "-engine",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ins), ins); err != nil {
								return false
							}
							condition := apimeta.FindStatusCondition(ins.Status.Conditions, "ProvisionReady")
							if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "InputError" ||
								condition.Message != "Failed to create or update DB Instance: required parameter Engine is missing" {
								return false
							}
							return true
						}, timeout).Should(BeTrue())
					})
				})

				Context("when identifier is not set for the instance", func() {
					instanceIdentifier := &rdsdbaasv1alpha1.RDSInstance{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instanceName + "-identifier",
							Namespace: testNamespace,
						},
						Spec: dbaasv1alpha1.DBaaSInstanceSpec{
							InventoryRef: dbaasv1alpha1.NamespacedName{
								Name:      inventoryName,
								Namespace: testNamespace,
							},
							Name:          instanceName + "-identifier",
							CloudProvider: "AWS",
							CloudRegion:   "us-east-1a",
							OtherInstanceParams: map[string]string{
								"Engine":           "postgres",
								"DBInstanceClass":  "db.t3.micro",
								"AllocatedStorage": "20",
							},
						},
					}
					BeforeEach(assertResourceCreation(instanceIdentifier))
					AfterEach(assertResourceDeletion(instanceIdentifier))

					It("should generate DB instance identifier", func() {
						dbInstance := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      instanceName + "-identifier",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
								return false
							}
							if dbInstance.Spec.DBInstanceIdentifier == nil {
								return false
							}
							Expect(strings.HasPrefix(*dbInstance.Spec.DBInstanceIdentifier, "rhoda-postgres-")).Should(BeTrue())
							id := strings.TrimPrefix(*dbInstance.Spec.DBInstanceIdentifier, "rhoda-postgres-")
							_, err := uuid.Parse(id)
							Expect(err).ShouldNot(HaveOccurred())
							if dbInstance.Spec.PubliclyAccessible == nil || *dbInstance.Spec.PubliclyAccessible != true {
								return false
							}
							return true
						}, timeout).Should(BeTrue())
					})
				})

				Context("when class is not set for the instance", func() {
					instanceClass := &rdsdbaasv1alpha1.RDSInstance{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instanceName + "-class",
							Namespace: testNamespace,
						},
						Spec: dbaasv1alpha1.DBaaSInstanceSpec{
							InventoryRef: dbaasv1alpha1.NamespacedName{
								Name:      inventoryName,
								Namespace: testNamespace,
							},
							Name:          instanceName + "-class",
							CloudProvider: "AWS",
							CloudRegion:   "us-east-1a",
							OtherInstanceParams: map[string]string{
								"Engine":           "postgres",
								"AllocatedStorage": "20",
							},
						},
					}
					BeforeEach(assertResourceCreation(instanceClass))
					AfterEach(assertResourceDeletion(instanceClass))

					It("should use the default instance class", func() {
						dbInstance := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      instanceName + "-class",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
								return false
							}
							if dbInstance.Spec.DBInstanceClass == nil || *dbInstance.Spec.DBInstanceClass != "db.t3.micro" {
								return false
							}
							if dbInstance.Spec.PubliclyAccessible == nil || *dbInstance.Spec.PubliclyAccessible != true {
								return false
							}
							return true
						}, timeout).Should(BeTrue())
					})
				})

				Context("when storage is not set for the instance", func() {
					instanceStorage := &rdsdbaasv1alpha1.RDSInstance{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instanceName + "-storage",
							Namespace: testNamespace,
						},
						Spec: dbaasv1alpha1.DBaaSInstanceSpec{
							InventoryRef: dbaasv1alpha1.NamespacedName{
								Name:      inventoryName,
								Namespace: testNamespace,
							},
							Name:          instanceName + "-storage",
							CloudProvider: "AWS",
							CloudRegion:   "us-east-1a",
							OtherInstanceParams: map[string]string{
								"Engine":          "postgres",
								"DBInstanceClass": "db.t3.micro",
							},
						},
					}
					BeforeEach(assertResourceCreation(instanceStorage))
					AfterEach(assertResourceDeletion(instanceStorage))

					It("should use the default storage", func() {
						dbInstance := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      instanceName + "-storage",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
								return false
							}
							if dbInstance.Spec.AllocatedStorage == nil || *dbInstance.Spec.AllocatedStorage != 20 {
								return false
							}
							if dbInstance.Spec.PubliclyAccessible == nil || *dbInstance.Spec.PubliclyAccessible != true {
								return false
							}
							return true
						}, timeout).Should(BeTrue())
					})
				})

				Context("when region is not set for the instance", func() {
					instanceRegion := &rdsdbaasv1alpha1.RDSInstance{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instanceName + "-region",
							Namespace: testNamespace,
						},
						Spec: dbaasv1alpha1.DBaaSInstanceSpec{
							InventoryRef: dbaasv1alpha1.NamespacedName{
								Name:      inventoryName,
								Namespace: testNamespace,
							},
							Name:          instanceName + "-region",
							CloudProvider: "AWS",
							OtherInstanceParams: map[string]string{
								"Engine":               "postgres",
								"DBInstanceIdentifier": "rds-instance-instance-controller",
								"DBInstanceClass":      "db.t3.micro",
								"AllocatedStorage":     "20",
							},
						},
					}
					BeforeEach(assertResourceCreation(instanceRegion))
					AfterEach(assertResourceDeletion(instanceRegion))

					It("should use the default region", func() {
						dbInstance := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      instanceName + "-region",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
								return false
							}
							if dbInstance.Spec.AvailabilityZone == nil || *dbInstance.Spec.AvailabilityZone != "us-east-1a" {
								return false
							}
							if dbInstance.Spec.PubliclyAccessible == nil || *dbInstance.Spec.PubliclyAccessible != true {
								return false
							}
							return true
						}, timeout).Should(BeTrue())
					})
				})

				Context("when Instance creation info is complete", func() {
					Context("when checking the creation of the RDS DB Instance", func() {
						It("should create RDS DB instance and Secret for user password", func() {
							By("checking if the RDS DB instance is created")
							dbInstance := &rdsv1alpha1.DBInstance{
								ObjectMeta: metav1.ObjectMeta{
									Name:      instanceName,
									Namespace: testNamespace,
								},
							}
							Eventually(func() bool {
								if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
									return false
								}
								Expect(dbInstance.Spec.Engine).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.Engine).Should(Equal("postgres"))
								Expect(dbInstance.Spec.DBInstanceIdentifier).ShouldNot(BeNil())
								Expect(strings.HasPrefix(*dbInstance.Spec.DBInstanceIdentifier, "rhoda-postgres-")).Should(BeTrue())
								id := strings.TrimPrefix(*dbInstance.Spec.DBInstanceIdentifier, "rhoda-postgres-")
								_, err := uuid.Parse(id)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(dbInstance.Spec.DBInstanceClass).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.DBInstanceClass).Should(Equal("db.t3.micro"))
								Expect(dbInstance.Spec.AllocatedStorage).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.AllocatedStorage).Should(BeNumerically("==", 20))
								Expect(dbInstance.Spec.EngineVersion).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.EngineVersion).Should(Equal("13.2"))
								Expect(dbInstance.Spec.StorageType).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.StorageType).Should(Equal("gp2"))
								Expect(dbInstance.Spec.IOPS).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.IOPS).Should(BeNumerically("==", 1000))
								Expect(dbInstance.Spec.MaxAllocatedStorage).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.MaxAllocatedStorage).Should(BeNumerically("==", 50))
								Expect(dbInstance.Spec.DBSubnetGroupName).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.DBSubnetGroupName).Should(Equal("default"))
								Expect(dbInstance.Spec.PubliclyAccessible).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.PubliclyAccessible).Should(BeFalse())
								Expect(len(dbInstance.Spec.VPCSecurityGroupIDs)).Should(BeNumerically("==", 1))
								Expect(dbInstance.Spec.VPCSecurityGroupIDs[0]).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.VPCSecurityGroupIDs[0]).Should(Equal("default"))
								Expect(dbInstance.Spec.LicenseModel).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.LicenseModel).Should(Equal("license-included"))
								Expect(dbInstance.Spec.AvailabilityZone).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.AvailabilityZone).Should(Equal("us-east-1a"))
								Expect(dbInstance.Spec.DBName).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.DBName).Should(Equal("postgres"))
								Expect(dbInstance.Spec.MasterUsername).ShouldNot(BeNil())
								Expect(*dbInstance.Spec.MasterUsername).Should(Equal("postgres"))
								Expect(dbInstance.Spec.MasterUserPassword).ShouldNot(BeNil())
								Expect(dbInstance.Spec.MasterUserPassword.Key).Should(Equal("password"))
								Expect(dbInstance.Spec.MasterUserPassword.Namespace).Should(Equal(testNamespace))
								Expect(dbInstance.Spec.MasterUserPassword.Name).Should(Equal(fmt.Sprintf("%s-credentials", instanceName)))

								typeString, typeOk := dbInstance.GetAnnotations()[ophandler.TypeAnnotation]
								Expect(typeOk).Should(BeTrue())
								Expect(typeString).Should(Equal("RDSInstance.dbaas.redhat.com"))
								namespacedNameString, nsnOk := dbInstance.GetAnnotations()[ophandler.NamespacedNameAnnotation]
								Expect(nsnOk).Should(BeTrue())
								Expect(namespacedNameString).Should(Equal(testNamespace + "/" + instanceName))
								return true
							}, timeout).Should(BeTrue())

							By("checking if the Secret for DB user is created")
							dbSecret := &v1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      fmt.Sprintf("%s-credentials", instanceName),
									Namespace: testNamespace,
								},
							}
							Eventually(func() bool {
								if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret), dbSecret); err != nil {
									return false
								}
								v, ok := dbSecret.Data["password"]
								Expect(ok).Should(BeTrue())
								Expect(len(v)).Should(BeNumerically("==", 12))

								secretOwner := metav1.GetControllerOf(dbSecret)
								Expect(secretOwner).ShouldNot(BeNil())
								Expect(secretOwner.Kind).Should(Equal("RDSInstance"))
								Expect(secretOwner.Name).Should(Equal(instanceName))
								Expect(secretOwner.Controller).ShouldNot(BeNil())
								Expect(*secretOwner.Controller).Should(BeTrue())
								Expect(secretOwner.BlockOwnerDeletion).ShouldNot(BeNil())
								Expect(*secretOwner.BlockOwnerDeletion).Should(BeTrue())
								return true
							}, timeout).Should(BeTrue())
						})
					})

					Context("when the Instance is deleted", func() {
						instanceDelete := &rdsdbaasv1alpha1.RDSInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      instanceName + "-delete",
								Namespace: testNamespace,
							},
							Spec: dbaasv1alpha1.DBaaSInstanceSpec{
								InventoryRef: dbaasv1alpha1.NamespacedName{
									Name:      inventoryName,
									Namespace: testNamespace,
								},
								Name:          instanceName + "-delete",
								CloudProvider: "AWS",
								CloudRegion:   "us-east-1a",
								OtherInstanceParams: map[string]string{
									"Engine":               "postgres",
									"DBInstanceIdentifier": "rds-instance-instance-controller",
									"DBInstanceClass":      "db.t3.micro",
									"AllocatedStorage":     "20",
								},
							},
						}
						BeforeEach(assertResourceCreation(instanceDelete))

						It("should delete the RDS DB instance", func() {
							dbInstance := &rdsv1alpha1.DBInstance{
								ObjectMeta: metav1.ObjectMeta{
									Name:      instanceName + "-delete",
									Namespace: testNamespace,
								},
							}
							Eventually(func() bool {
								if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
									return false
								}
								return true
							}, timeout).Should(BeTrue())

							assertResourceDeletion(instanceDelete)()

							By("checking if the RDS DB instance is deleted")
							Eventually(func() bool {
								if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
									if errors.IsNotFound(err) {
										return true
									}
								}
								return false
							}, timeout).Should(BeTrue())
						})
					})

					Context("when checking the status of the Instance", func() {
						DescribeTable("checking handling DB instance status",
							func(instanceStatus *string, phase, cond, reason, message string) {
								dbInstance := &rdsv1alpha1.DBInstance{
									ObjectMeta: metav1.ObjectMeta{
										Name:      instanceName,
										Namespace: testNamespace,
									},
								}
								Eventually(func() bool {
									if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
										return false
									}

									dbInstance.Status.DBInstanceStatus = instanceStatus

									tm := metav1.Time{Time: time.Now()}
									dbInstance.Status.Conditions = []*ackv1alpha1.Condition{
										{
											Type:               "test-condition-1",
											Status:             "False",
											LastTransitionTime: &tm,
											Reason:             pointer.String("test:reason"),
											Message:            pointer.String("test-message"),
										},
										{
											Type:               "test-condition-2",
											Status:             "True",
											LastTransitionTime: &tm,
											Reason:             nil,
											Message:            pointer.String("test-message"),
										},
										{
											Type:               "test-condition-3",
											Status:             "True",
											LastTransitionTime: &tm,
											Reason:             pointer.String("test++reason--"),
											Message:            pointer.String("test-message"),
										},
										{
											Type:               "test-condition-4",
											Status:             "False",
											LastTransitionTime: nil,
											Reason:             pointer.String("test##reason&&"),
											Message:            nil,
										},
									}

									arn := ackv1alpha1.AWSResourceName("test-arn")
									region := ackv1alpha1.AWSRegion("test-region")
									accountId := ackv1alpha1.AWSAccountID("test-id")
									dbInstance.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
										ARN:            &arn,
										Region:         &region,
										OwnerAccountID: &accountId,
									}
									dbInstance.Status.ActivityStreamEngineNativeAuditFieldsIncluded = pointer.Bool(false)
									dbInstance.Status.ActivityStreamKinesisStreamName = pointer.String("test-name")
									dbInstance.Status.ActivityStreamKMSKeyID = pointer.String("test-id")
									dbInstance.Status.ActivityStreamMode = pointer.String("test-mode")
									dbInstance.Status.ActivityStreamStatus = pointer.String("test-status")
									dbInstance.Status.AssociatedRoles = []*rdsv1alpha1.DBInstanceRole{
										{
											FeatureName: pointer.String("test-name"),
											RoleARN:     pointer.String("test-arn"),
											Status:      pointer.String("test-status"),
										},
									}
									dbInstance.Status.AutomaticRestartTime = &tm
									dbInstance.Status.AutomationMode = pointer.String("test-mode")
									dbInstance.Status.AWSBackupRecoveryPointARN = pointer.String("test-arn")
									dbInstance.Status.CACertificateIdentifier = pointer.String("test-identifier")
									dbInstance.Status.CustomerOwnedIPEnabled = pointer.Bool(false)
									dbInstance.Status.DBInstanceAutomatedBackupsReplications = []*rdsv1alpha1.DBInstanceAutomatedBackupsReplication{
										{
											DBInstanceAutomatedBackupsARN: pointer.String("test-arn"),
										},
									}
									dbInstance.Status.DBParameterGroups = []*rdsv1alpha1.DBParameterGroupStatus_SDK{
										{
											DBParameterGroupName: pointer.String("test-name"),
											ParameterApplyStatus: pointer.String("test-status"),
										},
									}
									dbInstance.Status.DBSubnetGroup = &rdsv1alpha1.DBSubnetGroup_SDK{
										DBSubnetGroupARN:         pointer.String("test-arn"),
										DBSubnetGroupDescription: pointer.String("test-desc"),
										DBSubnetGroupName:        pointer.String("test-name"),
										SubnetGroupStatus:        pointer.String("test-status"),
										Subnets: []*rdsv1alpha1.Subnet{
											{
												SubnetAvailabilityZone: &rdsv1alpha1.AvailabilityZone{
													Name: pointer.String("test-name"),
												},
												SubnetIdentifier: pointer.String("test-identifier"),
												SubnetOutpost: &rdsv1alpha1.Outpost{
													ARN: pointer.String("test-arn"),
												},
												SubnetStatus: pointer.String("test-status"),
											},
										},
										VPCID: pointer.String("test-id"),
									}
									dbInstance.Status.DBInstancePort = pointer.Int64(9000)
									dbInstance.Status.DBIResourceID = pointer.String("test-id")
									dbInstance.Status.DomainMemberships = []*rdsv1alpha1.DomainMembership{
										{
											Domain:      pointer.String("test-domain"),
											FQDN:        pointer.String("test-fqdn"),
											IAMRoleName: pointer.String("test-name"),
											Status:      pointer.String("test-status"),
										},
									}
									dbInstance.Status.EnabledCloudwatchLogsExports = []*string{
										pointer.String("test-export"),
									}
									dbInstance.Status.Endpoint = &rdsv1alpha1.Endpoint{
										Address:      pointer.String("test-address"),
										HostedZoneID: pointer.String("test-id"),
										Port:         pointer.Int64(9000),
									}
									dbInstance.Status.EnhancedMonitoringResourceARN = pointer.String("test-arn")
									dbInstance.Status.IAMDatabaseAuthenticationEnabled = pointer.Bool(false)
									dbInstance.Status.InstanceCreateTime = &tm
									dbInstance.Status.LatestRestorableTime = &tm
									dbInstance.Status.ListenerEndpoint = &rdsv1alpha1.Endpoint{
										Address:      pointer.String("test-address"),
										HostedZoneID: pointer.String("test-id"),
										Port:         pointer.Int64(9000),
									}
									dbInstance.Status.OptionGroupMemberships = []*rdsv1alpha1.OptionGroupMembership{
										{
											OptionGroupName: pointer.String("test-name"),
											Status:          pointer.String("test-status"),
										},
									}
									dbInstance.Status.PendingModifiedValues = &rdsv1alpha1.PendingModifiedValues{
										AllocatedStorage:                 pointer.Int64(25),
										AutomationMode:                   pointer.String("test-mode"),
										BackupRetentionPeriod:            pointer.Int64(100),
										CACertificateIdentifier:          pointer.String("test-identifier"),
										DBInstanceClass:                  pointer.String("test-class"),
										DBInstanceIdentifier:             pointer.String("test-identifier"),
										DBSubnetGroupName:                pointer.String("test-name"),
										EngineVersion:                    pointer.String("test-version"),
										IAMDatabaseAuthenticationEnabled: pointer.Bool(false),
										IOPS:                             pointer.Int64(50),
										LicenseModel:                     pointer.String("test-model"),
										MasterUserPassword:               pointer.String("test-password"),
										MultiAZ:                          pointer.Bool(false),
										PendingCloudwatchLogsExports: &rdsv1alpha1.PendingCloudwatchLogsExports{
											LogTypesToDisable: []*string{
												pointer.String("test-disable"),
											},
											LogTypesToEnable: []*string{
												pointer.String("test-enable"),
											},
										},
										Port: pointer.Int64(9000),
										ProcessorFeatures: []*rdsv1alpha1.ProcessorFeature{
											{
												Name:  pointer.String("test-name"),
												Value: pointer.String("test-value"),
											},
										},
										ResumeFullAutomationModeTime: &tm,
										StorageType:                  pointer.String("test-type"),
									}
									dbInstance.Status.ReadReplicaDBClusterIdentifiers = []*string{
										pointer.String("test-identifier"),
									}
									dbInstance.Status.ReadReplicaDBInstanceIdentifiers = []*string{
										pointer.String("test-identifier"),
									}
									dbInstance.Status.ReadReplicaSourceDBInstanceIdentifier = pointer.String("test-identifier")
									dbInstance.Status.ResumeFullAutomationModeTime = &tm
									dbInstance.Status.SecondaryAvailabilityZone = pointer.String("test-zone")
									dbInstance.Status.StatusInfos = []*rdsv1alpha1.DBInstanceStatusInfo{
										{
											Message:    pointer.String("test-message"),
											Normal:     pointer.Bool(false),
											Status:     pointer.String("test-status"),
											StatusType: pointer.String("test-type"),
										},
									}
									dbInstance.Status.TagList = []*rdsv1alpha1.Tag{
										{
											Key:   pointer.String("test-key"),
											Value: pointer.String("test-value"),
										},
									}
									dbInstance.Status.VPCSecurityGroups = []*rdsv1alpha1.VPCSecurityGroupMembership{
										{
											Status:             pointer.String("test-status"),
											VPCSecurityGroupID: pointer.String("test-id"),
										},
									}
									if err := k8sClient.Status().Update(ctx, dbInstance); err != nil {
										return false
									}
									return true
								}, timeout).Should(BeTrue())

								ins := &rdsdbaasv1alpha1.RDSInstance{
									ObjectMeta: metav1.ObjectMeta{
										Name:      instanceName,
										Namespace: testNamespace,
									},
								}
								Eventually(func() bool {
									if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ins), ins); err != nil {
										return false
									}
									if ins.Status.Phase != phase {
										return false
									}
									condition := apimeta.FindStatusCondition(ins.Status.Conditions, "ProvisionReady")
									if condition == nil || string(condition.Status) != cond ||
										condition.Reason != reason || condition.Message != message {
										return false
									}

									condition1 := apimeta.FindStatusCondition(ins.Status.Conditions, "test-condition-1")
									if condition1 == nil {
										return false
									}
									Expect(string(condition1.Status)).Should(Equal("False"))
									Expect(condition1.Reason).Should(Equal("test:reason"))
									Expect(condition1.Message).Should(Equal("test-message"))

									condition2 := apimeta.FindStatusCondition(ins.Status.Conditions, "test-condition-2")
									if condition2 == nil {
										return false
									}
									Expect(string(condition2.Status)).Should(Equal("True"))
									Expect(condition2.Reason).Should(Equal("DBInstance"))
									Expect(condition2.Message).Should(Equal("test-message"))

									condition3 := apimeta.FindStatusCondition(ins.Status.Conditions, "test-condition-3")
									if condition3 == nil {
										return false
									}
									Expect(string(condition3.Status)).Should(Equal("True"))
									Expect(condition3.Reason).Should(Equal("DBInstance"))
									Expect(condition3.Message).Should(Equal("Reason: test++reason--, Message: test-message"))

									condition4 := apimeta.FindStatusCondition(ins.Status.Conditions, "test-condition-4")
									if condition4 == nil {
										return false
									}
									Expect(string(condition4.Status)).Should(Equal("False"))
									Expect(condition4.Reason).Should(Equal("DBInstance"))
									Expect(condition4.Message).Should(Equal("Reason: test##reason&&"))

									Expect(strings.HasPrefix(ins.Status.InstanceID, "rhoda-postgres-")).Should(BeTrue())
									id := strings.TrimPrefix(ins.Status.InstanceID, "rhoda-postgres-")
									_, err := uuid.Parse(id)
									Expect(err).ShouldNot(HaveOccurred())

									if ins.Status.InstanceInfo == nil || len(ins.Status.InstanceInfo) == 0 {
										return false
									}
									info := map[string]string{
										"engine":                                        "postgres",
										"engineVersion":                                 "13.2",
										"ackResourceMetadata.arn":                       "test-arn",
										"ackResourceMetadata.ownerAccountID":            "test-id",
										"ackResourceMetadata.region":                    "test-region",
										"activityStreamEngineNativeAuditFieldsIncluded": "false",
										"activityStreamKinesisStreamName":               "test-name",
										"activityStreamKMSKeyID":                        "test-id",
										"activityStreamMode":                            "test-mode",
										"activityStreamStatus":                          "test-status",
										"associatedRoles[0].featureName":                "test-name",
										"associatedRoles[0].roleARN":                    "test-arn",
										"associatedRoles[0].status":                     "test-status",
										"automaticRestartTime":                          dbInstance.Status.AutomaticRestartTime.String(),
										"automationMode":                                "test-mode",
										"awsBackupRecoveryPointARN":                     "test-arn",
										"caCertificateIdentifier":                       "test-identifier",
										"customerOwnedIPEnabled":                        "false",
										"dbInstanceAutomatedBackupsReplications[0].dbInstanceAutomatedBackupsARN": "test-arn",
										"dbParameterGroups[0].dbParameterGroupName":                               "test-name",
										"dbParameterGroups[0].parameterApplyStatus":                               "test-status",
										"dbSubnetGroup.dbSubnetGroupARN":                                          "test-arn",
										"dbSubnetGroup.dbSubnetGroupDescription":                                  "test-desc",
										"dbSubnetGroup.dbSubnetGroupName":                                         "test-name",
										"dbSubnetGroup.subnetGroupStatus":                                         "test-status",
										"dbSubnetGroup.subnets[0].subnetAvailabilityZone.name":                    "test-name",
										"dbSubnetGroup.subnets[0].subnetIdentifier":                               "test-identifier",
										"dbSubnetGroup.subnets[0].subnetOutpost.arn":                              "test-arn",
										"dbSubnetGroup.subnets[0].subnetStatus":                                   "test-status",
										"dbSubnetGroup.vpcID":                                                     "test-id",
										"dbInstancePort":                                                          "9000",
										"dbiResourceID":                                                           "test-id",
										"domainMemberships[0].domain":                                             "test-domain",
										"domainMemberships[0].fQDN":                                               "test-fqdn",
										"domainMemberships[0].iamRoleName":                                        "test-name",
										"domainMemberships[0].status":                                             "test-status",
										"enabledCloudwatchLogsExports[0]":                                         "test-export",
										"endpoint.address":                                                        "test-address",
										"endpoint.hostedZoneID":                                                   "test-id",
										"endpoint.port":                                                           "9000",
										"enhancedMonitoringResourceARN":                                           "test-arn",
										"iamDatabaseAuthenticationEnabled":                                        "false",
										"instanceCreateTime":                                                      dbInstance.Status.InstanceCreateTime.String(),
										"latestRestorableTime":                                                    dbInstance.Status.LatestRestorableTime.String(),
										"listenerEndpoint.address":                                                "test-address",
										"listenerEndpoint.hostedZoneID":                                           "test-id",
										"listenerEndpoint.port":                                                   "9000",
										"optionGroupMemberships[0].optionGroupName":                               "test-name",
										"optionGroupMemberships[0].status":                                        "test-status",
										"pendingModifiedValues.allocatedStorage":                                  "25",
										"pendingModifiedValues.automationMode":                                    "test-mode",
										"pendingModifiedValues.backupRetentionPeriod":                             "100",
										"pendingModifiedValues.caCertificateIdentifier":                           "test-identifier",
										"pendingModifiedValues.dbInstanceClass":                                   "test-class",
										"pendingModifiedValues.dbInstanceIdentifier":                              "test-identifier",
										"pendingModifiedValues.dbSubnetGroupName":                                 "test-name",
										"pendingModifiedValues.engineVersion":                                     "test-version",
										"pendingModifiedValues.iamDatabaseAuthenticationEnabled":                  "false",
										"pendingModifiedValues.iops":                                              "50",
										"pendingModifiedValues.licenseModel":                                      "test-model",
										"pendingModifiedValues.masterUserPassword":                                "test-password",
										"pendingModifiedValues.multiAZ":                                           "false",
										"pendingModifiedValues.pendingCloudwatchLogsExports.logTypesToDisable[0]": "test-disable",
										"pendingModifiedValues.pendingCloudwatchLogsExports.logTypesToEnable[0]":  "test-enable",
										"pendingModifiedValues.port":                                              "9000",
										"pendingModifiedValues.ProcessorFeature[0].name":                          "test-name",
										"pendingModifiedValues.ProcessorFeature[0].value":                         "test-value",
										"pendingModifiedValues.resumeFullAutomationModeTime":                      dbInstance.Status.PendingModifiedValues.ResumeFullAutomationModeTime.String(),
										"pendingModifiedValues.storageType":                                       "test-type",
										"readReplicaDBClusterIdentifiers[0]":                                      "test-identifier",
										"readReplicaDBInstanceIdentifiers[0]":                                     "test-identifier",
										"readReplicaSourceDBInstanceIdentifier":                                   "test-identifier",
										"resumeFullAutomationModeTime":                                            dbInstance.Status.ResumeFullAutomationModeTime.String(),
										"secondaryAvailabilityZone":                                               "test-zone",
										"statusInfos[0].message":                                                  "test-message",
										"statusInfos[0].normal":                                                   "false",
										"statusInfos[0].status":                                                   "test-status",
										"statusInfos[0].statusType":                                               "test-type",
										"tagList[0].key":                                                          "test-key",
										"tagList[0].value":                                                        "test-value",
										"vpcSecurityGroups[0].status":                                             "test-status",
										"vpcSecurityGroups[0].vpcSecurityGroupID":                                 "test-id",
									}
									if instanceStatus != nil {
										info["dbInstanceStatus"] = *instanceStatus
									}
									Expect(ins.Status.InstanceInfo).Should(Equal(info))

									return true
								}, timeout).Should(BeTrue())
							},
							Entry("available", pointer.String("available"), "Ready", "True", "Ready", ""),
							Entry("creating", pointer.String("creating"), "Creating", "Unknown", "Updating", "Updating Instance"),
							Entry("deleting", pointer.String("deleting"), "Deleting", "Unknown", "Updating", "Updating Instance"),
							Entry("failed", pointer.String("failed"), "Failed", "False", "Terminated", "Failed"),
							Entry("inaccessible-encryption-credentials-recoverable", pointer.String("inaccessible-encryption-credentials-recoverable"), "Error", "False", "BackendError", "Instance with error"),
							Entry("incompatible-parameters", pointer.String("incompatible-parameters"), "Error", "False", "BackendError", "Instance with error"),
							Entry("restore-error", pointer.String("restore-error"), "Error", "False", "BackendError", "Instance with error"),
							Entry("backing-up", pointer.String("backing-up"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("configuring-enhanced-monitoring", pointer.String("configuring-enhanced-monitoring"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("configuring-iam-database-auth", pointer.String("configuring-iam-database-auth"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("configuring-log-exports", pointer.String("configuring-log-exports"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("converting-to-vpc", pointer.String("converting-to-vpc"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("moving-to-vpc", pointer.String("moving-to-vpc"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("rebooting", pointer.String("rebooting"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("resetting-master-credentials", pointer.String("resetting-master-credentials"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("renaming", pointer.String("renaming"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("starting", pointer.String("starting"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("stopping", pointer.String("stopping"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("storage-optimization", pointer.String("storage-optimization"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("upgrading", pointer.String("upgrading"), "Updating", "Unknown", "Updating", "Updating Instance"),
							Entry("inaccessible-encryption-credentials", pointer.String("inaccessible-encryption-credentials"), "Unknown", "False", "BackendError", "Instance with error"),
							Entry("incompatible-network", pointer.String("incompatible-network"), "Unknown", "False", "BackendError", "Instance with error"),
							Entry("incompatible-option-group", pointer.String("incompatible-option-group"), "Unknown", "False", "BackendError", "Instance with error"),
							Entry("incompatible-restore", pointer.String("incompatible-restore"), "Unknown", "False", "BackendError", "Instance with error"),
							Entry("insufficient-capacity", pointer.String("insufficient-capacity"), "Unknown", "False", "BackendError", "Instance with error"),
							Entry("stopped", pointer.String("stopped"), "Unknown", "False", "BackendError", "Instance with error"),
							Entry("storage-full", pointer.String("storage-full"), "Unknown", "False", "BackendError", "Instance with error"),
							Entry("blank", pointer.String(""), "Unknown", "False", "BackendError", "Instance with error"),
							Entry("nil", nil, "Unknown", "False", "BackendError", "Instance with error"),
						)
					})
				})
			})
		})
	})
})
