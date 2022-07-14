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

	controllersrds "github.com/RHEcosystemAppEng/rds-dbaas-operator/controllers/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

var dbInstances = []*rds.DescribeDBInstancesOutput{
	{
		DBInstances: []types.DBInstance{
			{
				DBInstanceIdentifier: pointer.String("mock-db-instance-1"),
				DBInstanceStatus:     pointer.String("available"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-db-instance-2"),
				DBInstanceStatus:     pointer.String("available"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-db-instance-3"),
				DBInstanceStatus:     pointer.String("available"),
			},
		},
	},
	{
		DBInstances: []types.DBInstance{
			{
				DBInstanceIdentifier: pointer.String("mock-db-instance-4"),
				DBInstanceStatus:     pointer.String("available"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-db-instance-5"),
				DBInstanceStatus:     pointer.String("deleting"),
			},
		},
	},
	{
		DBInstances: []types.DBInstance{
			{
				DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-3"),
				DBInstanceStatus:     pointer.String("available"),
				DBName:               pointer.String("test-dbname"),
				MasterUsername:       pointer.String("test-username"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-4"),
				DBInstanceStatus:     pointer.String("deleting"),
				DBName:               pointer.String("test-dbname"),
				MasterUsername:       pointer.String("test-username"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-adopted-db-instance-5"),
				DBInstanceStatus:     pointer.String("creating"),
				DBName:               pointer.String("test-dbname"),
				MasterUsername:       pointer.String("test-username"),
			},
		},
	},
	{
		DBInstances: []types.DBInstance{
			{
				DBInstanceIdentifier: pointer.String("mock-db-cluster-1"),
				DBInstanceStatus:     pointer.String("available"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-db-cluster-2"),
				DBInstanceStatus:     pointer.String("available"),
			},
		},
	},
	{
		DBInstances: []types.DBInstance{
			{
				DBInstanceIdentifier: pointer.String("mock-db-aurora-1"),
				DBInstanceStatus:     pointer.String("available"),
				Engine:               pointer.String("aurora"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-db-aurora-2"),
				DBInstanceStatus:     pointer.String("available"),
				Engine:               pointer.String("aurora-mysql"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-db-aurora-3"),
				DBInstanceStatus:     pointer.String("available"),
				Engine:               pointer.String("aurora-postgresql"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-db-custom-1"),
				DBInstanceStatus:     pointer.String("available"),
				Engine:               pointer.String("custom-oracle-ee"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-db-custom-2"),
				DBInstanceStatus:     pointer.String("available"),
				Engine:               pointer.String("custom-sqlserver-ee"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-db-custom-3"),
				DBInstanceStatus:     pointer.String("available"),
				Engine:               pointer.String("custom-sqlserver-se"),
			},
			{
				DBInstanceIdentifier: pointer.String("mock-db-custom-4"),
				DBInstanceStatus:     pointer.String("available"),
				Engine:               pointer.String("custom-sqlserver-web"),
			},
		},
	},
}

type mockDescribeDBInstancesPaginator struct {
	accessKey, secretKey, region string
	counter                      int
}

func NewMockDescribeDBInstancesPaginator(accessKey, secretKey, region string) controllersrds.DescribeDBInstancesPaginatorAPI {
	counter := 0
	if strings.HasSuffix(accessKey, "INVENTORYCONTROLLER") {
		counter = 3
	}
	return &mockDescribeDBInstancesPaginator{accessKey: accessKey, secretKey: secretKey, region: region, counter: counter}
}

func (m *mockDescribeDBInstancesPaginator) HasMorePages() bool {
	return m.counter > 0
}

func (m *mockDescribeDBInstancesPaginator) NextPage(ctx context.Context, f ...func(option *rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	if m.counter > 0 {
		m.counter--
		return dbInstances[m.counter], nil
	}
	return nil, nil
}

type mockModifyDBInstance struct {
	accessKey, secretKey, region string
}

func NewModifyDBInstance(accessKey, secretKey, region string) controllersrds.ModifyDBInstanceAPI {
	return &mockModifyDBInstance{accessKey: accessKey, secretKey: secretKey, region: region}
}

func (m *mockModifyDBInstance) ModifyDBInstance(ctx context.Context, params *rds.ModifyDBInstanceInput, optFns ...func(*rds.Options)) (*rds.ModifyDBInstanceOutput, error) {
	return nil, nil
}

type mockDescribeDBInstances struct {
	accessKey, secretKey, region string
}

func NewDescribeDBInstances(accessKey, secretKey, region string) controllersrds.DescribeDBInstancesAPI {
	return &mockDescribeDBInstances{accessKey: accessKey, secretKey: secretKey, region: region}
}

func (d *mockDescribeDBInstances) DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	if strings.HasSuffix(d.accessKey, "INVALID") {
		return nil, fmt.Errorf("invalid accesskey")
	}
	return nil, nil
}
