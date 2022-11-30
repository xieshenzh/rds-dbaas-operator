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

var inventoryTestDBClusters = []*rds.DescribeDBClustersOutput{
	{
		DBClusters: []types.DBCluster{
			// Adopted successful
			{
				DBClusterIdentifier: pointer.String("mock-db-cluster-1"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("postgres"),
				DBClusterArn:        pointer.String("mock-db-cluster-1"),
			},
			// Adopted successful
			{
				DBClusterIdentifier: pointer.String("mock-db-cluster-2"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("mysql"),
				DBClusterArn:        pointer.String("mock-db-cluster-2"),
			},
			// Adopted successful
			{
				DBClusterIdentifier: pointer.String("mock-db-cluster-3"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("aurora"),
				DBClusterArn:        pointer.String("mock-db-cluster-3"),
			},
		},
	},
	{
		DBClusters: []types.DBCluster{
			// Adopted successful
			{
				DBClusterIdentifier: pointer.String("mock-db-cluster-4"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("aurora-postgresql"),
				DBClusterArn:        pointer.String("mock-db-cluster-4"),
			},
			// Deleting, Not Adopted
			{
				DBClusterIdentifier: pointer.String("mock-db-cluster-5"),
				Status:              pointer.String("deleting"),
				Engine:              pointer.String("postgres"),
				DBClusterArn:        pointer.String("mock-db-cluster-5"),
			},
			// DB cluster exists, not adopted
			{
				DBClusterIdentifier: pointer.String("mock-db-cluster-6"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("aurora"),
				DBClusterArn:        pointer.String("mock-db-cluster-6"),
			},
			// Adopted successfully
			{
				DBClusterIdentifier: pointer.String("mock-db-cluster-7"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("aurora-mysql"),
				DBClusterArn:        pointer.String("mock-db-cluster-7"),
			},
			// Already adopted, not adopted
			{
				DBClusterIdentifier: pointer.String("mock-db-cluster-8"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("mysql"),
				DBClusterArn:        pointer.String("mock-db-cluster-8"),
			},
		},
	},
	{
		DBClusters: []types.DBCluster{
			// Reset credentials successfully
			{
				DBClusterIdentifier: pointer.String("mock-adopted-db-cluster-3"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("aurora"),
				DBClusterArn:        pointer.String("mock-adopted-db-cluster-3"),
				DatabaseName:        pointer.String("test-dbname"),
				MasterUsername:      pointer.String("test-username"),
			},
			// Deleting, Not reset credentials
			{
				DBClusterIdentifier: pointer.String("mock-adopted-db-cluster-4"),
				Status:              pointer.String("deleting"),
				Engine:              pointer.String("aurora"),
				DBClusterArn:        pointer.String("mock-adopted-db-cluster-4"),
				DatabaseName:        pointer.String("test-dbname"),
				MasterUsername:      pointer.String("test-username"),
			},
			// Not available, Not reset credentials
			{
				DBClusterIdentifier: pointer.String("mock-adopted-db-cluster-5"),
				Status:              pointer.String("creating"),
				Engine:              pointer.String("aurora"),
				DBClusterArn:        pointer.String("mock-adopted-db-cluster-5"),
				DatabaseName:        pointer.String("test-dbname"),
				MasterUsername:      pointer.String("test-username"),
			},
			// ARN not match, Not reset credentials
			{
				DBClusterIdentifier: pointer.String("mock-adopted-db-cluster-20"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("aurora"),
				DBClusterArn:        pointer.String("mock-adopted-db-cluster-20-0"),
				DatabaseName:        pointer.String("test-dbname"),
				MasterUsername:      pointer.String("test-username"),
			},
		},
	},
	{
		// Engine not supported
		DBClusters: []types.DBCluster{
			{
				DBClusterIdentifier: pointer.String("mock-db-mariadb-1"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("mariadb"),
				DBClusterArn:        pointer.String("mock-db-mariadb-1"),
			},
			{
				DBClusterIdentifier: pointer.String("mock-db-oracle-1"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("oracle-se2"),
				DBClusterArn:        pointer.String("mock-db-oracle-1"),
			},
			{
				DBClusterIdentifier: pointer.String("mock-db-oracle-2"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("oracle-se2-cdb"),
				DBClusterArn:        pointer.String("mock-db-oracle-2"),
			},
			{
				DBClusterIdentifier: pointer.String("mock-db-oracle-3"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("oracle-ee"),
				DBClusterArn:        pointer.String("mock-db-oracle-3"),
			},
			{
				DBClusterIdentifier: pointer.String("mock-db-oracle-4"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("oracle-ee-cdb"),
				DBClusterArn:        pointer.String("mock-db-oracle-4"),
			},
			{
				DBClusterIdentifier: pointer.String("mock-db-oracle-5"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("custom-oracle-ee"),
				DBClusterArn:        pointer.String("mock-db-oracle-5"),
			},
		},
	},
	{
		// Engine not supported
		DBClusters: []types.DBCluster{
			{
				DBClusterIdentifier: pointer.String("mock-db-sqlserver-1"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("sqlserver-ee"),
				DBClusterArn:        pointer.String("mock-db-sqlserver-1"),
			},
			{
				DBClusterIdentifier: pointer.String("mock-db-sqlserver-2"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("sqlserver-se"),
				DBClusterArn:        pointer.String("mock-db-sqlserver-2"),
			},
			{
				DBClusterIdentifier: pointer.String("mock-db-sqlserver-3"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("sqlserver-ex"),
				DBClusterArn:        pointer.String("mock-db-sqlserver-3"),
			},
			{
				DBClusterIdentifier: pointer.String("mock-db-sqlserver-4"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("sqlserver-web"),
				DBClusterArn:        pointer.String("mock-db-sqlserver-4"),
			},
			{
				DBClusterIdentifier: pointer.String("mock-db-sqlserver-5"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("custom-sqlserver-ee"),
				DBClusterArn:        pointer.String("mock-db-sqlserver-5"),
			},
			{
				DBClusterIdentifier: pointer.String("mock-db-sqlserver-6"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("custom-sqlserver-se"),
				DBClusterArn:        pointer.String("mock-db-sqlserver-6"),
			},
			{
				DBClusterIdentifier: pointer.String("mock-db-sqlserver-7"),
				Status:              pointer.String("available"),
				Engine:              pointer.String("custom-sqlserver-web"),
				DBClusterArn:        pointer.String("mock-db-sqlserver-7"),
			},
		},
	},
}

var connectionTestDBClusters = []*rds.DescribeDBClustersOutput{
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
	ClusterInventoryControllerTestAccessKeySuffix  = "INVENTORYCONTROLLER"
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
			return inventoryTestDBClusters[m.counter], nil
		} else if strings.HasSuffix(m.accessKey, ClusterConnectionControllerTestAccessKeySuffix) {
			return connectionTestDBClusters[m.counter], nil
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
