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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/RHEcosystemAppEng/dbaas-operator/api/v1alpha1"
	rdsdbaasv1alpha1 "github.com/RHEcosystemAppEng/rds-dbaas-operator/api/v1alpha1"
	rdsv1alpha1 "github.com/aws-controllers-k8s/rds-controller/apis/v1alpha1"
)

var _ = Describe("RDSConnectionController", func() {
	Context("when Connection is created", func() {
		connectionName := "rds-connection-connection-controller"
		inventoryName := "rds-inventory-connection-controller"
		instanceID := "instance-id-connection-controller"

		connection := &rdsdbaasv1alpha1.RDSConnection{
			ObjectMeta: metav1.ObjectMeta{
				Name:      connectionName,
				Namespace: testNamespace,
			},
			Spec: dbaasv1alpha1.DBaaSConnectionSpec{
				InventoryRef: dbaasv1alpha1.NamespacedName{
					Name:      inventoryName,
					Namespace: testNamespace,
				},
				InstanceID: instanceID,
			},
		}
		BeforeEach(assertResourceCreation(connection))

		Context("when Inventory is not created", func() {
			AfterEach(assertResourceDeletion(connection))

			It("should make Connection in error status", func() {
				conn := &rdsdbaasv1alpha1.RDSConnection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      connectionName,
						Namespace: testNamespace,
					},
				}
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(conn), conn); err != nil {
						return false
					}
					condition := apimeta.FindStatusCondition(conn.Status.Conditions, "ReadyForBinding")
					if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "NotFound" {
						return false
					}
					return true
				}, timeout).Should(BeTrue())
			})
		})

		Context("when Inventory is created", func() {
			credentialName := "credentials-ref-connection-controller"

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
				AfterEach(assertResourceDeletion(connection))

				It("should make Connection in error status", func() {
					conn := &rdsdbaasv1alpha1.RDSConnection{
						ObjectMeta: metav1.ObjectMeta{
							Name:      connectionName,
							Namespace: testNamespace,
						},
					}
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(conn), conn); err != nil {
							return false
						}
						condition := apimeta.FindStatusCondition(conn.Status.Conditions, "ReadyForBinding")
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

				Context("when Instance is not found", func() {
					AfterEach(assertResourceDeletion(connection))

					It("should make Connection in error status", func() {
						conn := &rdsdbaasv1alpha1.RDSConnection{
							ObjectMeta: metav1.ObjectMeta{
								Name:      connectionName,
								Namespace: testNamespace,
							},
						}
						Eventually(func() bool {
							if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(conn), conn); err != nil {
								return false
							}
							condition := apimeta.FindStatusCondition(conn.Status.Conditions, "ReadyForBinding")
							if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "NotFound" {
								return false
							}
							return true
						}, timeout).Should(BeTrue())
					})
				})

				Context("when Inventory and Instance are created", func() {
					dbInstance := &rdsv1alpha1.DBInstance{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "db-instance-connection-controller",
							Namespace: testNamespace,
						},
						Spec: rdsv1alpha1.DBInstanceSpec{
							Engine:               pointer.String("postgres"),
							DBInstanceIdentifier: pointer.String("dbInstance1"),
							DBInstanceClass:      pointer.String("db.t3.micro"),
						},
					}
					BeforeEach(assertResourceCreation(dbInstance))
					AfterEach(assertResourceDeletion(dbInstance))

					Context("when Instance is not ready", func() {
						AfterEach(assertResourceDeletion(connection))

						It("should make Connection in error status", func() {
							conn := &rdsdbaasv1alpha1.RDSConnection{
								ObjectMeta: metav1.ObjectMeta{
									Name:      connectionName,
									Namespace: testNamespace,
								},
							}
							Eventually(func() bool {
								if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(conn), conn); err != nil {
									return false
								}
								condition := apimeta.FindStatusCondition(conn.Status.Conditions, "ReadyForBinding")
								if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "Unreachable" {
									return false
								}
								return true
							}, timeout).Should(BeTrue())
						})
					})

					Context("when Inventory and Instance are ready", func() {
						BeforeEach(func() {
							dbInstance.Status = rdsv1alpha1.DBInstanceStatus{
								DBInstanceStatus: pointer.String("available"),
							}
							Eventually(func() bool {
								err := k8sClient.Status().Update(ctx, dbInstance)
								return err == nil
							}, timeout).Should(BeTrue())
						})

						Context("when checking the status of the Connection", func() {
							AfterEach(assertResourceDeletion(connection))

							Context("when the Instance user password is not set", func() {

							})

							Context("when the Instance user password Secret is not created", func() {

							})

							Context("when the Instance user password Secret is not created", func() {

							})

							Context("when the Instance user password Secret is not valid", func() {

							})

							Context("when the Instance user name is not set", func() {

							})

							Context("when the Instance endpoint is not available", func() {

							})

							Context("when the Instance endpoint is not available", func() {

							})

							Context("when the Instance connection info is complete", func() {

							})
						})

						Context("when the Connection is deleted", func() {

						})
					})
				})
			})
		})
	})
})
