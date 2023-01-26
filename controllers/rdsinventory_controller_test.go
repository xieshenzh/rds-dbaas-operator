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

	dbaasv1alpha1 "github.com/RHEcosystemAppEng/dbaas-operator/api/v1alpha1"
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
				Spec: dbaasv1alpha1.DBaaSInventorySpec{
					CredentialsRef: &dbaasv1alpha1.LocalObjectReference{
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

				// Deleting
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

				// Not adopted, DB instance exists
				mockDBInstance9 := &rdsv1alpha1.DBInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mock-db-instance-9",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBInstanceSpec{
						Engine:               pointer.String("mysql"),
						DBInstanceIdentifier: pointer.String("mock-db-instance-9"),
						DBInstanceClass:      pointer.String("db.t3.micro"),
					},
				}
				BeforeEach(assertResourceCreation(mockDBInstance9))
				AfterEach(assertResourceDeletionIfExist(mockDBInstance9))
				BeforeEach(func() {
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mockDBInstance9), mockDBInstance9); err != nil {
							return false
						}
						mockDBInstance9.Status.DBInstanceStatus = pointer.String("available")
						arn := ackv1alpha1.AWSResourceName("mock-db-instance-9")
						ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
						region := ackv1alpha1.AWSRegion("us-east-1")
						mockDBInstance9.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
							ARN:            &arn,
							OwnerAccountID: &ownerAccountID,
							Region:         &region,
						}
						err := k8sClient.Status().Update(ctx, mockDBInstance9)
						return err == nil
					}, timeout).Should(BeTrue())
				})

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

				Context("when Inventory is created", func() {
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

					It("should start the RDS controller, adopt the DB instances and sync DB instance status", func() {
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

						By("checking if DB instances are adopted")
						Eventually(func() bool {
							adoptedDBInstances := &ackv1alpha1.AdoptedResourceList{}
							if err := k8sClient.List(ctx, adoptedDBInstances, client.InNamespace(testNamespace)); err != nil {
								return false
							}
							if len(adoptedDBInstances.Items) < 6 {
								return false
							}
							dbInstancesMap := map[string]ackv1alpha1.AdoptedResource{}
							for i := range adoptedDBInstances.Items {
								instance := adoptedDBInstances.Items[i]
								dbInstancesMap[instance.Spec.AWS.NameOrID] = instance
							}

							if instance, ok := dbInstancesMap["mock-db-instance-1"]; !ok {
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
							if instance, ok := dbInstancesMap["mock-db-instance-2"]; !ok {
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
							if instance, ok := dbInstancesMap["mock-db-instance-3"]; !ok {
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
							if instance, ok := dbInstancesMap["mock-db-instance-4"]; !ok {
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
							if instance, ok := dbInstancesMap["mock-db-instance-7"]; !ok {
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
							if instance, ok := dbInstancesMap["mock-db-instance-8"]; !ok {
								return false
							} else {
								if instance.Name != "mock-db-instance-8" {
									return false
								}
								for i := range adoptedDBInstances.Items {
									ins := adoptedDBInstances.Items[i]
									if ins.Spec.AWS.NameOrID == "mock-db-instance-8" {
										if ins.Name != "mock-db-instance-8" {
											return false
										}
									}
								}
							}
							if _, ok := dbInstancesMap["mock-db-instance-9"]; ok {
								return false
							}
							if _, ok := dbInstancesMap["mock-db-instance-5"]; ok {
								return false
							}
							if _, ok := dbInstancesMap["mock-db-cluster-1"]; ok {
								return false
							}
							if _, ok := dbInstancesMap["mock-db-cluster-2"]; ok {
								return false
							}
							if _, ok := dbInstancesMap["mock-db-aurora-1"]; ok {
								return false
							}
							if _, ok := dbInstancesMap["mock-db-aurora-2"]; ok {
								return false
							}
							if _, ok := dbInstancesMap["mock-db-aurora-3"]; ok {
								return false
							}
							if _, ok := dbInstancesMap["mock-db-custom-1"]; ok {
								return false
							}
							if _, ok := dbInstancesMap["mock-db-custom-2"]; ok {
								return false
							}
							if _, ok := dbInstancesMap["mock-db-custom-3"]; ok {
								return false
							}
							if _, ok := dbInstancesMap["mock-db-custom-4"]; ok {
								return false
							}

							By("making DB instances adopted")
							for i := range adoptedDBInstances.Items {
								instance := adoptedDBInstances.Items[i]
								if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&instance), &instance); err != nil {
									return false
								}
								instance.Status.Conditions = []*ackv1alpha1.Condition{
									{
										Type:   ackv1alpha1.ConditionTypeAdopted,
										Status: v1.ConditionTrue,
									},
								}
								if err := k8sClient.Status().Update(ctx, &instance); err != nil {
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
							if len(inv.Status.Instances) < 9 {
								return false
							}
							instancesMap := map[string]dbaasv1alpha1.Instance{}
							for i := range inv.Status.Instances {
								ins := inv.Status.Instances[i]
								instancesMap[ins.InstanceID] = ins
							}
							if _, ok := instancesMap[*dbInstance1.Spec.DBInstanceIdentifier]; ok {
								return false
							}
							if _, ok := instancesMap[*dbInstance2.Spec.DBInstanceIdentifier]; ok {
								return false
							}
							if ins, ok := instancesMap[*dbInstance3.Spec.DBInstanceIdentifier]; !ok {
								return false
							} else {
								Expect(ins.Name).Should(Equal(dbInstance3.Name))
								s, ok := ins.InstanceInfo["dbInstanceStatus"]
								Expect(ok).Should(BeTrue())
								Expect(s).Should(Equal(*dbInstance3.Status.DBInstanceStatus))
							}
							if ins, ok := instancesMap[*dbInstance4.Spec.DBInstanceIdentifier]; !ok {
								return false
							} else {
								Expect(ins.Name).Should(Equal(dbInstance4.Name))
								s, ok := ins.InstanceInfo["dbInstanceStatus"]
								Expect(ok).Should(BeTrue())
								Expect(s).Should(Equal(*dbInstance4.Status.DBInstanceStatus))
							}
							if ins, ok := instancesMap[*dbInstance5.Spec.DBInstanceIdentifier]; !ok {
								return false
							} else {
								Expect(ins.Name).Should(Equal(dbInstance5.Name))
								s, ok := ins.InstanceInfo["dbInstanceStatus"]
								Expect(ok).Should(BeTrue())
								Expect(s).Should(Equal(*dbInstance5.Status.DBInstanceStatus))
							}
							if _, ok := instancesMap[*dbInstance14.Spec.DBInstanceIdentifier]; ok {
								return false
							}
							if _, ok := instancesMap[*dbInstance15.Spec.DBInstanceIdentifier]; !ok {
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
					Spec: dbaasv1alpha1.DBaaSInventorySpec{
						CredentialsRef: &dbaasv1alpha1.LocalObjectReference{
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
				Spec: dbaasv1alpha1.DBaaSInventorySpec{
					CredentialsRef: &dbaasv1alpha1.LocalObjectReference{
						Name: credentialName,
					},
				},
			}
			BeforeEach(assertResourceCreation(inventory))
			AfterEach(assertResourceDeletion(inventory))

			It("should start the RDS controller, adopt the DB instances and the inventory should not be ready", func() {
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

				By("checking if DB instances are adopted")
				Eventually(func() bool {
					adoptedDBInstances := &ackv1alpha1.AdoptedResourceList{}
					if err := k8sClient.List(ctx, adoptedDBInstances, client.InNamespace(testNamespace)); err != nil {
						return false
					}
					if len(adoptedDBInstances.Items) < 10 {
						return false
					}
					dbInstancesMap := map[string]ackv1alpha1.AdoptedResource{}
					for i := range adoptedDBInstances.Items {
						instance := adoptedDBInstances.Items[i]
						dbInstancesMap[instance.Spec.AWS.NameOrID] = instance
					}

					if _, ok := dbInstancesMap["mock-db-instance-1"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-2"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-3"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-4"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-7"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-8"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-9"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-adopted-db-instance-3"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-adopted-db-instance-5"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-adopted-db-instance-15"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-5"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-adopted-db-instance-4"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-cluster-1"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-cluster-2"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-aurora-1"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-aurora-2"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-aurora-3"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-custom-1"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-custom-2"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-custom-3"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-custom-4"]; ok {
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
					if len(inv.Status.Instances) > 0 {
						return false
					}
					return true
				}, timeout).Should(BeTrue())
			})

			It("should start the RDS controller, adopt the DB instances and the inventory should be ready", func() {
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

				By("checking if DB instances are adopted")
				Eventually(func() bool {
					adoptedDBInstances := &ackv1alpha1.AdoptedResourceList{}
					if err := k8sClient.List(ctx, adoptedDBInstances, client.InNamespace(testNamespace)); err != nil {
						return false
					}
					if len(adoptedDBInstances.Items) < 10 {
						return false
					}
					dbInstancesMap := map[string]ackv1alpha1.AdoptedResource{}
					for i := range adoptedDBInstances.Items {
						instance := adoptedDBInstances.Items[i]
						dbInstancesMap[instance.Spec.AWS.NameOrID] = instance
					}

					if _, ok := dbInstancesMap["mock-db-instance-1"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-2"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-3"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-4"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-7"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-8"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-9"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-adopted-db-instance-3"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-adopted-db-instance-5"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-adopted-db-instance-15"]; !ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-instance-5"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-adopted-db-instance-4"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-cluster-1"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-cluster-2"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-aurora-1"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-aurora-2"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-aurora-3"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-custom-1"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-custom-2"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-custom-3"]; ok {
						return false
					}
					if _, ok := dbInstancesMap["mock-db-custom-4"]; ok {
						return false
					}

					By("making DB instances adopted")
					for i := range adoptedDBInstances.Items {
						instance := adoptedDBInstances.Items[i]
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&instance), &instance); err != nil {
							return false
						}
						instance.Status.Conditions = []*ackv1alpha1.Condition{
							{
								Type:   ackv1alpha1.ConditionTypeAdopted,
								Status: v1.ConditionTrue,
							},
						}
						if err := k8sClient.Status().Update(ctx, &instance); err != nil {
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
					if len(inv.Status.Instances) > 0 {
						return false
					}
					return true
				}, timeout).Should(BeTrue())
			})
		})
	})
})
