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

package rds

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type DescribeDBClustersPaginatorAPI interface {
	HasMorePages() bool
	NextPage(context.Context, ...func(option *rds.Options)) (*rds.DescribeDBClustersOutput, error)
}

type sdkV2DescribeDBClustersPaginator struct {
	paginator *rds.DescribeDBClustersPaginator
}

func NewDescribeDBClustersPaginator(accessKey, secretKey, region string) DescribeDBClustersPaginatorAPI {
	awsClient := rds.New(rds.Options{
		Region:      region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	})
	paginator := rds.NewDescribeDBClustersPaginator(awsClient, nil)
	return &sdkV2DescribeDBClustersPaginator{
		paginator: paginator,
	}
}

func (p *sdkV2DescribeDBClustersPaginator) HasMorePages() bool {
	return p.paginator.HasMorePages()
}

func (p *sdkV2DescribeDBClustersPaginator) NextPage(ctx context.Context, f ...func(option *rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return p.paginator.NextPage(ctx, f...)
}

type ModifyDBClusterAPI interface {
	ModifyDBCluster(ctx context.Context, params *rds.ModifyDBClusterInput, optFns ...func(*rds.Options)) (*rds.ModifyDBClusterOutput, error)
}

type sdkV2ModifyDBCluster struct {
	client *rds.Client
}

func NewModifyDBCluster(accessKey, secretKey, region string) ModifyDBClusterAPI {
	awsClient := rds.New(rds.Options{
		Region:      region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	})
	return &sdkV2ModifyDBCluster{
		client: awsClient,
	}
}

func (m *sdkV2ModifyDBCluster) ModifyDBCluster(ctx context.Context, params *rds.ModifyDBClusterInput, optFns ...func(*rds.Options)) (*rds.ModifyDBClusterOutput, error) {
	return m.client.ModifyDBCluster(ctx, params, optFns...)
}

type DescribeDBClustersAPI interface {
	DescribeDBClusters(context.Context, *rds.DescribeDBClustersInput, ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error)
}

type sdkV2DescribeDBClusters struct {
	client *rds.Client
}

func NewDescribeDBClusters(accessKey, secretKey, region string) DescribeDBClustersAPI {
	awsClient := rds.New(rds.Options{
		Region:      region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	})
	return &sdkV2DescribeDBClusters{
		client: awsClient,
	}
}

func (d *sdkV2DescribeDBClusters) DescribeDBClusters(ctx context.Context, params *rds.DescribeDBClustersInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return d.client.DescribeDBClusters(ctx, params, optFns...)
}
