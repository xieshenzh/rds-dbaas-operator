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

package test

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/utils/pointer"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"

	controllersrds "github.com/RHEcosystemAppEng/rds-dbaas-operator/controllers/rds"
)

var clusterInventoryTestDBInstances = []*rds.DescribeDBClustersOutput{
	{
		DBClusters: []types.DBCluster{
			//	// Adopted successful
			//	{
			//		DBInstanceIdentifier: pointer.String("mock-db-instance-1"),
			//		DBInstanceStatus:     pointer.String("available"),
			//		Engine:               pointer.String("postgres"),
			//		DBInstanceArn:        pointer.String("mock-db-instance-1"),
			//	},
			//	// Adopted successful
			//	{
			//		DBInstanceIdentifier: pointer.String("mock-db-instance-2"),
			//		DBInstanceStatus:     pointer.String("available"),
			//		Engine:               pointer.String("postgres"),
			//		DBInstanceArn:        pointer.String("mock-db-instance-2"),
			//	},
			//	// Adopted successful
			//	{
			//		DBInstanceIdentifier: pointer.String("mock-db-instance-3"),
			//		DBInstanceStatus:     pointer.String("available"),
			//		Engine:               pointer.String("postgres"),
			//		DBInstanceArn:        pointer.String("mock-db-instance-3"),
			//	},
		},
	},
	{
		DBClusters: []types.DBCluster{
			//// Adopted successful
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-instance-4"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("postgres"),
			//	DBInstanceArn:        pointer.String("mock-db-instance-4"),
			//},
			//// Deleting, Not Adopted
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-instance-5"),
			//	DBInstanceStatus:     pointer.String("deleting"),
			//	Engine:               pointer.String("mysql"),
			//	DBInstanceArn:        pointer.String("mock-db-instance-5"),
			//},
			//// Adopted successfully
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-instance-7"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("mysql"),
			//	DBInstanceArn:        pointer.String("mock-db-instance-7"),
			//},
			//// Already adopted, not adopted
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-instance-8"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("mysql"),
			//	DBInstanceArn:        pointer.String("mock-db-instance-8"),
			//},
			//// DB instance exists, not adopted
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-instance-9"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("mysql"),
			//	DBInstanceArn:        pointer.String("mock-db-instance-9"),
			//},
		},
	},
	{
		DBClusters: []types.DBCluster{
			//// Reset credentials successfully
			//{
			//	DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-3"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("mariadb"),
			//	DBName:               pointer.String("test-dbname"),
			//	MasterUsername:       pointer.String("test-username"),
			//	DBInstanceArn:        pointer.String("mock-adopted-db-instance-3"),
			//},
			//// Deleting, Not reset credentials
			//{
			//	DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-4"),
			//	DBInstanceStatus:     pointer.String("deleting"),
			//	Engine:               pointer.String("postgres"),
			//	DBName:               pointer.String("test-dbname"),
			//	MasterUsername:       pointer.String("test-username"),
			//	DBInstanceArn:        pointer.String("mock-adopted-db-instance-4"),
			//},
			//// Not available, Not reset credentials
			//{
			//	DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-5"),
			//	DBInstanceStatus:     pointer.String("creating"),
			//	Engine:               pointer.String("mysql"),
			//	DBName:               pointer.String("test-dbname"),
			//	MasterUsername:       pointer.String("test-username"),
			//	DBInstanceArn:        pointer.String("mock-adopted-db-instance-5"),
			//},
			//// ARN not match, Not reset credentials
			//{
			//	DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-15"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("mysql"),
			//	DBName:               pointer.String("test-dbname"),
			//	MasterUsername:       pointer.String("test-username"),
			//	DBInstanceArn:        pointer.String("mock-adopted-db-instance-15-0"),
			//},
		},
	},
	{
		// Cluster not supported
		DBClusters: []types.DBCluster{
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-cluster-1"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("mysql"),
			//	DBInstanceArn:        pointer.String("mock-db-cluster-1"),
			//},
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-cluster-2"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("postgres"),
			//	DBInstanceArn:        pointer.String("mock-db-cluster-2"),
			//},
		},
	},
	{
		// Engine not supported
		DBClusters: []types.DBCluster{
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-aurora-1"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("aurora"),
			//	DBInstanceArn:        pointer.String("mock-db-aurora-1"),
			//},
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-aurora-2"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("aurora-mysql"),
			//	DBInstanceArn:        pointer.String("mock-db-aurora-2"),
			//},
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-aurora-3"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("aurora-postgresql"),
			//	DBInstanceArn:        pointer.String("mock-db-aurora-3"),
			//},
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-custom-1"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("custom-oracle-ee"),
			//	DBInstanceArn:        pointer.String("mock-db-custom-1"),
			//},
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-custom-2"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("custom-sqlserver-ee"),
			//	DBInstanceArn:        pointer.String("mock-db-custom-2"),
			//},
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-custom-3"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("custom-sqlserver-se"),
			//	DBInstanceArn:        pointer.String("mock-db-custom-3"),
			//},
			//{
			//	DBInstanceIdentifier: pointer.String("mock-db-custom-4"),
			//	DBInstanceStatus:     pointer.String("available"),
			//	Engine:               pointer.String("custom-sqlserver-web"),
			//	DBInstanceArn:        pointer.String("mock-db-custom-4"),
			//},
		},
	},
}

