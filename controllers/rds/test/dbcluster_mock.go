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

	controllersrds "github.com/RHEcosystemAppEng/rds-dbaas-operator/controllers/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type mockDescribeDBClustersPaginator struct {
	accessKey, secretKey, region string
	counter                      int
}

func NewDescribeDBClustersPaginator(accessKey, secretKey, region string) controllersrds.DescribeDBClustersPaginatorAPI {
	return &mockDescribeDBClustersPaginator{accessKey: accessKey, secretKey: secretKey, region: region, counter: 0}
}

func (m *mockDescribeDBClustersPaginator) HasMorePages() bool {
	return false
}

func (m *mockDescribeDBClustersPaginator) NextPage(ctx context.Context, f ...func(option *rds.Options)) (*rds.DescribeDBClustersOutput, error) {
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
	return nil, nil
}
