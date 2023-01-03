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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1beta1 "github.com/RHEcosystemAppEng/dbaas-operator/api/v1beta1"
	rdsdbaasv1alpha1 "github.com/RHEcosystemAppEng/rds-dbaas-operator/api/v1alpha1"
	"github.com/RHEcosystemAppEng/rds-dbaas-operator/controllers/rds/test"
	rdsv1alpha1 "github.com/aws-controllers-k8s/rds-controller/apis/v1alpha1"
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
)

var _ = Describe("RDSConnectionController", func() {
	Context("when DB instances are created", func() {
		instanceID := "instance-id-connection-controller"

		dbInstance := &rdsv1alpha1.DBInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db-instance-postgres-connection-controller",
				Namespace: testNamespace,
			},
			Spec: rdsv1alpha1.DBInstanceSpec{
				Engine:               pointer.String("postgres"),
				DBInstanceIdentifier: pointer.String(instanceID),
				DBInstanceClass:      pointer.String("db.t3.micro"),
			},
		}
		BeforeEach(assertResourceCreation(dbInstance))
		AfterEach(assertResourceDeletion(dbInstance))
		BeforeEach(func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
					return false
				}
				arn := ackv1alpha1.AWSResourceName(instanceID)
				ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
				region := ackv1alpha1.AWSRegion("us-east-1")
				dbInstance.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
					ARN:            &arn,
					OwnerAccountID: &ownerAccountID,
					Region:         &region,
				}
				err := k8sClient.Status().Update(ctx, dbInstance)
				return err == nil
			}, timeout).Should(BeTrue())
		})

		instanceIDOracle := "instance-id-oracle-connection-controller"
		dbInstanceOracle := &rdsv1alpha1.DBInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db-instance-oracle-connection-controller",
				Namespace: testNamespace,
			},
			Spec: rdsv1alpha1.DBInstanceSpec{
				Engine:               pointer.String("oracle-se2"),
				DBInstanceIdentifier: pointer.String(instanceIDOracle),
				DBInstanceClass:      pointer.String("db.t3.micro"),
				MasterUserPassword: &ackv1alpha1.SecretKeyReference{
					SecretReference: v1.SecretReference{
						Name:      "secret-jdbc-url-connection-controller",
						Namespace: testNamespace,
					},
					Key: "password",
				},
				MasterUsername: pointer.String("user-oracle-connection-controller"),
			},
		}
		BeforeEach(assertResourceCreation(dbInstanceOracle))
		AfterEach(assertResourceDeletion(dbInstanceOracle))
		BeforeEach(func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstanceOracle), dbInstanceOracle); err != nil {
					return false
				}
				arn := ackv1alpha1.AWSResourceName(instanceIDOracle)
				ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
				region := ackv1alpha1.AWSRegion("us-east-1")
				dbInstanceOracle.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
					ARN:            &arn,
					OwnerAccountID: &ownerAccountID,
					Region:         &region,
				}
				err := k8sClient.Status().Update(ctx, dbInstanceOracle)
				return err == nil
			}, timeout).Should(BeTrue())
		})

		instanceIDSqlServer := "instance-id-sqlserver-connection-controller"
		dbInstanceSqlServer := &rdsv1alpha1.DBInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "db-instance-sqlserver-connection-controller",
				Namespace: testNamespace,
			},
			Spec: rdsv1alpha1.DBInstanceSpec{
				Engine:               pointer.String("sqlserver-ex"),
				DBInstanceIdentifier: pointer.String(instanceIDSqlServer),
				DBInstanceClass:      pointer.String("db.t3.micro"),
				MasterUserPassword: &ackv1alpha1.SecretKeyReference{
					SecretReference: v1.SecretReference{
						Name:      "secret-jdbc-url-connection-controller",
						Namespace: testNamespace,
					},
					Key: "password",
				},
				MasterUsername: pointer.String("user-sqlserver-connection-controller"),
			},
		}
		BeforeEach(assertResourceCreation(dbInstanceSqlServer))
		AfterEach(assertResourceDeletion(dbInstanceSqlServer))
		BeforeEach(func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstanceSqlServer), dbInstanceSqlServer); err != nil {
					return false
				}
				arn := ackv1alpha1.AWSResourceName(instanceIDSqlServer)
				ownerAccountID := ackv1alpha1.AWSAccountID("testOwnerId")
				region := ackv1alpha1.AWSRegion("us-east-1")
				dbInstanceSqlServer.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{
					ARN:            &arn,
					OwnerAccountID: &ownerAccountID,
					Region:         &region,
				}
				err := k8sClient.Status().Update(ctx, dbInstanceSqlServer)
				return err == nil
			}, timeout).Should(BeTrue())
		})

		Context("when Connection is created", func() {
			connectionName := "rds-connection-connection-controller"
			inventoryName := "rds-inventory-connection-controller"

			connection := &rdsdbaasv1alpha1.RDSConnection{
				ObjectMeta: metav1.ObjectMeta{
					Name:      connectionName,
					Namespace: testNamespace,
				},
				Spec: dbaasv1beta1.DBaaSConnectionSpec{
					InventoryRef: dbaasv1beta1.NamespacedName{
						Name:      inventoryName,
						Namespace: testNamespace,
					},
					InstanceID: instanceID,
				},
			}
			BeforeEach(assertResourceCreation(connection))
			AfterEach(assertResourceDeletion(connection))

			Context("when Inventory is not created", func() {
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
					Spec: dbaasv1beta1.DBaaSInventorySpec{
						CredentialsRef: &dbaasv1beta1.LocalObjectReference{
							Name: credentialName,
						},
					},
				}
				BeforeEach(assertResourceCreation(inventory))
				AfterEach(assertResourceDeletion(inventory))

				Context("when Inventory is not ready", func() {
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
					accessKey := "AKIAIOSFODNN7EXAMPLE" + test.ConnectionControllerTestAccessKeySuffix
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
						BeforeEach(func() {
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
								conn.Spec.InstanceID = "instance-id-connection-controller-not-exist"
								err := k8sClient.Update(ctx, conn)
								return err == nil
							}, timeout).Should(BeTrue())
						})

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
						Context("when Instance is not ready", func() {
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
								Eventually(func() bool {
									if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
										return false
									}
									dbInstance.Status.DBInstanceStatus = pointer.String("available")
									err := k8sClient.Status().Update(ctx, dbInstance)
									return err == nil
								}, timeout).Should(BeTrue())
							})

							Context("when the Instance user password is not set", func() {
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
										if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "InputError" {
											return false
										}
										return true
									}, timeout).Should(BeTrue())
								})
							})

							Context("when the Instance user password is set", func() {
								BeforeEach(func() {
									Eventually(func() bool {
										if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
											return false
										}
										dbInstance.Spec.MasterUserPassword = &ackv1alpha1.SecretKeyReference{
											SecretReference: v1.SecretReference{
												Name:      "secret-connection-controller",
												Namespace: testNamespace,
											},
											Key: "password",
										}
										err := k8sClient.Update(ctx, dbInstance)
										return err == nil
									}, timeout).Should(BeTrue())
								})

								Context("when the Instance user password Secret is not created", func() {
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

								Context("when the Instance user password Secret is created", func() {
									passwordSecret := &v1.Secret{
										ObjectMeta: metav1.ObjectMeta{
											Name:      "secret-connection-controller",
											Namespace: testNamespace,
										},
									}
									BeforeEach(assertResourceCreation(passwordSecret))
									AfterEach(assertResourceDeletion(passwordSecret))

									Context("when the Instance user password Secret is not valid", func() {
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
												if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "InputError" {
													return false
												}
												return true
											}, timeout).Should(BeTrue())
										})
									})

									Context("when the Instance user password Secret is valid", func() {
										BeforeEach(func() {
											Eventually(func() bool {
												if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(passwordSecret), passwordSecret); err != nil {
													return false
												}
												passwordSecret.Data = map[string][]byte{
													"password": []byte("testpassword"),
												}
												err := k8sClient.Update(ctx, passwordSecret)
												return err == nil
											}, timeout).Should(BeTrue())
										})

										Context("when the Instance user name is not set", func() {
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
													if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != "InputError" {
														return false
													}
													return true
												}, timeout).Should(BeTrue())
											})
										})

										Context("when the Instance user name is set", func() {
											BeforeEach(func() {
												Eventually(func() bool {
													if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
														return false
													}
													dbInstance.Spec.MasterUsername = pointer.String("user-connection-controller")
													err := k8sClient.Update(ctx, dbInstance)
													return err == nil
												}, timeout).Should(BeTrue())
											})

											Context("when the Instance endpoint is not available", func() {
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

											Context("when the Instance endpoint is available", func() {
												BeforeEach(func() {
													Eventually(func() bool {
														if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
															return false
														}
														dbInstance.Status.Endpoint = &rdsv1alpha1.Endpoint{
															Address: pointer.String("address-connection-controller"),
															Port:    pointer.Int64(9000),
														}
														err := k8sClient.Status().Update(ctx, dbInstance)
														return err == nil
													}, timeout).Should(BeTrue())
												})

												Context("when the Instance connection info is complete", func() {
													It("should create Secret and ConfigMap for binding", func() {
														By("checking the status of the Connection")
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
															if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != "Ready" {
																return false
															}
															Expect(conn.Status.CredentialsRef).ShouldNot(BeNil())
															Expect(conn.Status.CredentialsRef.Name).Should(Equal(fmt.Sprintf("%s-credentials", conn.Name)))
															Expect(conn.Status.ConnectionInfoRef).ShouldNot(BeNil())
															Expect(conn.Status.ConnectionInfoRef.Name).Should(Equal(fmt.Sprintf("%s-configs", conn.Name)))
															return true
														}, timeout).Should(BeTrue())

														By("checking the Secret of the Connection")
														secret := &v1.Secret{
															ObjectMeta: metav1.ObjectMeta{
																Name:      conn.Status.CredentialsRef.Name,
																Namespace: testNamespace,
															},
														}
														err := k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
														Expect(err).ShouldNot(HaveOccurred())
														secretOwner := metav1.GetControllerOf(secret)
														Expect(secretOwner).ShouldNot(BeNil())
														Expect(secretOwner.Kind).Should(Equal("RDSConnection"))
														Expect(secretOwner.Name).Should(Equal(conn.Name))
														Expect(secretOwner.Controller).ShouldNot(BeNil())
														Expect(*secretOwner.Controller).Should(BeTrue())
														Expect(secretOwner.BlockOwnerDeletion).ShouldNot(BeNil())
														Expect(*secretOwner.BlockOwnerDeletion).Should(BeTrue())
														user, userOk := secret.Data["username"]
														Expect(userOk).Should(BeTrue())
														Expect(string(user)).Should(Equal("user-connection-controller"))
														password, passwordOk := secret.Data["password"]
														Expect(passwordOk).Should(BeTrue())
														Expect(string(password)).Should(Equal("testpassword"))

														By("checking the ConfigMap of the Connection")
														configmap := &v1.ConfigMap{
															ObjectMeta: metav1.ObjectMeta{
																Name:      conn.Status.ConnectionInfoRef.Name,
																Namespace: testNamespace,
															},
														}
														err = k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), configmap)
														Expect(err).ShouldNot(HaveOccurred())
														configmapOwner := metav1.GetControllerOf(configmap)
														Expect(configmapOwner).ShouldNot(BeNil())
														Expect(configmapOwner.Kind).Should(Equal("RDSConnection"))
														Expect(configmapOwner.Name).Should(Equal(conn.Name))
														Expect(configmapOwner.Controller).ShouldNot(BeNil())
														Expect(*configmapOwner.Controller).Should(BeTrue())
														Expect(configmapOwner.BlockOwnerDeletion).ShouldNot(BeNil())
														Expect(*configmapOwner.BlockOwnerDeletion).Should(BeTrue())
														t, typeOk := configmap.Data["type"]
														Expect(typeOk).Should(BeTrue())
														Expect(t).Should(Equal("postgresql"))
														provider, providerOk := configmap.Data["provider"]
														Expect(providerOk).Should(BeTrue())
														Expect(provider).Should(Equal("Red Hat DBaaS / Amazon Relational Database Service (RDS)"))
														host, hostOk := configmap.Data["host"]
														Expect(hostOk).Should(BeTrue())
														Expect(host).Should(Equal("address-connection-controller"))
														port, portOk := configmap.Data["port"]
														Expect(portOk).Should(BeTrue())
														Expect(port).Should(Equal("9000"))
														db, dbOk := configmap.Data["database"]
														Expect(dbOk).Should(BeTrue())
														Expect(db).Should(Equal("postgres"))
													})
												})
											})
										})
									})
								})
							})
						})
					})
				})
			})
		})

		Context("when Inventory is created and ready", func() {
			credentialName := "credentials-ref-jdbc-url-connection-controller"
			accessKey := "AKIAIOSFODNN7EXAMPLE" + test.ConnectionControllerTestAccessKeySuffix
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

			inventoryName := "rds-inventory-jdbc-url-connection-controller"
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

			passwordSecret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-jdbc-url-connection-controller",
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					"password": []byte("testpassword"),
				},
			}
			BeforeEach(assertResourceCreation(passwordSecret))
			AfterEach(assertResourceDeletion(passwordSecret))

			Context("when Connection for Oracle is created", func() {
				BeforeEach(func() {
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstanceOracle), dbInstanceOracle); err != nil {
							return false
						}
						dbInstanceOracle.Status.DBInstanceStatus = pointer.String("available")
						dbInstanceOracle.Status.Endpoint = &rdsv1alpha1.Endpoint{
							Address: pointer.String("address-oracle-connection-controller"),
							Port:    pointer.Int64(9000),
						}
						err := k8sClient.Status().Update(ctx, dbInstanceOracle)
						return err == nil
					}, timeout).Should(BeTrue())
				})

				connectionName := "rds-connection-oracle-connection-controller"
				connection := &rdsdbaasv1alpha1.RDSConnection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      connectionName,
						Namespace: testNamespace,
					},
					Spec: dbaasv1beta1.DBaaSConnectionSpec{
						InventoryRef: dbaasv1beta1.NamespacedName{
							Name:      inventoryName,
							Namespace: testNamespace,
						},
						InstanceID: instanceIDOracle,
					},
				}
				BeforeEach(assertResourceCreation(connection))
				AfterEach(assertResourceDeletion(connection))

				It("should add jdbc-url to the ConfigMap for service binding", func() {
					By("checking the status of the Connection")
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
						if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != "Ready" {
							return false
						}
						Expect(conn.Status.CredentialsRef).ShouldNot(BeNil())
						Expect(conn.Status.CredentialsRef.Name).Should(Equal(fmt.Sprintf("%s-credentials", conn.Name)))
						Expect(conn.Status.ConnectionInfoRef).ShouldNot(BeNil())
						Expect(conn.Status.ConnectionInfoRef.Name).Should(Equal(fmt.Sprintf("%s-configs", conn.Name)))
						return true
					}, timeout).Should(BeTrue())

					By("checking the ConfigMap of the Connection")
					configmap := &v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      conn.Status.ConnectionInfoRef.Name,
							Namespace: testNamespace,
						},
					}
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), configmap)
					Expect(err).ShouldNot(HaveOccurred())
					ju, juOk := configmap.Data["jdbc-url"]
					Expect(juOk).Should(BeTrue())
					Expect(ju).Should(Equal("jdbc:oracle:thin:@address-oracle-connection-controller:9000/ORCL"))
					t, typeOk := configmap.Data["type"]
					Expect(typeOk).Should(BeTrue())
					Expect(t).Should(Equal("oracle"))
					provider, providerOk := configmap.Data["provider"]
					Expect(providerOk).Should(BeTrue())
					Expect(provider).Should(Equal("Red Hat DBaaS / Amazon Relational Database Service (RDS)"))
					host, hostOk := configmap.Data["host"]
					Expect(hostOk).Should(BeTrue())
					Expect(host).Should(Equal("address-oracle-connection-controller"))
					port, portOk := configmap.Data["port"]
					Expect(portOk).Should(BeTrue())
					Expect(port).Should(Equal("9000"))
					db, dbOk := configmap.Data["database"]
					Expect(dbOk).Should(BeTrue())
					Expect(db).Should(Equal("ORCL"))
				})
			})

			Context("when Connection for SqlServer is created", func() {
				BeforeEach(func() {
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstanceSqlServer), dbInstanceSqlServer); err != nil {
							return false
						}
						dbInstanceSqlServer.Status.DBInstanceStatus = pointer.String("available")
						dbInstanceSqlServer.Status.Endpoint = &rdsv1alpha1.Endpoint{
							Address: pointer.String("address-sqlserver-connection-controller"),
							Port:    pointer.Int64(9000),
						}
						err := k8sClient.Status().Update(ctx, dbInstanceSqlServer)
						return err == nil
					}, timeout).Should(BeTrue())
				})

				connectionName := "rds-connection-sqlserver-connection-controller"
				connection := &rdsdbaasv1alpha1.RDSConnection{
					ObjectMeta: metav1.ObjectMeta{
						Name:      connectionName,
						Namespace: testNamespace,
					},
					Spec: dbaasv1beta1.DBaaSConnectionSpec{
						InventoryRef: dbaasv1beta1.NamespacedName{
							Name:      inventoryName,
							Namespace: testNamespace,
						},
						InstanceID: instanceIDSqlServer,
					},
				}
				BeforeEach(assertResourceCreation(connection))
				AfterEach(assertResourceDeletion(connection))

				It("should add jdbc-url to the ConfigMap for service binding", func() {
					By("checking the status of the Connection")
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
						if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != "Ready" {
							return false
						}
						Expect(conn.Status.CredentialsRef).ShouldNot(BeNil())
						Expect(conn.Status.CredentialsRef.Name).Should(Equal(fmt.Sprintf("%s-credentials", conn.Name)))
						Expect(conn.Status.ConnectionInfoRef).ShouldNot(BeNil())
						Expect(conn.Status.ConnectionInfoRef.Name).Should(Equal(fmt.Sprintf("%s-configs", conn.Name)))
						return true
					}, timeout).Should(BeTrue())

					By("checking the ConfigMap of the Connection")
					configmap := &v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      conn.Status.ConnectionInfoRef.Name,
							Namespace: testNamespace,
						},
					}
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), configmap)
					Expect(err).ShouldNot(HaveOccurred())
					ju, juOk := configmap.Data["jdbc-url"]
					Expect(juOk).Should(BeTrue())
					Expect(ju).Should(Equal("jdbc:sqlserver://address-sqlserver-connection-controller:9000;databaseName=master"))
					t, typeOk := configmap.Data["type"]
					Expect(typeOk).Should(BeTrue())
					Expect(t).Should(Equal("sqlserver"))
					provider, providerOk := configmap.Data["provider"]
					Expect(providerOk).Should(BeTrue())
					Expect(provider).Should(Equal("Red Hat DBaaS / Amazon Relational Database Service (RDS)"))
					host, hostOk := configmap.Data["host"]
					Expect(hostOk).Should(BeTrue())
					Expect(host).Should(Equal("address-sqlserver-connection-controller"))
					port, portOk := configmap.Data["port"]
					Expect(portOk).Should(BeTrue())
					Expect(port).Should(Equal("9000"))
					db, dbOk := configmap.Data["database"]
					Expect(dbOk).Should(BeTrue())
					Expect(db).Should(Equal("master"))
				})
			})
		})
	})
})
