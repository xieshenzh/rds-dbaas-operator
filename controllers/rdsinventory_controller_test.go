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
	rdsv1alpha1 "github.com/aws-controllers-k8s/rds-controller/apis/v1alpha1"
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ophandler "github.com/operator-framework/operator-lib/handler"
)

var _ = Describe("RDSInventoryController", func() {
	Context("when Secret for launching RDS controller is created", func() {
		credentialName := "credentials-ref-inventory-controller"
		inventoryName := "rds-inventory-inventory-controller"

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

		Context("when Inventory is created", func() {
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

			Context("when checking the status of the Inventory", func() {
				AfterEach(assertResourceDeletion(inventory))

				dbInstance1 := &rdsv1alpha1.DBInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db-instance-inventory-controller-1",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBInstanceSpec{
						Engine:               pointer.String("postgres"),
						DBInstanceIdentifier: pointer.String("dbInstance1"),
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
						err := k8sClient.Status().Update(ctx, dbInstance1)
						return err == nil
					}, timeout).Should(BeTrue())
				})

				dbInstance2 := &rdsv1alpha1.DBInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db-instance-inventory-controller-2",
						Namespace: testNamespace,
					},
					Spec: rdsv1alpha1.DBInstanceSpec{
						Engine:               pointer.String("mysql"),
						DBInstanceIdentifier: pointer.String("dbInstance2"),
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
						err := k8sClient.Status().Update(ctx, dbInstance2)
						return err == nil
					}, timeout).Should(BeTrue())
				})

				dbInstance3 := &rdsv1alpha1.DBInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "db-instance-inventory-controller-3",
						Namespace: testNamespace,
						Labels: map[string]string{
							"rds.dbaas.redhat.com/adopted": "true",
						},
					},
					Spec: rdsv1alpha1.DBInstanceSpec{
						Engine:               pointer.String("postgres"),
						DBInstanceIdentifier: pointer.String("dbInstance3"),
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
						err := k8sClient.Status().Update(ctx, dbInstance3)
						return err == nil
					}, timeout).Should(BeTrue())
				})

				It("should start the RDS controller, adopt the DB instances and sync DB instance status", func() {
					By("checking if the Secret for RDS controller is created")
					rdsSecret := &v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "ack-user-secrets",
							Namespace: testNamespace,
						},
					}
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsSecret), rdsSecret); err != nil {
							return false
						}
						ak, akOk := rdsSecret.Data["AWS_ACCESS_KEY_ID"]
						Expect(akOk).Should(BeTrue())
						Expect(ak).Should(Equal([]byte(accessKey)))
						sk, skOk := rdsSecret.Data["AWS_SECRET_ACCESS_KEY"]
						Expect(skOk).Should(BeTrue())
						Expect(sk).Should(Equal([]byte(secretKey)))
						return true
					}, timeout).Should(BeTrue())

					By("checking if the ConfigMap for RDS controller is created")
					rdsConfigMap := &v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "ack-user-config",
							Namespace: testNamespace,
						},
					}
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsConfigMap), rdsConfigMap); err != nil {
							return false
						}
						r, ok := rdsConfigMap.Data["AWS_REGION"]
						Expect(ok).Should(BeTrue())
						Expect(r).Should(Equal(region))
						return true
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
						if len(inv.Status.Instances) != 3 {
							return false
						}
						instancesMap := make(map[string]dbaasv1alpha1.Instance, 3)
						for i := range inv.Status.Instances {
							ins := inv.Status.Instances[i]
							instancesMap[ins.InstanceID] = ins
						}
						if ins, ok := instancesMap[*dbInstance1.Spec.DBInstanceIdentifier]; !ok {
							return false
						} else {
							Expect(ins.Name).Should(Equal(dbInstance1.Name))
							s, ok := ins.InstanceInfo["dbInstanceStatus"]
							Expect(ok).Should(BeTrue())
							Expect(s).Should(Equal(*dbInstance1.Status.DBInstanceStatus))
						}
						if ins, ok := instancesMap[*dbInstance2.Spec.DBInstanceIdentifier]; !ok {
							return false
						} else {
							Expect(ins.Name).Should(Equal(dbInstance2.Name))
							s, ok := ins.InstanceInfo["dbInstanceStatus"]
							Expect(ok).Should(BeTrue())
							Expect(s).Should(Equal(*dbInstance2.Status.DBInstanceStatus))
						}
						if ins, ok := instancesMap[*dbInstance3.Spec.DBInstanceIdentifier]; !ok {
							return false
						} else {
							Expect(ins.Name).Should(Equal(dbInstance3.Name))
							s, ok := ins.InstanceInfo["dbInstanceStatus"]
							Expect(ok).Should(BeTrue())
							Expect(s).Should(Equal(*dbInstance3.Status.DBInstanceStatus))
						}
						return true
					}, timeout).Should(BeTrue())

					By("checking if DB instances are adopted")
					Eventually(func() bool {
						adoptedDBInstances := &ackv1alpha1.AdoptedResourceList{}
						if err := k8sClient.List(ctx, adoptedDBInstances, client.InNamespace(testNamespace)); err != nil {
							return false
						}
						if len(adoptedDBInstances.Items) != 5 {
							return false
						}
						dbInstancesMap := make(map[string]ackv1alpha1.AdoptedResource, 5)
						for i := range adoptedDBInstances.Items {
							instance := adoptedDBInstances.Items[i]
							dbInstancesMap[instance.Spec.AWS.NameOrID] = instance
						}
						if instance, ok := dbInstancesMap["mock-db-instance-1"]; !ok {
							return false
						} else {
							if !strings.HasPrefix(instance.Name, "mock-db-instance-1") {
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
						}
						if instance, ok := dbInstancesMap["mock-db-instance-2"]; !ok {
							return false
						} else {
							if !strings.HasPrefix(instance.Name, "mock-db-instance-2") {
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
						}
						if instance, ok := dbInstancesMap["mock-db-instance-3"]; !ok {
							return false
						} else {
							if !strings.HasPrefix(instance.Name, "mock-db-instance-3") {
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
						}
						if instance, ok := dbInstancesMap["mock-db-instance-4"]; !ok {
							return false
						} else {
							if !strings.HasPrefix(instance.Name, "mock-db-instance-4") {
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
						}
						if instance, ok := dbInstancesMap["mock-db-instance-5"]; !ok {
							return false
						} else {
							if !strings.HasPrefix(instance.Name, "mock-db-instance-5") {
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
						}
						return true
					}, timeout).Should(BeTrue())

					By("checking if the password of adopted db instance is reset")
					dbInstance := &rdsv1alpha1.DBInstance{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "db-instance-inventory-controller-3",
							Namespace: testNamespace,
						},
					}
					dbSecret := &v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "db-instance-inventory-controller-3-credentials",
							Namespace: testNamespace,
						},
					}
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbInstance), dbInstance); err != nil {
							return false
						}
						if dbInstance.Spec.MasterUserPassword == nil {
							return false
						}
						Expect(dbInstance.Spec.MasterUserPassword.Key).Should(Equal("password"))
						Expect(dbInstance.Spec.MasterUserPassword.Namespace).Should(Equal(testNamespace))
						Expect(dbInstance.Spec.MasterUserPassword.Name).Should(Equal("db-instance-inventory-controller-3-credentials"))
						err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dbSecret), dbSecret)
						Expect(err).ShouldNot(HaveOccurred())
						v, ok := dbSecret.Data["password"]
						Expect(ok).Should(BeTrue())
						Expect(len(v)).Should(BeNumerically(">", 0))
						return true
					}, timeout).Should(BeTrue())
				})
			})

			Context("when the Inventory is deleted", func() {
				It("should delete the owned resources and stop the RDS controller", func() {
					assertResourceDeletion(inventory)()

					By("checking if the Secret for RDS controller is deleted")
					rdsSecret := &v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "ack-user-secrets",
							Namespace: testNamespace,
						},
					}
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsSecret), rdsSecret); err != nil {
							if errors.IsNotFound(err) {
								return true
							}
						}
						return false
					}, timeout).Should(BeTrue())

					By("checking if the ConfigMap for RDS controller is deleted")
					rdsConfigMap := &v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "ack-user-config",
							Namespace: testNamespace,
						},
					}
					Eventually(func() bool {
						if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(rdsConfigMap), rdsConfigMap); err != nil {
							if errors.IsNotFound(err) {
								return true
							}
						}
						return false
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
	})
})