var clusterConnectionTestDBInstances = []*rds.DescribeDBClustersOutput{
	{
		DBClusters: []types.DBCluster{
			{
				DBClusterIdentifier: pointer.String("cluster-id-connection-controller"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("postgres"),
				DBClusterArn:        pointer.String("cluster-id-connection-controller"),
			},
		},
	},
}

var (
	ClusterInventoryControllerTestAccessKeySuffix  = "INVENTORYCONTROLLER_CLUSTER"
	ClusterConnectionControllerTestAccessKeySuffix = "CONNECTIONCONTROLLER_CLUSTER"
)

type mockDescribeDBClustersPaginator struct {
	accessKey, secretKey, region string
	counter                      int
}

func NewDescribeDBClustersPaginator(accessKey, secretKey, region string) controllersrds.DescribeDBClustersPaginatorAPI {
	counter := 0
	if strings.HasSuffix(accessKey, ClusterInventoryControllerTestAccessKeySuffix) {
		counter = 5
	} else if strings.HasSuffix(accessKey, ClusterConnectionControllerTestAccessKeySuffix) {
		counter = 1
	}
	return &mockDescribeDBClustersPaginator{accessKey: accessKey, secretKey: secretKey, region: region, counter: counter}
}

func (m *mockDescribeDBClustersPaginator) HasMorePages() bool {
	return m.counter > 0
}

func (m *mockDescribeDBClustersPaginator) NextPage(ctx context.Context, f ...func(option *rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	if m.counter > 0 {
		m.counter--
		if strings.HasSuffix(m.accessKey, ClusterInventoryControllerTestAccessKeySuffix) {
			return clusterInventoryTestDBInstances[m.counter], nil
		} else if strings.HasSuffix(m.accessKey, ClusterConnectionControllerTestAccessKeySuffix) {
			return clusterConnectionTestDBInstances[m.counter], nil
		}
	}
	return nil, nil
}

type mockModifyDBCluster struct {
	accessKey, secretKey, region string
}

func NewModifyDBCluster(accessKey, secretKey, region string) controllersrds.ModifyDBClusterAPI {
	return &mockModifyDBCluster{accessKey: accessKey, secretKey: secretKey, region: region}
}

func (m *mockModifyDBCluster) ModifyDBCluster(ctx context.Context, params *rds.ModifyDBClusterInput, optFns ...func(*rds.Options)) (*rds.ModifyDBClusterOutput, error) {
	return nil, nil
}

type mockDescribeDBClusters struct {
	accessKey, secretKey, region string
}

func NewDescribeDBClusters(accessKey, secretKey, region string) controllersrds.DescribeDBClustersAPI {
	return &mockDescribeDBClusters{accessKey: accessKey, secretKey: secretKey, region: region}
}

func (d *mockDescribeDBClusters) DescribeDBClusters(ctx context.Context, params *rds.DescribeDBClustersInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	if strings.HasSuffix(d.accessKey, "INVALID") {
		return nil, fmt.Errorf("invalid accesskey")
	}
	return nil, nil
}
