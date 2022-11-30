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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1beta1 "github.com/RHEcosystemAppEng/dbaas-operator/api/v1beta1"
	rdsdbaasv1alpha1 "github.com/RHEcosystemAppEng/rds-dbaas-operator/api/v1alpha1"
	"github.com/RHEcosystemAppEng/rds-dbaas-operator/controllers/rds/test"
	rdsv1alpha1 "github.com/aws-controllers-k8s/rds-controller/apis/v1alpha1"
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ophandler "github.com/operator-framework/operator-lib/handler"
)

var _ = Describe("RDSInventoryController", func() {
	Context("when invalid AWS credentials are used", func() {
		credentialName := "credentials-ref-invalid-inventory-controller"
		inventoryName := "rds-inventory-invalid-inventory-controller"

		accessKey := "AKIAIOSFODNN7EXAMPLEINVENTORYCONTROLLERINVALID"
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

		Context("when Inventory is created", func() {
			inventory := &rdsdbaasv1alpha1.RDSInventory{
				ObjectMeta: metav1.ObjectMeta{
					Name:      inventoryName,
					Namespace: testNamespace,
				},
				Spec: dbaasv1beta1.DBaaSInventorySpec{
					CredentialsRef: &dbaasv1beta1.LocalObjectReference{
						Name: credentialName,
					},
				},
			}
			BeforeEach(assertResourceCreation(inventory))
			AfterEach(assertResourceDeletion(inventory))

			It("should make Inventory in error status", func() {
				inv := &rdsdbaasv1alpha1.RDSInventory{
					ObjectMeta: metav1.ObjectMeta{
						Name:      inventoryName,
						Namespace: testNamespace,
					},
				}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(inv), inv); err != nil {
						return false
					}
					condition := apimeta.FindStatusCondition(inv.Status.Conditions, "SpecSynced")
					if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "InputError" ||
						condition.Message != "The AWS service account is not valid for accessing RDS DB instances" {
						return false
					}
					return true
				}, timeout).Should(BeTrue())
			})
		})
	})

	Context("when Secret for launching RDS controller is created", func() {
		credentialName := "credentials-ref-inventory-controller"
		inventoryName := "rds-inventory-inventory-controller"

		accessKey := "AKIAIOSFODNN7EXAMPLE" + test.InventoryControllerTestAccessKeySuffix
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

		Context("when checking the status of the Inventory", func() {
			// Not exist in AWS
			dbInstance1 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-1",
					Namespace: testNamespace,
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("postgres"),
					DBInstanceIdentifier: pointer.String("db-instance-1"),
					DBInstanceClass:      pointer.String("db.t3.micro"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance1))
			AfterEach(assertResourceDeletion(dbInstance1))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance1), dbInstance1); err != nil {
						return false
					}
					dbInstance1.Status.DBInstanceStatus = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("db-instance-1")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance1.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance1)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Not exist in AWS
			dbInstance2 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-2",
					Namespace: testNamespace,
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("mysql"),
					DBInstanceIdentifier: pointer.String("db-instance-2"),
					DBInstanceClass:      pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance2))
			AfterEach(assertResourceDeletion(dbInstance2))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance2), dbInstance2); err != nil {
						return false
					}
					dbInstance2.Status.DBInstanceStatus = pointer.String("creating")
					arn := ackv1alpha1.AWSResourceName("db-instance-2")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance2.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance2)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Available
			dbInstance3 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-3",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("mariadb"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-3"),
					DBInstanceClass:      pointer.String("db.t3.micro"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance3))
			AfterEach(assertResourceDeletion(dbInstance3))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance3), dbInstance3); err != nil {
						return false
					}
					dbInstance3.Status.DBInstanceStatus = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-3")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance3.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance3)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Deleting
			dbInstance4 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-4",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("postgres"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-4"),
					DBInstanceClass:      pointer.String("db.t3.micro"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance4))
			AfterEach(assertResourceDeletion(dbInstance4))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance4), dbInstance4); err != nil {
						return false
					}
					dbInstance4.Status.DBInstanceStatus = pointer.String("deleting")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-4")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance4.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance4)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Creating
			dbInstance5 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-5",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("mysql"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-5"),
					DBInstanceClass:      pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance5))
			AfterEach(assertResourceDeletion(dbInstance5))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance5), dbInstance5); err != nil {
						return false
					}
					dbInstance5.Status.DBInstanceStatus = pointer.String("creating")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-5")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance5.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance5)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Cluster
			dbInstance6 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-6",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("mysql"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-6"),
					DBInstanceClass:      pointer.String("db.t3.small"),
					DBClusterIdentifier:  pointer.String("test-cluster"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance6))
			AfterEach(assertResourceDeletion(dbInstance6))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance6), dbInstance6); err != nil {
						return false
					}
					dbInstance6.Status.DBInstanceStatus = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-6")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance6.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance6)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Aurora
			dbInstance7 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-7",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("aurora"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-7"),
					DBInstanceClass:      pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance7))
			AfterEach(assertResourceDeletion(dbInstance7))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance7), dbInstance7); err != nil {
						return false
					}
					dbInstance7.Status.DBInstanceStatus = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-7")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance7.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance7)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Aurora
			dbInstance8 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-8",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("aurora-mysql"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-8"),
					DBInstanceClass:      pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance8))
			AfterEach(assertResourceDeletion(dbInstance8))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance8), dbInstance8); err != nil {
						return false
					}
					dbInstance8.Status.DBInstanceStatus = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-8")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance8.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance8)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Aurora
			dbInstance9 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-9",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("aurora-postgresql"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-9"),
					DBInstanceClass:      pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance9))
			AfterEach(assertResourceDeletion(dbInstance9))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance9), dbInstance9); err != nil {
						return false
					}
					dbInstance9.Status.DBInstanceStatus = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-9")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance9.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance9)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Custom Oracle
			dbInstance10 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-10",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("custom-oracle-ee"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-10"),
					DBInstanceClass:      pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance10))
			AfterEach(assertResourceDeletion(dbInstance10))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance10), dbInstance10); err != nil {
						return false
					}
					dbInstance10.Status.DBInstanceStatus = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-10")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance10.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance10)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Custom SqlServer
			dbInstance11 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-11",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("custom-sqlserver-ee"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-11"),
					DBInstanceClass:      pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance11))
			AfterEach(assertResourceDeletion(dbInstance11))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance11), dbInstance11); err != nil {
						return false
					}
					dbInstance11.Status.DBInstanceStatus = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-11")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance11.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance11)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Custom SqlServer
			dbInstance12 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-12",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("custom-sqlserver-se"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-12"),
					DBInstanceClass:      pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance12))
			AfterEach(assertResourceDeletion(dbInstance12))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance12), dbInstance12); err != nil {
						return false
					}
					dbInstance12.Status.DBInstanceStatus = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-12")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance12.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance12)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Custom SqlServer
			dbInstance13 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-13",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("custom-sqlserver-web"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-13"),
					DBInstanceClass:      pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance13))
			AfterEach(assertResourceDeletion(dbInstance13))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance13), dbInstance13); err != nil {
						return false
					}
					dbInstance13.Status.DBInstanceStatus = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-13")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance13.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance13)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Not exist in AWS
			dbInstance14 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-14",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("mysql"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-14"),
					DBInstanceClass:      pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance14))
			AfterEach(assertResourceDeletion(dbInstance14))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance14), dbInstance14); err != nil {
						return false
					}
					dbInstance14.Status.DBInstanceStatus = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-14")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance14.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance14)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// ARN not match AWS
			dbInstance15 := &rdsv1alpha1.DBInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-instance-inventory-controller-15",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBInstanceSpec{
					Engine:               pointer.String("mysql"),
					DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-15"),
					DBInstanceClass:      pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbInstance15))
			AfterEach(assertResourceDeletion(dbInstance15))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance15), dbInstance15); err != nil {
						return false
					}
					dbInstance15.Status.DBInstanceStatus = pointer.String("creating")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-15")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbInstance15.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbInstance15)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Not exist in AWS
			dbCluster1 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-1",
					Namespace: testNamespace,
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("postgres"),
					DBClusterIdentifier:    pointer.String("db-cluster-1"),
					DBClusterInstanceClass: pointer.String("db.t3.micro"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster1))
			AfterEach(assertResourceDeletion(dbCluster1))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster1), dbCluster1); err != nil {
						return false
					}
					dbCluster1.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("db-cluster-1")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster1.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster1)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Not exist in AWS
			dbCluster2 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-2",
					Namespace: testNamespace,
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("mysql"),
					DBClusterIdentifier:    pointer.String("db-cluster-2"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster2))
			AfterEach(assertResourceDeletion(dbCluster2))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster2), dbCluster2); err != nil {
						return false
					}
					dbCluster2.Status.Status = pointer.String("creating")
					arn := ackv1alpha1.AWSResourceName("db-cluster-2")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster2.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster2)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Available
			dbCluster3 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-3",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("mysql"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-3"),
					DBClusterInstanceClass: pointer.String("db.t3.micro"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster3))
			AfterEach(assertResourceDeletion(dbCluster3))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster3), dbCluster3); err != nil {
						return false
					}
					dbCluster3.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-3")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster3.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster3)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Deleting
			dbCluster4 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-4",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("postgres"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-4"),
					DBClusterInstanceClass: pointer.String("db.t3.micro"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster4))
			AfterEach(assertResourceDeletion(dbCluster4))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster4), dbCluster4); err != nil {
						return false
					}
					dbCluster4.Status.Status = pointer.String("deleting")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-4")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster4.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster4)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Creating
			dbCluster5 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-5",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("mysql"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-5"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster5))
			AfterEach(assertResourceDeletion(dbCluster5))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster5), dbCluster5); err != nil {
						return false
					}
					dbCluster5.Status.Status = pointer.String("creating")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-5")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster5.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster5)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// MariaDB
			dbCluster6 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-6",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("mariadb"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-6"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster6))
			AfterEach(assertResourceDeletion(dbCluster6))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster6), dbCluster6); err != nil {
						return false
					}
					dbCluster6.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-6")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster6.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster6)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Oracle
			dbCluster7 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-7",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("oracle-se2"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-7"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster7))
			AfterEach(assertResourceDeletion(dbCluster7))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster7), dbCluster7); err != nil {
						return false
					}
					dbCluster7.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-7")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster7.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster7)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Oracle
			dbCluster8 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-8",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("oracle-se2-cdb"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-8"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster8))
			AfterEach(assertResourceDeletion(dbCluster8))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster8), dbCluster8); err != nil {
						return false
					}
					dbCluster8.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-8")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster8.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster8)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Oracle
			dbCluster9 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-9",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("oracle-ee"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-9"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster9))
			AfterEach(assertResourceDeletion(dbCluster9))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster9), dbCluster9); err != nil {
						return false
					}
					dbCluster9.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-9")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster9.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster9)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Oracle
			dbCluster10 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-10",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("oracle-ee-cdb"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-10"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster10))
			AfterEach(assertResourceDeletion(dbCluster10))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster10), dbCluster10); err != nil {
						return false
					}
					dbCluster10.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-10")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster10.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster10)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Oracle
			dbCluster11 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-11",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("custom-oracle-ee"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-11"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster11))
			AfterEach(assertResourceDeletion(dbCluster11))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster11), dbCluster11); err != nil {
						return false
					}
					dbCluster11.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-11")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster11.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster11)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// SqlServer
			dbCluster12 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-12",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("sqlserver-ee"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-12"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster12))
			AfterEach(assertResourceDeletion(dbCluster12))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster12), dbCluster12); err != nil {
						return false
					}
					dbCluster12.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-12")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster12.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster12)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// SqlServer
			dbCluster13 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-13",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("sqlserver-se"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-13"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster13))
			AfterEach(assertResourceDeletion(dbCluster13))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster13), dbCluster13); err != nil {
						return false
					}
					dbCluster13.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-13")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster13.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster13)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// SqlServer
			dbCluster14 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-14",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("sqlserver-ex"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-14"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster14))
			AfterEach(assertResourceDeletion(dbCluster14))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster14), dbCluster14); err != nil {
						return false
					}
					dbCluster14.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-14")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster14.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster14)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// SqlServer
			dbCluster15 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-15",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("sqlserver-web"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-15"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster15))
			AfterEach(assertResourceDeletion(dbCluster15))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster15), dbCluster15); err != nil {
						return false
					}
					dbCluster15.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-15")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster15.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster15)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// SqlServer
			dbCluster16 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-16",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("custom-sqlserver-ee"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-16"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster16))
			AfterEach(assertResourceDeletion(dbCluster16))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster16), dbCluster16); err != nil {
						return false
					}
					dbCluster16.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-16")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster16.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster16)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// SqlServer
			dbCluster17 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-17",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("custom-sqlserver-se"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-17"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster17))
			AfterEach(assertResourceDeletion(dbCluster17))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster17), dbCluster17); err != nil {
						return false
					}
					dbCluster17.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-17")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster17.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster17)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// SqlServer
			dbCluster18 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-18",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("custom-sqlserver-web"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-18"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster18))
			AfterEach(assertResourceDeletion(dbCluster18))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster18), dbCluster18); err != nil {
						return false
					}
					dbCluster18.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-18")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster18.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster18)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// Not exist in AWS
			dbCluster19 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-19",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("mysql"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-19"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster19))
			AfterEach(assertResourceDeletion(dbCluster19))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster19), dbCluster19); err != nil {
						return false
					}
					dbCluster19.Status.Status = pointer.String("available")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-19")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster19.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster19)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			// ARN not match AWS
			dbCluster20 := &rdsv1alpha1.DBCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db-cluster-inventory-controller-20",
					Namespace: testNamespace,
					Labels: map[string]string{
						"rds.dbaas.redhat.com/adopted": "true",
					},
				},
				Spec: rdsv1alpha1.DBClusterSpec{
					Engine:                 pointer.String("mysql"),
					DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-20"),
					DBClusterInstanceClass: pointer.String("db.t3.small"),
				},
			}
			BeforeEach(assertResourceCreation(dbCluster20))
			AfterEach(assertResourceDeletion(dbCluster20))
			BeforeEach(func() {
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster20), dbCluster20); err != nil {
						return false
					}
					dbCluster20.Status.Status = pointer.String("creating")
					arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-20")
					ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
					region := ackv1alpha1.AWSRegion("us-east-1")
					dbCluster20.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
						ARN:            &arn,
						OwnerAccountID: &ownerAccountID,
						Region:         &region,
					}
					err := k8sClient.Status().Update(ctx, dbCluster20)
					return err == nil
				}, timeout).Should(BeTrue())
			})

			Context("when mocking RDS controller adopting resources", func() {
				// Available
				mockDBInstance1 := &rdsv1alpha1.DBInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-postgres-mock-db-instance-1",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBInstanceSpec{
						Engine:               pointer.String("postgres"),
						DBInstanceIdentifier: pointer.String("mock-db-instance-1"),
						DBInstanceClass:      pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(mockDBInstance1))

				// Available
				mockDBInstance2 := &rdsv1alpha1.DBInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-postgres-mock-db-instance-2",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBInstanceSpec{
						Engine:               pointer.String("postgres"),
						DBInstanceIdentifier: pointer.String("mock-db-instance-2"),
						DBInstanceClass:      pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(mockDBInstance2))

				// Available
				mockDBInstance3 := &rdsv1alpha1.DBInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-postgres-mock-db-instance-3",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBInstanceSpec{
						Engine:               pointer.String("postgres"),
						DBInstanceIdentifier: pointer.String("mock-db-instance-3"),
						DBInstanceClass:      pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(mockDBInstance3))

				// Available
				mockDBInstance4 := &rdsv1alpha1.DBInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-postgres-mock-db-instance-4",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBInstanceSpec{
						Engine:               pointer.String("postgres"),
						DBInstanceIdentifier: pointer.String("mock-db-instance-4"),
						DBInstanceClass:      pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(mockDBInstance4))

				// Not adopted, DB instance exists
				mockDBInstance6 := &rdsv1alpha1.DBInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mock-db-instance-6",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBInstanceSpec{
						Engine:               pointer.String("mysql"),
						DBInstanceIdentifier: pointer.String("mock-db-instance-6"),
						DBInstanceClass:      pointer.String("db.t3.micro"),
					},
				}
				BeforeEach(assertResourceCreation(mockDBInstance6))
				AfterEach(assertResourceDeletionIfExist(mockDBInstance6))
				BeforeEach(func() {
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBInstance6), mockDBInstance6); err != nil {
							return false
						}
						mockDBInstance6.Status.DBInstanceStatus = pointer.String("available")
						arn := ackv1alpha1.AWSResourceName("mock-db-instance-6")
						ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
						region := ackv1alpha1.AWSRegion("us-east-1")
						mockDBInstance6.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
							ARN:            &arn,
							OwnerAccountID: &ownerAccountID,
							Region:         &region,
						}
						err := k8sClient.Status().Update(ctx, mockDBInstance6)
						return err == nil
					}, timeout).Should(BeTrue())
				})

				// Adopted ARN not match AWS
				mockDBInstance7 := &rdsv1alpha1.DBInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-mysql-mock-db-instance-7",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBInstanceSpec{
						Engine:               pointer.String("mysql"),
						DBInstanceIdentifier: pointer.String("mock-db-instance-7"),
						DBInstanceClass:      pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(mockDBInstance7))
				adoptedMockInstance7ARN := ackv1alpha1.AWSResourceName("mock-db-instance-7-0")
				adoptedMockInstance7 := &ackv1alpha1.AdoptedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-mock-db-instance-7",
						Namespace: testNamespace,
					},
					Spec: ackv1alpha1.AdoptedResourceSpec{
						AWS: &ackv1alpha1.AWSIdentifiers{
							NameOrID: "mock-db-instance-7",
							ARN:      &adoptedMockInstance7ARN,
						},
						Kubernetes: &ackv1alpha1.ResourceWithMetadata{
							GroupKind: metav1.GroupKind{
								Group: rdsv1alpha1.GroupVersion.Group,
								Kind:  "DBInstance",
							},
						},
					},
				}
				BeforeEach(assertResourceCreation(adoptedMockInstance7))
				AfterEach(assertResourceDeletionIfExist(adoptedMockInstance7))

				// Not adopted, already adopted
				adoptedMockInstance8ARN := ackv1alpha1.AWSResourceName("mock-db-instance-8")
				adoptedMockInstance8 := &ackv1alpha1.AdoptedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mock-db-instance-8",
						Namespace: testNamespace,
					},
					Spec: ackv1alpha1.AdoptedResourceSpec{
						AWS: &ackv1alpha1.AWSIdentifiers{
							NameOrID: "mock-db-instance-8",
							ARN:      &adoptedMockInstance8ARN,
						},
						Kubernetes: &ackv1alpha1.ResourceWithMetadata{
							GroupKind: metav1.GroupKind{
								Group: rdsv1alpha1.GroupVersion.Group,
								Kind:  "DBInstance",
							},
						},
					},
				}
				BeforeEach(assertResourceCreation(adoptedMockInstance8))
				AfterEach(assertResourceDeletionIfExist(adoptedMockInstance8))

				// ARN match AWS
				dbInstance15_0 := &rdsv1alpha1.DBInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mock-adopted-db-instance-15-0",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBInstanceSpec{
						Engine:               pointer.String("mysql"),
						DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-15"),
						DBInstanceClass:      pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(dbInstance15_0))

				// Available
				mockDBCluster1 := &rdsv1alpha1.DBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-postgres-mock-db-cluster-1",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBClusterSpec{
						Engine:                 pointer.String("postgres"),
						DBClusterIdentifier:    pointer.String("mock-db-cluster-1"),
						DBClusterInstanceClass: pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(mockDBCluster1))

				// Available
				mockDBCluster2 := &rdsv1alpha1.DBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-postgres-mock-db-cluster-2",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBClusterSpec{
						Engine:                 pointer.String("mysql"),
						DBClusterIdentifier:    pointer.String("mock-db-cluster-2"),
						DBClusterInstanceClass: pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(mockDBCluster2))

				// Available
				mockDBCluster3 := &rdsv1alpha1.DBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-postgres-mock-db-cluster-3",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBClusterSpec{
						Engine:                 pointer.String("aurora"),
						DBClusterIdentifier:    pointer.String("mock-db-cluster-3"),
						DBClusterInstanceClass: pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(mockDBCluster3))

				// Available
				mockDBCluster4 := &rdsv1alpha1.DBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-postgres-mock-db-cluster-4",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBClusterSpec{
						Engine:                 pointer.String("aurora-postgresql"),
						DBClusterIdentifier:    pointer.String("mock-db-cluster-4"),
						DBClusterInstanceClass: pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(mockDBCluster4))

				// Not adopted, DB cluster exists
				mockDBCluster6 := &rdsv1alpha1.DBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mock-db-cluster-6",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBClusterSpec{
						Engine:                 pointer.String("aurora"),
						DBClusterIdentifier:    pointer.String("mock-db-cluster-6"),
						DBClusterInstanceClass: pointer.String("db.t3.micro"),
					},
				}
				BeforeEach(assertResourceCreation(mockDBCluster6))
				AfterEach(assertResourceDeletionIfExist(mockDBCluster6))
				BeforeEach(func() {
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBCluster6), mockDBCluster6); err != nil {
							return false
						}
						mockDBCluster6.Status.Status = pointer.String("available")
						arn := ackv1alpha1.AWSResourceName("mock-db-cluster-6")
						ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
						region := ackv1alpha1.AWSRegion("us-east-1")
						mockDBCluster6.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
							ARN:            &arn,
							OwnerAccountID: &ownerAccountID,
							Region:         &region,
						}
						err := k8sClient.Status().Update(ctx, mockDBCluster6)
						return err == nil
					}, timeout).Should(BeTrue())
				})

				// Adopted ARN not match AWS
				mockDBCluster7 := &rdsv1alpha1.DBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-mysql-mock-db-cluster-7",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBClusterSpec{
						Engine:                 pointer.String("aurora-mysql"),
						DBClusterIdentifier:    pointer.String("mock-db-cluster-7"),
						DBClusterInstanceClass: pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(mockDBCluster7))
				adoptedMockCluster7ARN := ackv1alpha1.AWSResourceName("mock-db-cluster-7-0")
				adoptedMockCluster7 := &ackv1alpha1.AdoptedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rhoda-adopted-mock-db-cluster-7",
						Namespace: testNamespace,
					},
					Spec: ackv1alpha1.AdoptedResourceSpec{
						AWS: &ackv1alpha1.AWSIdentifiers{
							NameOrID: "mock-db-cluster-7",
							ARN:      &adoptedMockCluster7ARN,
						},
						Kubernetes: &ackv1alpha1.ResourceWithMetadata{
							GroupKind: metav1.GroupKind{
								Group: rdsv1alpha1.GroupVersion.Group,
								Kind:  "DBCluster",
							},
						},
					},
				}
				BeforeEach(assertResourceCreation(adoptedMockCluster7))
				AfterEach(assertResourceDeletionIfExist(adoptedMockCluster7))

				// Not adopted, already adopted
				adoptedMockCluster8ARN := ackv1alpha1.AWSResourceName("mock-db-cluster-8")
				adoptedMockCluster8 := &ackv1alpha1.AdoptedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mock-db-cluster-8",
						Namespace: testNamespace,
					},
					Spec: ackv1alpha1.AdoptedResourceSpec{
						AWS: &ackv1alpha1.AWSIdentifiers{
							NameOrID: "mock-db-cluster-8",
							ARN:      &adoptedMockCluster8ARN,
						},
						Kubernetes: &ackv1alpha1.ResourceWithMetadata{
							GroupKind: metav1.GroupKind{
								Group: rdsv1alpha1.GroupVersion.Group,
								Kind:  "DBCluster",
							},
						},
					},
				}
				BeforeEach(assertResourceCreation(adoptedMockCluster8))
				AfterEach(assertResourceDeletionIfExist(adoptedMockCluster8))

				// ARN match AWS
				dbCluster20_0 := &rdsv1alpha1.DBCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mock-adopted-db-cluster-20-0",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBClusterSpec{
						Engine:                 pointer.String("mysql"),
						DBClusterIdentifier:    pointer.String("mock-adopted-db-cluster-20"),
						DBClusterInstanceClass: pointer.String("db.t3.micro"),
					},
				}
				AfterEach(assertResourceDeletionIfExist(dbCluster20_0))

				Context("when Inventory is created", func() {
					inventory := &rdsdbaasv1alpha1.RDSInventory{
						ObjectMeta: metav1.ObjectMeta{
							Name:      inventoryName,
							Namespace: testNamespace,
						},
						Spec: dbaasv1beta1.DBaaSInventorySpec{
							CredentialsRef: &dbaasv1beta1.LocalObjectReference{
								Name: credentialName,
							},
						},
					}
					BeforeEach(assertResourceCreation(inventory))
					AfterEach(assertResourceDeletion(inventory))

					It("should start the RDS controller, adopt the DB instances and clusters, and sync DB instance status", func() {
						By("checking if the Secret for RDS controller is created")
						rdsSecret := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "ack-rds-user-secrets",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsSecret), rdsSecret); err != nil {
								return false
							}
							ak, akOk := rdsSecret.Data["AWS_ACCESS_KEY_ID"]
							Expect(akOk).Should(BeTrue())
							if string(ak) != accessKey {
								return false
							}
							sk, skOk := rdsSecret.Data["AWS_SECRET_ACCESS_KEY"]
							Expect(skOk).Should(BeTrue())
							return string(sk) == secretKey
						}, timeout).Should(BeTrue())

						By("checking if the ConfigMap for RDS controller is created")
						rdsConfigMap := &v1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "ack-rds-user-config",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsConfigMap), rdsConfigMap); err != nil {
								return false
							}
							r, ok := rdsConfigMap.Data["AWS_REGION"]
							Expect(ok).Should(BeTrue())
							return r == region
						}, timeout).Should(BeTrue())

						By("checking if the RDS controller is started")
						deployment := &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "ack-rds-controller",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
								return false
							}
							return *deployment.Spec.Replicas == 1 && deployment.Status.Replicas == 1 && deployment.Status.ReadyReplicas == 1
						}, timeout).Should(BeTrue())

						By("checking if DB instances and clusters are adopted")
						Eventually(func() bool {
							adoptedDBResources := &ackv1alpha1.AdoptedResourceList{}
							if err := k8sClient.List(ctx, adoptedDBResources, client.InNamespace(testNamespace)); err != nil {
								return false
							}
							if len(adoptedDBResources.Items) < 12 {
								return false
							}
							dbResourcesMap := map[string]ackv1alpha1.AdoptedResource{}
							for i := range adoptedDBResources.Items {
								db := adoptedDBResources.Items[i]
								if r, ok := dbResourcesMap[db.Spec.AWS.NameOrID]; ok && len(r.Name) > len(db.Name) {
									continue
								}
								dbResourcesMap[db.Spec.AWS.NameOrID] = db
							}

							if instance, ok := dbResourcesMap["mock-db-instance-1"]; !ok {
								return false
							} else {
								if !strings.HasPrefix(instance.Name, "rhoda-adopted-postgres-mock-db-instance-1") {
									return false
								}
								typeString, typeOk := instance.GetAnnotations()[ophandler.TypeAnnotation]
								Expect(typeOk).Should(BeTrue())
								Expect(typeString).Should(Equal("RDSInventory.dbaas.redhat.com"))
								namespacedNameString, nsnOk := instance.GetAnnotations()[ophandler.NamespacedNameAnnotation]
								Expect(nsnOk).Should(BeTrue())
								Expect(namespacedNameString).Should(Equal(testNamespace + "/" + inventoryName))
								Expect(instance.Spec.Kubernetes.GroupKind.Kind).Should(Equal("DBInstance"))
								Expect(instance.Spec.Kubernetes.GroupKind.Group).Should(Equal(rdsv1alpha1.GroupVersion.Group))
								Expect(instance.Spec.Kubernetes.Metadata.Namespace).Should(Equal(testNamespace))
								label, labelOk := instance.Spec.Kubernetes.Metadata.Labels["rds.dbaas.redhat.com/adopted"]
								Expect(labelOk).Should(BeTrue())
								Expect(label).Should(Equal("true"))
								Expect(instance.Spec.AWS.NameOrID).Should(Equal("mock-db-instance-1"))
								Expect(instance.Spec.AWS.ARN).ShouldNot(BeNil())
								Expect(string(*instance.Spec.AWS.ARN)).Should(Equal("mock-db-instance-1"))
							}
							if instance, ok := dbResourcesMap["mock-db-instance-2"]; !ok {
								return false
							} else {
								if !strings.HasPrefix(instance.Name, "rhoda-adopted-postgres-mock-db-instance-2") {
									return false
								}
								typeString, typeOk := instance.GetAnnotations()[ophandler.TypeAnnotation]
								Expect(typeOk).Should(BeTrue())
								Expect(typeString).Should(Equal("RDSInventory.dbaas.redhat.com"))
								namespacedNameString, nsnOk := instance.GetAnnotations()[ophandler.NamespacedNameAnnotation]
								Expect(nsnOk).Should(BeTrue())
								Expect(namespacedNameString).Should(Equal(testNamespace + "/" + inventoryName))
								Expect(instance.Spec.Kubernetes.GroupKind.Kind).Should(Equal("DBInstance"))
								Expect(instance.Spec.Kubernetes.GroupKind.Group).Should(Equal(rdsv1alpha1.GroupVersion.Group))
								Expect(instance.Spec.Kubernetes.Metadata.Namespace).Should(Equal(testNamespace))
								label, labelOk := instance.Spec.Kubernetes.Metadata.Labels["rds.dbaas.redhat.com/adopted"]
								Expect(labelOk).Should(BeTrue())
								Expect(label).Should(Equal("true"))
								Expect(instance.Spec.AWS.NameOrID).Should(Equal("mock-db-instance-2"))
								Expect(instance.Spec.AWS.ARN).ShouldNot(BeNil())
								Expect(string(*instance.Spec.AWS.ARN)).Should(Equal("mock-db-instance-2"))
							}
							if instance, ok := dbResourcesMap["mock-db-instance-3"]; !ok {
								return false
							} else {
								if !strings.HasPrefix(instance.Name, "rhoda-adopted-postgres-mock-db-instance-3") {
									return false
								}
								typeString, typeOk := instance.GetAnnotations()[ophandler.TypeAnnotation]
								Expect(typeOk).Should(BeTrue())
								Expect(typeString).Should(Equal("RDSInventory.dbaas.redhat.com"))
								namespacedNameString, nsnOk := instance.GetAnnotations()[ophandler.NamespacedNameAnnotation]
								Expect(nsnOk).Should(BeTrue())
								Expect(namespacedNameString).Should(Equal(testNamespace + "/" + inventoryName))
								Expect(instance.Spec.Kubernetes.GroupKind.Kind).Should(Equal("DBInstance"))
								Expect(instance.Spec.Kubernetes.GroupKind.Group).Should(Equal(rdsv1alpha1.GroupVersion.Group))
								Expect(instance.Spec.Kubernetes.Metadata.Namespace).Should(Equal(testNamespace))
								label, labelOk := instance.Spec.Kubernetes.Metadata.Labels["rds.dbaas.redhat.com/adopted"]
								Expect(labelOk).Should(BeTrue())
								Expect(label).Should(Equal("true"))
								Expect(instance.Spec.AWS.NameOrID).Should(Equal("mock-db-instance-3"))
								Expect(instance.Spec.AWS.ARN).ShouldNot(BeNil())
								Expect(string(*instance.Spec.AWS.ARN)).Should(Equal("mock-db-instance-3"))
							}
							if instance, ok := dbResourcesMap["mock-db-instance-4"]; !ok {
								return false
							} else {
								if !strings.HasPrefix(instance.Name, "rhoda-adopted-postgres-mock-db-instance-4") {
									return false
								}
								typeString, typeOk := instance.GetAnnotations()[ophandler.TypeAnnotation]
								Expect(typeOk).Should(BeTrue())
								Expect(typeString).Should(Equal("RDSInventory.dbaas.redhat.com"))
								namespacedNameString, nsnOk := instance.GetAnnotations()[ophandler.NamespacedNameAnnotation]
								Expect(nsnOk).Should(BeTrue())
								Expect(namespacedNameString).Should(Equal(testNamespace + "/" + inventoryName))
								Expect(instance.Spec.Kubernetes.GroupKind.Kind).Should(Equal("DBInstance"))
								Expect(instance.Spec.Kubernetes.GroupKind.Group).Should(Equal(rdsv1alpha1.GroupVersion.Group))
								Expect(instance.Spec.Kubernetes.Metadata.Namespace).Should(Equal(testNamespace))
								label, labelOk := instance.Spec.Kubernetes.Metadata.Labels["rds.dbaas.redhat.com/adopted"]
								Expect(labelOk).Should(BeTrue())
								Expect(label).Should(Equal("true"))
								Expect(instance.Spec.AWS.NameOrID).Should(Equal("mock-db-instance-4"))
								Expect(instance.Spec.AWS.ARN).ShouldNot(BeNil())
								Expect(string(*instance.Spec.AWS.ARN)).Should(Equal("mock-db-instance-4"))
							}
							if instance, ok := dbResourcesMap["mock-db-instance-7"]; !ok {
								return false
							} else {
								if !strings.HasPrefix(instance.Name, "rhoda-adopted-mysql-mock-db-instance-7") {
									return false
								}
								typeString, typeOk := instance.GetAnnotations()[ophandler.TypeAnnotation]
								Expect(typeOk).Should(BeTrue())
								Expect(typeString).Should(Equal("RDSInventory.dbaas.redhat.com"))
								namespacedNameString, nsnOk := instance.GetAnnotations()[ophandler.NamespacedNameAnnotation]
								Expect(nsnOk).Should(BeTrue())
								Expect(namespacedNameString).Should(Equal(testNamespace + "/" + inventoryName))
								Expect(instance.Spec.Kubernetes.GroupKind.Kind).Should(Equal("DBInstance"))
								Expect(instance.Spec.Kubernetes.GroupKind.Group).Should(Equal(rdsv1alpha1.GroupVersion.Group))
								Expect(instance.Spec.Kubernetes.Metadata.Namespace).Should(Equal(testNamespace))
								label, labelOk := instance.Spec.Kubernetes.Metadata.Labels["rds.dbaas.redhat.com/adopted"]
								Expect(labelOk).Should(BeTrue())
								Expect(label).Should(Equal("true"))
								Expect(instance.Spec.AWS.NameOrID).Should(Equal("mock-db-instance-7"))
								Expect(instance.Spec.AWS.ARN).ShouldNot(BeNil())
								Expect(string(*instance.Spec.AWS.ARN)).Should(Equal("mock-db-instance-7"))
							}
							if instance, ok := dbResourcesMap["mock-db-instance-8"]; !ok {
								return false
							} else {
								if instance.Name != "mock-db-instance-8" {
									return false
								}
								for i := range adoptedDBResources.Items {
									ins := adoptedDBResources.Items[i]
									if ins.Spec.AWS.NameOrID == "mock-db-instance-8" {
										if ins.Name != "mock-db-instance-8" {
											return false
										}
									}
								}
							}
							if _, ok := dbResourcesMap["mock-db-instance-6"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-instance-5"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-instance-cluster-1"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-instance-cluster-2"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-aurora-1"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-aurora-2"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-aurora-3"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-custom-1"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-custom-2"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-custom-3"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-custom-4"]; ok {
								return false
							}

							if cluster, ok := dbResourcesMap["mock-db-cluster-1"]; !ok {
								return false
							} else {
								if !strings.HasPrefix(cluster.Name, "rhoda-adopted-postgres-mock-db-cluster-1") {
									return false
								}
								typeString, typeOk := cluster.GetAnnotations()[ophandler.TypeAnnotation]
								Expect(typeOk).Should(BeTrue())
								Expect(typeString).Should(Equal("RDSInventory.dbaas.redhat.com"))
								namespacedNameString, nsnOk := cluster.GetAnnotations()[ophandler.NamespacedNameAnnotation]
								Expect(nsnOk).Should(BeTrue())
								Expect(namespacedNameString).Should(Equal(testNamespace + "/" + inventoryName))
								Expect(cluster.Spec.Kubernetes.GroupKind.Kind).Should(Equal("DBCluster"))
								Expect(cluster.Spec.Kubernetes.GroupKind.Group).Should(Equal(rdsv1alpha1.GroupVersion.Group))
								Expect(cluster.Spec.Kubernetes.Metadata.Namespace).Should(Equal(testNamespace))
								label, labelOk := cluster.Spec.Kubernetes.Metadata.Labels["rds.dbaas.redhat.com/adopted"]
								Expect(labelOk).Should(BeTrue())
								Expect(label).Should(Equal("true"))
								Expect(cluster.Spec.AWS.NameOrID).Should(Equal("mock-db-cluster-1"))
								Expect(cluster.Spec.AWS.ARN).ShouldNot(BeNil())
								Expect(string(*cluster.Spec.AWS.ARN)).Should(Equal("mock-db-cluster-1"))
							}
							if cluster, ok := dbResourcesMap["mock-db-cluster-2"]; !ok {
								return false
							} else {
								if !strings.HasPrefix(cluster.Name, "rhoda-adopted-mysql-mock-db-cluster-2") {
									return false
								}
								typeString, typeOk := cluster.GetAnnotations()[ophandler.TypeAnnotation]
								Expect(typeOk).Should(BeTrue())
								Expect(typeString).Should(Equal("RDSInventory.dbaas.redhat.com"))
								namespacedNameString, nsnOk := cluster.GetAnnotations()[ophandler.NamespacedNameAnnotation]
								Expect(nsnOk).Should(BeTrue())
								Expect(namespacedNameString).Should(Equal(testNamespace + "/" + inventoryName))
								Expect(cluster.Spec.Kubernetes.GroupKind.Kind).Should(Equal("DBCluster"))
								Expect(cluster.Spec.Kubernetes.GroupKind.Group).Should(Equal(rdsv1alpha1.GroupVersion.Group))
								Expect(cluster.Spec.Kubernetes.Metadata.Namespace).Should(Equal(testNamespace))
								label, labelOk := cluster.Spec.Kubernetes.Metadata.Labels["rds.dbaas.redhat.com/adopted"]
								Expect(labelOk).Should(BeTrue())
								Expect(label).Should(Equal("true"))
								Expect(cluster.Spec.AWS.NameOrID).Should(Equal("mock-db-cluster-2"))
								Expect(cluster.Spec.AWS.ARN).ShouldNot(BeNil())
								Expect(string(*cluster.Spec.AWS.ARN)).Should(Equal("mock-db-cluster-2"))
							}
							if cluster, ok := dbResourcesMap["mock-db-cluster-3"]; !ok {
								return false
							} else {
								if !strings.HasPrefix(cluster.Name, "rhoda-adopted-aurora-mock-db-cluster-3") {
									return false
								}
								typeString, typeOk := cluster.GetAnnotations()[ophandler.TypeAnnotation]
								Expect(typeOk).Should(BeTrue())
								Expect(typeString).Should(Equal("RDSInventory.dbaas.redhat.com"))
								namespacedNameString, nsnOk := cluster.GetAnnotations()[ophandler.NamespacedNameAnnotation]
								Expect(nsnOk).Should(BeTrue())
								Expect(namespacedNameString).Should(Equal(testNamespace + "/" + inventoryName))
								Expect(cluster.Spec.Kubernetes.GroupKind.Kind).Should(Equal("DBCluster"))
								Expect(cluster.Spec.Kubernetes.GroupKind.Group).Should(Equal(rdsv1alpha1.GroupVersion.Group))
								Expect(cluster.Spec.Kubernetes.Metadata.Namespace).Should(Equal(testNamespace))
								label, labelOk := cluster.Spec.Kubernetes.Metadata.Labels["rds.dbaas.redhat.com/adopted"]
								Expect(labelOk).Should(BeTrue())
								Expect(label).Should(Equal("true"))
								Expect(cluster.Spec.AWS.NameOrID).Should(Equal("mock-db-cluster-3"))
								Expect(cluster.Spec.AWS.ARN).ShouldNot(BeNil())
								Expect(string(*cluster.Spec.AWS.ARN)).Should(Equal("mock-db-cluster-3"))
							}
							if cluster, ok := dbResourcesMap["mock-db-cluster-4"]; !ok {
								return false
							} else {
								if !strings.HasPrefix(cluster.Name, "rhoda-adopted-aurora-postgres-mock-db-cluster-4") {
									return false
								}
								typeString, typeOk := cluster.GetAnnotations()[ophandler.TypeAnnotation]
								Expect(typeOk).Should(BeTrue())
								Expect(typeString).Should(Equal("RDSInventory.dbaas.redhat.com"))
								namespacedNameString, nsnOk := cluster.GetAnnotations()[ophandler.NamespacedNameAnnotation]
								Expect(nsnOk).Should(BeTrue())
								Expect(namespacedNameString).Should(Equal(testNamespace + "/" + inventoryName))
								Expect(cluster.Spec.Kubernetes.GroupKind.Kind).Should(Equal("DBCluster"))
								Expect(cluster.Spec.Kubernetes.GroupKind.Group).Should(Equal(rdsv1alpha1.GroupVersion.Group))
								Expect(cluster.Spec.Kubernetes.Metadata.Namespace).Should(Equal(testNamespace))
								label, labelOk := cluster.Spec.Kubernetes.Metadata.Labels["rds.dbaas.redhat.com/adopted"]
								Expect(labelOk).Should(BeTrue())
								Expect(label).Should(Equal("true"))
								Expect(cluster.Spec.AWS.NameOrID).Should(Equal("mock-db-cluster-4"))
								Expect(cluster.Spec.AWS.ARN).ShouldNot(BeNil())
								Expect(string(*cluster.Spec.AWS.ARN)).Should(Equal("mock-db-cluster-4"))
							}
							if cluster, ok := dbResourcesMap["mock-db-cluster-7"]; !ok {
								return false
							} else {
								if !strings.HasPrefix(cluster.Name, "rhoda-adopted-aurora-mysql-mock-db-cluster-7") {
									return false
								}
								typeString, typeOk := cluster.GetAnnotations()[ophandler.TypeAnnotation]
								Expect(typeOk).Should(BeTrue())
								Expect(typeString).Should(Equal("RDSInventory.dbaas.redhat.com"))
								namespacedNameString, nsnOk := cluster.GetAnnotations()[ophandler.NamespacedNameAnnotation]
								Expect(nsnOk).Should(BeTrue())
								Expect(namespacedNameString).Should(Equal(testNamespace + "/" + inventoryName))
								Expect(cluster.Spec.Kubernetes.GroupKind.Kind).Should(Equal("DBCluster"))
								Expect(cluster.Spec.Kubernetes.GroupKind.Group).Should(Equal(rdsv1alpha1.GroupVersion.Group))
								Expect(cluster.Spec.Kubernetes.Metadata.Namespace).Should(Equal(testNamespace))
								label, labelOk := cluster.Spec.Kubernetes.Metadata.Labels["rds.dbaas.redhat.com/adopted"]
								Expect(labelOk).Should(BeTrue())
								Expect(label).Should(Equal("true"))
								Expect(cluster.Spec.AWS.NameOrID).Should(Equal("mock-db-cluster-7"))
								Expect(cluster.Spec.AWS.ARN).ShouldNot(BeNil())
								Expect(string(*cluster.Spec.AWS.ARN)).Should(Equal("mock-db-cluster-7"))
							}
							if cluster, ok := dbResourcesMap["mock-db-cluster-8"]; !ok {
								return false
							} else {
								if cluster.Name != "mock-db-cluster-8" {
									return false
								}
								for i := range adoptedDBResources.Items {
									ins := adoptedDBResources.Items[i]
									if ins.Spec.AWS.NameOrID == "mock-db-cluster-8" {
										if ins.Name != "mock-db-cluster-8" {
											return false
										}
									}
								}
							}
							if _, ok := dbResourcesMap["mock-db-cluster-6"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-cluster-5"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-mariadb-1"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-oracle-1"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-oracle-2"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-oracle-3"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-oracle-4"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-oracle-5"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-sqlserver-1"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-sqlserver-2"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-sqlserver-3"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-sqlserver-4"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-sqlserver-5"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-sqlserver-6"]; ok {
								return false
							}
							if _, ok := dbResourcesMap["mock-db-sqlserver-7"]; ok {
								return false
							}

							By("making DB resources adopted")
							for i := range adoptedDBResources.Items {
								resource := adoptedDBResources.Items[i]
								if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&resource), &resource); err != nil {
									return false
								}
								resource.Status.Conditions = []*ackv1alpha1.Condition{
									{
										Type:   ackv1alpha1.ConditionTypeAdopted,
										Status: v1.ConditionTrue,
									},
								}
								if err := k8sClient.Status().Update(ctx, &resource); err != nil {
									return false
								}
							}
							return true
						}, timeout).Should(BeTrue())

						assertResourceCreation(mockDBInstance1)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBInstance1), mockDBInstance1); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-db-instance-1")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							mockDBInstance1.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, mockDBInstance1)
							return err == nil
						}, timeout).Should(BeTrue())

						assertResourceCreation(mockDBInstance2)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBInstance2), mockDBInstance2); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-db-instance-2")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							mockDBInstance2.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, mockDBInstance2)
							return err == nil
						}, timeout).Should(BeTrue())

						assertResourceCreation(mockDBInstance3)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBInstance3), mockDBInstance3); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-db-instance-3")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							mockDBInstance3.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, mockDBInstance3)
							return err == nil
						}, timeout).Should(BeTrue())

						assertResourceCreation(mockDBInstance4)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBInstance4), mockDBInstance4); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-db-instance-4")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							mockDBInstance4.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, mockDBInstance4)
							return err == nil
						}, timeout).Should(BeTrue())

						assertResourceCreation(mockDBInstance7)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBInstance7), mockDBInstance7); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-db-instance-7")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							mockDBInstance7.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, mockDBInstance7)
							return err == nil
						}, timeout).Should(BeTrue())

						assertResourceCreation(dbInstance15_0)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance15_0), dbInstance15_0); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-adopted-db-instance-15-0")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							dbInstance15_0.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, dbInstance15_0)
							return err == nil
						}, timeout).Should(BeTrue())

						assertResourceCreation(mockDBCluster1)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBCluster1), mockDBCluster1); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-db-cluster-1")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							mockDBCluster1.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, mockDBCluster1)
							return err == nil
						}, timeout).Should(BeTrue())

						assertResourceCreation(mockDBCluster2)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBCluster2), mockDBCluster2); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-db-cluster-2")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							mockDBCluster2.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, mockDBCluster2)
							return err == nil
						}, timeout).Should(BeTrue())

						assertResourceCreation(mockDBCluster3)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBCluster3), mockDBCluster3); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-db-cluster-3")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							mockDBCluster3.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, mockDBCluster3)
							return err == nil
						}, timeout).Should(BeTrue())

						assertResourceCreation(mockDBCluster4)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBCluster4), mockDBCluster4); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-db-cluster-4")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							mockDBCluster4.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, mockDBCluster4)
							return err == nil
						}, timeout).Should(BeTrue())

						assertResourceCreation(mockDBCluster7)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBCluster7), mockDBCluster7); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-db-cluster-7")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							mockDBCluster7.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, mockDBCluster7)
							return err == nil
						}, timeout).Should(BeTrue())

						assertResourceCreation(dbCluster20_0)()
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster20_0), dbCluster20_0); err != nil {
								return false
							}
							arn := ackv1alpha1.AWSResourceName("mock-adopted-db-cluster-20-0")
							ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
							region := ackv1alpha1.AWSRegion("us-east-1")
							dbCluster20_0.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
								ARN:            &arn,
								OwnerAccountID: &ownerAccountID,
								Region:         &region,
							}
							err := k8sClient.Status().Update(ctx, dbCluster20_0)
							return err == nil
						}, timeout).Should(BeTrue())

						Eventually(func() bool {
							clusterDBInstanceList := &rdsv1alpha1.DBInstanceList{}
							if e := k8sClient.List(ctx, clusterDBInstanceList, client.InNamespace(inventory.Namespace)); e != nil {
								return false
							}
							dbInstanceMap := map[string]rdsv1alpha1.DBInstance{}
							for _, dbInstance := range clusterDBInstanceList.Items {
								dbInstanceMap[string(*dbInstance.Status.ACKResourceMetadata.ARN)] = dbInstance
							}
							if _, ok := dbInstanceMap["mock-db-instance-1"]; !ok {
								return false
							}
							if _, ok := dbInstanceMap["mock-db-instance-2"]; !ok {
								return false
							}
							if _, ok := dbInstanceMap["mock-db-instance-3"]; !ok {
								return false
							}
							if _, ok := dbInstanceMap["mock-db-instance-4"]; !ok {
								return false
							}
							if _, ok := dbInstanceMap["mock-db-instance-7"]; !ok {
								return false
							}
							if _, ok := dbInstanceMap["mock-adopted-db-instance-15-0"]; !ok {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						Eventually(func() bool {
							clusterDBClusterList := &rdsv1alpha1.DBClusterList{}
							if e := k8sClient.List(ctx, clusterDBClusterList, client.InNamespace(inventory.Namespace)); e != nil {
								return false
							}
							dbClusterMap := map[string]rdsv1alpha1.DBCluster{}
							for _, dbCluster := range clusterDBClusterList.Items {
								dbClusterMap[string(*dbCluster.Status.ACKResourceMetadata.ARN)] = dbCluster
							}
							if _, ok := dbClusterMap["mock-db-cluster-1"]; !ok {
								return false
							}
							if _, ok := dbClusterMap["mock-db-cluster-2"]; !ok {
								return false
							}
							if _, ok := dbClusterMap["mock-db-cluster-3"]; !ok {
								return false
							}
							if _, ok := dbClusterMap["mock-db-cluster-4"]; !ok {
								return false
							}
							if _, ok := dbClusterMap["mock-db-cluster-7"]; !ok {
								return false
							}
							if _, ok := dbClusterMap["mock-adopted-db-cluster-20-0"]; !ok {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						By("checking Inventory status")
						inv := &rdsdbaasv1alpha1.RDSInventory{
							ObjectMeta: metav1.ObjectMeta{
								Name:      inventoryName,
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(inv), inv); err != nil {
								return false
							}
							condition := apimeta.FindStatusCondition(inv.Status.Conditions, "SpecSynced")
							if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != "SyncOK" {
								return false
							}
							if len(inv.Status.DatabaseServices) < 18 {
								return false
							}
							servicesMap := map[string]dbaasv1beta1.DatabaseService{}
							for i := range inv.Status.DatabaseServices {
								ds := inv.Status.DatabaseServices[i]
								servicesMap[ds.ServiceID] = ds
							}
							if _, ok := servicesMap[*dbInstance1.Spec.DBInstanceIdentifier]; ok {
								return false
							}
							if _, ok := servicesMap[*dbInstance2.Spec.DBInstanceIdentifier]; ok {
								return false
							}
							if ds, ok := servicesMap[*dbInstance3.Spec.DBInstanceIdentifier]; !ok {
								return false
							} else {
								Expect(ds.ServiceName).Should(Equal(dbInstance3.Name))
								Expect(ds.ServiceType).ShouldNot(BeNil())
								Expect(string(*ds.ServiceType)).Should(Equal("instance"))
								s, ok := ds.ServiceInfo["dbInstanceStatus"]
								Expect(ok).Should(BeTrue())
								Expect(s).Should(Equal(*dbInstance3.Status.DBInstanceStatus))
							}
							if ds, ok := servicesMap[*dbInstance4.Spec.DBInstanceIdentifier]; !ok {
								return false
							} else {
								Expect(ds.ServiceName).Should(Equal(dbInstance4.Name))
								Expect(ds.ServiceType).ShouldNot(BeNil())
								Expect(string(*ds.ServiceType)).Should(Equal("instance"))
								s, ok := ds.ServiceInfo["dbInstanceStatus"]
								Expect(ok).Should(BeTrue())
								Expect(s).Should(Equal(*dbInstance4.Status.DBInstanceStatus))
							}
							if ds, ok := servicesMap[*dbInstance5.Spec.DBInstanceIdentifier]; !ok {
								return false
							} else {
								Expect(ds.ServiceName).Should(Equal(dbInstance5.Name))
								Expect(ds.ServiceType).ShouldNot(BeNil())
								Expect(string(*ds.ServiceType)).Should(Equal("instance"))
								s, ok := ds.ServiceInfo["dbInstanceStatus"]
								Expect(ok).Should(BeTrue())
								Expect(s).Should(Equal(*dbInstance5.Status.DBInstanceStatus))
							}
							if _, ok := servicesMap[*dbInstance14.Spec.DBInstanceIdentifier]; ok {
								return false
							}
							if _, ok := servicesMap[*dbInstance15.Spec.DBInstanceIdentifier]; !ok {
								return false
							}

							if _, ok := servicesMap[*dbCluster1.Spec.DBClusterIdentifier]; ok {
								return false
							}
							if _, ok := servicesMap[*dbCluster2.Spec.DBClusterIdentifier]; ok {
								return false
							}
							if ds, ok := servicesMap[*dbCluster3.Spec.DBClusterIdentifier]; !ok {
								return false
							} else {
								Expect(ds.ServiceName).Should(Equal(dbCluster3.Name))
								Expect(ds.ServiceType).ShouldNot(BeNil())
								Expect(string(*ds.ServiceType)).Should(Equal("cluster"))
								s, ok := ds.ServiceInfo["status"]
								Expect(ok).Should(BeTrue())
								Expect(s).Should(Equal(*dbCluster3.Status.Status))
							}
							if ds, ok := servicesMap[*dbCluster4.Spec.DBClusterIdentifier]; !ok {
								return false
							} else {
								Expect(ds.ServiceName).Should(Equal(dbCluster4.Name))
								Expect(ds.ServiceType).ShouldNot(BeNil())
								Expect(string(*ds.ServiceType)).Should(Equal("cluster"))
								s, ok := ds.ServiceInfo["status"]
								Expect(ok).Should(BeTrue())
								Expect(s).Should(Equal(*dbCluster4.Status.Status))
							}
							if ds, ok := servicesMap[*dbCluster5.Spec.DBClusterIdentifier]; !ok {
								return false
							} else {
								Expect(ds.ServiceName).Should(Equal(dbCluster5.Name))
								Expect(ds.ServiceType).ShouldNot(BeNil())
								Expect(string(*ds.ServiceType)).Should(Equal("cluster"))
								s, ok := ds.ServiceInfo["status"]
								Expect(ok).Should(BeTrue())
								Expect(s).Should(Equal(*dbCluster5.Status.Status))
							}
							if _, ok := servicesMap[*dbCluster19.Spec.DBClusterIdentifier]; ok {
								return false
							}
							if _, ok := servicesMap[*dbCluster20.Spec.DBClusterIdentifier]; !ok {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						By("checking if the password of adopted db instance is reset")
						dbInstance3 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-3",
								Namespace: testNamespace,
							},
						}
						dbSecret3 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-3-credentials",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance3), dbInstance3); err != nil {
								return false
							}
							if dbInstance3.Spec.DBName == nil {
								return false
							}
							if dbInstance3.Spec.MasterUsername == nil {
								return false
							}
							if dbInstance3.Spec.MasterUserPassword == nil {
								return false
							}
							Expect(dbInstance3.Spec.MasterUserPassword.Key).Should(Equal("password"))
							Expect(dbInstance3.Spec.MasterUserPassword.Namespace).Should(Equal(testNamespace))
							Expect(dbInstance3.Spec.MasterUserPassword.Name).Should(Equal("db-instance-inventory-controller-3-credentials"))
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret3), dbSecret3)
							Expect(err).ShouldNot(HaveOccurred())
							v, ok := dbSecret3.Data["password"]
							Expect(ok).Should(BeTrue())
							Expect(len(v)).Should(BeNumerically("==", 12))
							return true
						}, timeout).Should(BeTrue())

						By("checking if the username and password of adopted db instance is not reset when deleting")
						dbInstance4 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-4",
								Namespace: testNamespace,
							},
						}
						dbSecret4 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-4-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance4), dbInstance4); err != nil {
								return false
							}
							if dbInstance4.Spec.DBName != nil {
								return false
							}
							if dbInstance4.Spec.MasterUsername != nil {
								return false
							}
							if dbInstance4.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret4), dbSecret4)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						By("checking if the password of adopted db instance is not reset when not available")
						dbInstance5 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-5",
								Namespace: testNamespace,
							},
						}
						dbSecret5 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-5-credentials",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance5), dbInstance5); err != nil {
								return false
							}
							if dbInstance5.Spec.DBName == nil {
								return false
							}
							if dbInstance5.Spec.MasterUsername == nil {
								return false
							}
							return true
						}, timeout).Should(BeTrue())
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance5), dbInstance5); err != nil {
								return false
							}
							if dbInstance5.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret5), dbSecret5)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						By("checking if the username and password of adopted db instance is not reset when the instance not exist in AWS")
						dbInstance14 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-14",
								Namespace: testNamespace,
							},
						}
						dbSecret14 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-14-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance14), dbInstance14); err != nil {
								return false
							}
							if dbInstance14.Spec.DBName != nil {
								return false
							}
							if dbInstance14.Spec.MasterUsername != nil {
								return false
							}
							if dbInstance14.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret14), dbSecret14)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbInstance15 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-15",
								Namespace: testNamespace,
							},
						}
						dbSecret15 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-15-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance15), dbInstance15); err != nil {
								return false
							}
							if dbInstance15.Spec.DBName != nil {
								return false
							}
							if dbInstance15.Spec.MasterUsername != nil {
								return false
							}
							if dbInstance15.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret15), dbSecret15)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						By("checking if the username and password of adopted db instance is not reset when the db instance is in cluster")
						dbInstance6 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-6",
								Namespace: testNamespace,
							},
						}
						dbSecret6 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-6-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance6), dbInstance6); err != nil {
								return false
							}
							if dbInstance6.Spec.DBName != nil {
								return false
							}
							if dbInstance6.Spec.MasterUsername != nil {
								return false
							}
							if dbInstance6.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret6), dbSecret6)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						By("checking if the username and password of adopted db instance is not reset when the db instance engine is aurora or custom")
						dbInstance7 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-7",
								Namespace: testNamespace,
							},
						}
						dbSecret7 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-7-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance7), dbInstance7); err != nil {
								return false
							}
							if dbInstance7.Spec.DBName != nil {
								return false
							}
							if dbInstance7.Spec.MasterUsername != nil {
								return false
							}
							if dbInstance7.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret7), dbSecret7)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbInstance8 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-8",
								Namespace: testNamespace,
							},
						}
						dbSecret8 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-8-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance8), dbInstance8); err != nil {
								return false
							}
							if dbInstance8.Spec.DBName != nil {
								return false
							}
							if dbInstance8.Spec.MasterUsername != nil {
								return false
							}
							if dbInstance8.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret8), dbSecret8)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbInstance9 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-9",
								Namespace: testNamespace,
							},
						}
						dbSecret9 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-9-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance9), dbInstance9); err != nil {
								return false
							}
							if dbInstance9.Spec.DBName != nil {
								return false
							}
							if dbInstance9.Spec.MasterUsername != nil {
								return false
							}
							if dbInstance9.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret9), dbSecret9)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbInstance10 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-10",
								Namespace: testNamespace,
							},
						}
						dbSecret10 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-10-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance10), dbInstance10); err != nil {
								return false
							}
							if dbInstance10.Spec.DBName != nil {
								return false
							}
							if dbInstance10.Spec.MasterUsername != nil {
								return false
							}
							if dbInstance10.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret10), dbSecret10)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbInstance11 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-11",
								Namespace: testNamespace,
							},
						}
						dbSecret11 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-11-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance11), dbInstance11); err != nil {
								return false
							}
							if dbInstance11.Spec.DBName != nil {
								return false
							}
							if dbInstance11.Spec.MasterUsername != nil {
								return false
							}
							if dbInstance11.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret11), dbSecret11)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbInstance12 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-12",
								Namespace: testNamespace,
							},
						}
						dbSecret12 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-12-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance12), dbInstance12); err != nil {
								return false
							}
							if dbInstance12.Spec.DBName != nil {
								return false
							}
							if dbInstance12.Spec.MasterUsername != nil {
								return false
							}
							if dbInstance12.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret12), dbSecret12)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbInstance13 := &rdsv1alpha1.DBInstance{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-13",
								Namespace: testNamespace,
							},
						}
						dbSecret13 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-instance-inventory-controller-13-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance13), dbInstance13); err != nil {
								return false
							}
							if dbInstance13.Spec.DBName != nil {
								return false
							}
							if dbInstance13.Spec.MasterUsername != nil {
								return false
							}
							if dbInstance13.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret13), dbSecret13)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						By("checking if the password of adopted db cluster is reset")
						dbCluster3 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-3",
								Namespace: testNamespace,
							},
						}
						dbcSecret3 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-3-credentials",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster3), dbCluster3); err != nil {
								return false
							}
							if dbCluster3.Spec.DatabaseName == nil {
								return false
							}
							if dbCluster3.Spec.MasterUsername == nil {
								return false
							}
							if dbCluster3.Spec.MasterUserPassword == nil {
								return false
							}
							Expect(dbCluster3.Spec.MasterUserPassword.Key).Should(Equal("password"))
							Expect(dbCluster3.Spec.MasterUserPassword.Namespace).Should(Equal(testNamespace))
							Expect(dbCluster3.Spec.MasterUserPassword.Name).Should(Equal("db-cluster-inventory-controller-3-credentials"))
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret3), dbcSecret3)
							Expect(err).ShouldNot(HaveOccurred())
							v, ok := dbcSecret3.Data["password"]
							Expect(ok).Should(BeTrue())
							Expect(len(v)).Should(BeNumerically("==", 12))
							return true
						}, timeout).Should(BeTrue())

						By("checking if the username and password of adopted db cluster is not reset when deleting")
						dbCluster4 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-4",
								Namespace: testNamespace,
							},
						}
						dbcSecret4 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-4-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster4), dbCluster4); err != nil {
								return false
							}
							if dbCluster4.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster4.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster4.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret4), dbcSecret4)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						By("checking if the password of adopted db cluster is not reset when not available")
						dbCluster5 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-5",
								Namespace: testNamespace,
							},
						}
						dbcSecret5 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-5-credentials",
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster5), dbCluster5); err != nil {
								return false
							}
							if dbCluster5.Spec.DatabaseName == nil {
								return false
							}
							if dbCluster5.Spec.MasterUsername == nil {
								return false
							}
							return true
						}, timeout).Should(BeTrue())
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster5), dbCluster5); err != nil {
								return false
							}
							if dbCluster5.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret5), dbcSecret5)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						By("checking if the username and password of adopted db cluster is not reset when the cluster not exist in AWS")
						dbCluster19 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-19",
								Namespace: testNamespace,
							},
						}
						dbcSecret19 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-19-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster19), dbCluster19); err != nil {
								return false
							}
							if dbCluster19.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster19.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster19.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret19), dbcSecret19)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster20 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-20",
								Namespace: testNamespace,
							},
						}
						dbcSecret20 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-15-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster20), dbCluster20); err != nil {
								return false
							}
							if dbCluster20.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster20.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster20.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret20), dbcSecret20)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						By("checking if the username and password of adopted db cluster is not reset when the db cluster engine is mariadb, oracle or sql server")
						dbCluster6 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-6",
								Namespace: testNamespace,
							},
						}
						dbcSecret6 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-6-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster6), dbCluster6); err != nil {
								return false
							}
							if dbCluster6.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster6.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster6.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret6), dbcSecret6)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster7 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-7",
								Namespace: testNamespace,
							},
						}
						dbcSecret7 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-7-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster7), dbCluster7); err != nil {
								return false
							}
							if dbCluster7.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster7.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster7.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret7), dbcSecret7)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster8 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-8",
								Namespace: testNamespace,
							},
						}
						dbcSecret8 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-8-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster8), dbCluster8); err != nil {
								return false
							}
							if dbCluster8.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster8.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster8.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret8), dbcSecret8)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster9 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-9",
								Namespace: testNamespace,
							},
						}
						dbcSecret9 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-9-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster9), dbCluster9); err != nil {
								return false
							}
							if dbCluster9.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster9.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster9.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret9), dbcSecret9)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster10 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-10",
								Namespace: testNamespace,
							},
						}
						dbcSecret10 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-10-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster10), dbCluster10); err != nil {
								return false
							}
							if dbCluster10.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster10.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster10.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret10), dbcSecret10)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster11 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-11",
								Namespace: testNamespace,
							},
						}
						dbcSecret11 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-11-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster11), dbCluster11); err != nil {
								return false
							}
							if dbCluster11.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster11.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster11.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret11), dbcSecret11)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster12 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-12",
								Namespace: testNamespace,
							},
						}
						dbcSecret12 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-12-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster12), dbCluster12); err != nil {
								return false
							}
							if dbCluster12.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster12.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster12.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret12), dbcSecret12)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster13 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-13",
								Namespace: testNamespace,
							},
						}
						dbcSecret13 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-13-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster13), dbCluster13); err != nil {
								return false
							}
							if dbCluster13.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster13.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster13.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret13), dbcSecret13)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster14 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-14",
								Namespace: testNamespace,
							},
						}
						dbcSecret14 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-14-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster14), dbCluster14); err != nil {
								return false
							}
							if dbCluster14.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster14.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster14.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret14), dbcSecret14)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster15 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-15",
								Namespace: testNamespace,
							},
						}
						dbcSecret15 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-15-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster15), dbCluster15); err != nil {
								return false
							}
							if dbCluster15.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster15.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster15.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret15), dbcSecret15)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster16 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-16",
								Namespace: testNamespace,
							},
						}
						dbcSecret16 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-16-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster16), dbCluster16); err != nil {
								return false
							}
							if dbCluster16.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster16.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster16.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret16), dbcSecret16)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster17 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-17",
								Namespace: testNamespace,
							},
						}
						dbcSecret17 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-17-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster17), dbCluster17); err != nil {
								return false
							}
							if dbCluster17.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster17.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster17.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret17), dbcSecret17)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())

						dbCluster18 := &rdsv1alpha1.DBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-18",
								Namespace: testNamespace,
							},
						}
						dbcSecret18 := &v1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "db-cluster-inventory-controller-18-credentials",
								Namespace: testNamespace,
							},
						}
						Consistently(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbCluster18), dbCluster18); err != nil {
								return false
							}
							if dbCluster18.Spec.DatabaseName != nil {
								return false
							}
							if dbCluster18.Spec.MasterUsername != nil {
								return false
							}
							if dbCluster18.Spec.MasterUserPassword != nil {
								return false
							}
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbcSecret18), dbcSecret18)
							if err == nil || !errors.IsNotFound(err) {
								return false
							}
							return true
						}, timeout).Should(BeTrue())
					})

					Context("when is ACK controller is scaled down", func() {
						It("should scale the ACK controller up", func() {
							deployment := &appsv1.Deployment{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "ack-rds-controller",
									Namespace: testNamespace,
								},
							}

							By("checking if the replica is 1 and update it to 0")
							Eventually(func() bool {
								if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
									return false
								}

								if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 1 {
									return false
								}

								deployment.Spec.Replicas = pointer.Int32(0)
								if err := k8sClient.Update(ctx, deployment); err != nil {
									return false
								}
								return true
							}, timeout).Should(BeTrue())

							By("checking if the replica is updated to 1 again")
							Eventually(func() bool {
								if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
									return false
								}
								if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 1 {
									return false
								}
								return true
							}, timeout).Should(BeTrue())
						})
					})
				})
			})

			Context("when the Inventory is deleted", func() {
				inventoryDelete := &rdsdbaasv1alpha1.RDSInventory{
					ObjectMeta: metav1.ObjectMeta{
						Name:      inventoryName + "-delete",
						Namespace: testNamespace,
					},
					Spec: dbaasv1beta1.DBaaSInventorySpec{
						CredentialsRef: &dbaasv1beta1.LocalObjectReference{
							Name: credentialName,
						},
					},
				}
				BeforeEach(assertResourceCreation(inventoryDelete))

				It("should delete the owned resources and stop the RDS controller", func() {
					assertResourceDeletion(inventoryDelete)()

					By("checking if the Secret for RDS controller is reset")
					rdsSecret := &v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "ack-rds-user-secrets",
							Namespace: testNamespace,
						},
					}
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsSecret), rdsSecret); err != nil {
							return false
						}
						ak, akOk := rdsSecret.Data["AWS_ACCESS_KEY_ID"]
						Expect(akOk).Should(BeTrue())
						if string(ak) != "dummy" {
							return false
						}
						sk, skOk := rdsSecret.Data["AWS_SECRET_ACCESS_KEY"]
						Expect(skOk).Should(BeTrue())
						return string(sk) == "dummy"
					}, timeout).Should(BeTrue())

					By("checking if the ConfigMap for RDS controller is reset")
					rdsConfigMap := &v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "ack-rds-user-config",
							Namespace: testNamespace,
						},
					}
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsConfigMap), rdsConfigMap); err != nil {
							return false
						}
						r, ok := rdsConfigMap.Data["AWS_REGION"]
						Expect(ok).Should(BeTrue())
						return r == "dummy"
					}, timeout).Should(BeTrue())

					By("checking if the RDS controller is stopped")
					deployment := &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "ack-rds-controller",
							Namespace: testNamespace,
						},
					}
					Eventually(func() bool {
						err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)
						if err != nil {
							return false
						}
						return *deployment.Spec.Replicas == 0
					}, timeout).Should(BeTrue())

					By("checking if adopted resources are deleted")
					Eventually(func() bool {
						adoptedDBInstances := &ackv1alpha1.AdoptedResourceList{}
						if err := k8sClient.List(ctx, adoptedDBInstances, client.InNamespace(testNamespace)); err != nil {
							return false
						}
						return len(adoptedDBInstances.Items) == 0
					}, timeout).Should(BeTrue())
				})
			})
		})

		Context("when Inventory is created", func() {
			inventory := &rdsdbaasv1alpha1.RDSInventory{
				ObjectMeta: metav1.ObjectMeta{
					Name:      inventoryName + "-adopting",
					Namespace: testNamespace,
				},
				Spec: dbaasv1beta1.DBaaSInventorySpec{
					CredentialsRef: &dbaasv1beta1.LocalObjectReference{
						Name: credentialName,
					},
				},
			}
			BeforeEach(assertResourceCreation(inventory))
			AfterEach(assertResourceDeletion(inventory))

			It("should start the RDS controller, adopt the DB instances and clusters and the inventory should not be ready", func() {
				By("checking if the Secret for RDS controller is created")
				rdsSecret := &v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ack-rds-user-secrets",
						Namespace: testNamespace,
					},
				}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsSecret), rdsSecret); err != nil {
						return false
					}
					ak, akOk := rdsSecret.Data["AWS_ACCESS_KEY_ID"]
					Expect(akOk).Should(BeTrue())
					if string(ak) != accessKey {
						return false
					}
					sk, skOk := rdsSecret.Data["AWS_SECRET_ACCESS_KEY"]
					Expect(skOk).Should(BeTrue())
					return string(sk) == secretKey
				}, timeout).Should(BeTrue())

				By("checking if the ConfigMap for RDS controller is created")
				rdsConfigMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ack-rds-user-config",
						Namespace: testNamespace,
					},
				}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsConfigMap), rdsConfigMap); err != nil {
						return false
					}
					r, ok := rdsConfigMap.Data["AWS_REGION"]
					Expect(ok).Should(BeTrue())
					return r == region
				}, timeout).Should(BeTrue())

				By("checking if the RDS controller is started")
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ack-rds-controller",
						Namespace: testNamespace,
					},
				}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
						return false
					}
					return *deployment.Spec.Replicas == 1 && deployment.Status.Replicas == 1 && deployment.Status.ReadyReplicas == 1
				}, timeout).Should(BeTrue())

				By("checking if DB instances and clusters are adopted")
				Eventually(func() bool {
					adoptedDBResources := &ackv1alpha1.AdoptedResourceList{}
					if err := k8sClient.List(ctx, adoptedDBResources, client.InNamespace(testNamespace)); err != nil {
						return false
					}
					if len(adoptedDBResources.Items) < 20 {
						return false
					}
					dbResourcesMap := map[string]ackv1alpha1.AdoptedResource{}
					for i := range adoptedDBResources.Items {
						resource := adoptedDBResources.Items[i]
						dbResourcesMap[resource.Spec.AWS.NameOrID] = resource
					}

					if _, ok := dbResourcesMap["mock-db-instance-1"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-2"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-3"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-4"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-7"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-8"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-6"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-instance-3"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-instance-5"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-instance-15"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-5"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-instance-4"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-cluster-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-cluster-2"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-aurora-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-aurora-2"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-aurora-3"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-custom-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-custom-2"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-custom-3"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-custom-4"]; ok {
						return false
					}

					if _, ok := dbResourcesMap["mock-db-cluster-1"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-2"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-3"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-4"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-7"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-8"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-6"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-cluster-3"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-cluster-5"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-cluster-20"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-5"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-cluster-4"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-mariadb-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-oracle-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-oracle-2"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-oracle-3"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-oracle-4"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-oracle-5"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-2"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-3"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-4"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-5"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-6"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-7"]; ok {
						return false
					}

					return true
				}, timeout).Should(BeTrue())

				By("checking Inventory status")
				inv := &rdsdbaasv1alpha1.RDSInventory{
					ObjectMeta: metav1.ObjectMeta{
						Name:      inventoryName + "-adopting",
						Namespace: testNamespace,
					},
				}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(inv), inv); err != nil {
						return false
					}
					condition := apimeta.FindStatusCondition(inv.Status.Conditions, "SpecSynced")
					if condition != nil {
						return false
					}
					if len(inv.Status.DatabaseServices) > 0 {
						return false
					}
					return true
				}, timeout).Should(BeTrue())
			})

			It("should start the RDS controller, adopt the DB instances and clusters and the inventory should be ready", func() {
				By("checking if the Secret for RDS controller is created")
				rdsSecret := &v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ack-rds-user-secrets",
						Namespace: testNamespace,
					},
				}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsSecret), rdsSecret); err != nil {
						return false
					}
					ak, akOk := rdsSecret.Data["AWS_ACCESS_KEY_ID"]
					Expect(akOk).Should(BeTrue())
					if string(ak) != accessKey {
						return false
					}
					sk, skOk := rdsSecret.Data["AWS_SECRET_ACCESS_KEY"]
					Expect(skOk).Should(BeTrue())
					return string(sk) == secretKey
				}, timeout).Should(BeTrue())

				By("checking if the ConfigMap for RDS controller is created")
				rdsConfigMap := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ack-rds-user-config",
						Namespace: testNamespace,
					},
				}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsConfigMap), rdsConfigMap); err != nil {
						return false
					}
					r, ok := rdsConfigMap.Data["AWS_REGION"]
					Expect(ok).Should(BeTrue())
					return r == region
				}, timeout).Should(BeTrue())

				By("checking if the RDS controller is started")
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ack-rds-controller",
						Namespace: testNamespace,
					},
				}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
						return false
					}
					return *deployment.Spec.Replicas == 1 && deployment.Status.Replicas == 1 && deployment.Status.ReadyReplicas == 1
				}, timeout).Should(BeTrue())

				By("checking if DB instances and clusters are adopted")
				Eventually(func() bool {
					adoptedDBResources := &ackv1alpha1.AdoptedResourceList{}
					if err := k8sClient.List(ctx, adoptedDBResources, client.InNamespace(testNamespace)); err != nil {
						return false
					}
					if len(adoptedDBResources.Items) < 20 {
						return false
					}
					dbResourcesMap := map[string]ackv1alpha1.AdoptedResource{}
					for i := range adoptedDBResources.Items {
						resource := adoptedDBResources.Items[i]
						dbResourcesMap[resource.Spec.AWS.NameOrID] = resource
					}

					if _, ok := dbResourcesMap["mock-db-instance-1"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-2"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-3"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-4"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-7"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-8"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-6"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-instance-3"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-instance-5"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-instance-15"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-5"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-instance-4"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-cluster-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-instance-cluster-2"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-aurora-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-aurora-2"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-aurora-3"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-custom-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-custom-2"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-custom-3"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-custom-4"]; ok {
						return false
					}

					if _, ok := dbResourcesMap["mock-db-cluster-1"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-2"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-3"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-4"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-7"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-8"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-6"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-cluster-3"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-cluster-5"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-cluster-20"]; !ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-cluster-5"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-adopted-db-cluster-4"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-mariadb-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-oracle-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-oracle-2"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-oracle-3"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-oracle-4"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-oracle-5"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-1"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-2"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-3"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-4"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-5"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-6"]; ok {
						return false
					}
					if _, ok := dbResourcesMap["mock-db-sqlserver-7"]; ok {
						return false
					}

					By("making DB instances and clusters adopted")
					for i := range adoptedDBResources.Items {
						resource := adoptedDBResources.Items[i]
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&resource), &resource); err != nil {
							return false
						}
						resource.Status.Conditions = []*ackv1alpha1.Condition{
							{
								Type:   ackv1alpha1.ConditionTypeAdopted,
								Status: v1.ConditionTrue,
							},
						}
						if err := k8sClient.Status().Update(ctx, &resource); err != nil {
							return false
						}
					}
					return true
				}, timeout).Should(BeTrue())

				By("checking Inventory status")
				inv := &rdsdbaasv1alpha1.RDSInventory{
					ObjectMeta: metav1.ObjectMeta{
						Name:      inventoryName + "-adopting",
						Namespace: testNamespace,
					},
				}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(inv), inv); err != nil {
						return false
					}
					condition := apimeta.FindStatusCondition(inv.Status.Conditions, "SpecSynced")
					if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != "SyncOK" {
						return false
					}
					if len(inv.Status.DatabaseServices) > 0 {
						return false
					}
					return true
				}, timeout).Should(BeTrue())
			})
		})
	})
})
