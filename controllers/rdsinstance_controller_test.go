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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/RHEcosystemAppEng/dbaas-operator/api/v1alpha1"
	rdsdbaasv1alpha1 "github.com/RHEcosystemAppEng/rds-dbaas-operator/api/v1alpha1"
	rdsv1alpha1 "github.com/aws-controllers-k8s/rds-controller/apis/v1alpha1"
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
					"Engine":               "postgres",
					"DBInstanceIdentifier": "rds-instance-instance-controller",
					"DBInstanceClass":      "db.t3.micro",
					"AllocatedStorage":     "20",
					"EngineVersion":        "13.2",
					"StorageType":          "gp2",
					"IOPS":                 "1000",
					"MaxAllocatedStorage":  "50",
					"DBSubnetGroupName":    "default",
					"PubliclyAccessible":   "false",
					"VPCSecurityGroupIDs":  "default",
					"LicenseModel":         "license-included",
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
					CredentialsRef: &dbaasv1alpha1.NamespacedName{
						Name:      credentialName,
						Namespace: testNamespace,
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
				accessKey := "AKIAIOSFODNN7EXAMPLEINVENTORYCONTROLLER"
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
								"DBInstanceIdentifier": "rds-instance-instance-controller",
								"DBInstanceClass":      "db.t3.micro",
								"AllocatedStorage":     "20",
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
							Name:          instanceName + "-engine",
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

					It("should make Instance in error status", func() {
						ins := &rdsdbaasv1alpha1.RDSInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      instanceName + "-identifier",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ins), ins); err != nil {
								return false
							}
							condition := apimeta.FindStatusCondition(ins.Status.Conditions, "ProvisionReady")
							if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "InputError" ||
								condition.Message != "Failed to create or update DB Instance: required parameter DBInstanceIdentifier is missing" {
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
							Name:          instanceName + "-engine",
							CloudProvider: "AWS",
							CloudRegion:   "us-east-1a",
							OtherInstanceParams: map[string]string{
								"Engine":               "postgres",
								"DBInstanceIdentifier": "rds-instance-instance-controller",
								"AllocatedStorage":     "20",
							},
						},
					}
					BeforeEach(assertResourceCreation(instanceClass))
					AfterEach(assertResourceDeletion(instanceClass))

					It("should make Instance in error status", func() {
						ins := &rdsdbaasv1alpha1.RDSInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      instanceName + "-class",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ins), ins); err != nil {
								return false
							}
							condition := apimeta.FindStatusCondition(ins.Status.Conditions, "ProvisionReady")
							if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "InputError" ||
								condition.Message != "Failed to create or update DB Instance: required parameter DBInstanceClass is missing" {
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
							Name:          instanceName + "-engine",
							CloudProvider: "AWS",
							CloudRegion:   "us-east-1a",
							OtherInstanceParams: map[string]string{
								"Engine":               "postgres",
								"DBInstanceIdentifier": "rds-instance-instance-controller",
								"DBInstanceClass":      "db.t3.micro",
							},
						},
					}
					BeforeEach(assertResourceCreation(instanceStorage))
					AfterEach(assertResourceDeletion(instanceStorage))

					It("should make Instance in error status", func() {
						ins := &rdsdbaasv1alpha1.RDSInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      instanceName + "-storage",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ins), ins); err != nil {
								return false
							}
							condition := apimeta.FindStatusCondition(ins.Status.Conditions, "ProvisionReady")
							if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "InputError" ||
								condition.Message != "Failed to create or update DB Instance: required parameter AllocatedStorage is missing" {
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
							Name:          instanceName + "-engine",
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

					It("should make Instance in error status", func() {
						ins := &rdsdbaasv1alpha1.RDSInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      instanceName + "-region",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ins), ins); err != nil {
								return false
							}
							condition := apimeta.FindStatusCondition(ins.Status.Conditions, "ProvisionReady")
							if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "InputError" ||
								condition.Message != "Failed to create or update DB Instance: required parameter CloudRegion is missing" {
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
								Expect(*dbInstance.Spec.DBInstanceIdentifier).Should(Equal("rds-instance-instance-controller"))
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
								err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret), dbSecret)
								Expect(err).ShouldNot(HaveOccurred())
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

					Context("when checking the status of the Instance", func() {

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
				})
			})
		})
	})
})
