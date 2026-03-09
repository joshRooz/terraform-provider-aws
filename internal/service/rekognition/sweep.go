// Copyright IBM Corp. 2014, 2026

package rekognition

import (
	"context"

	"github.com/YakDriver/smarterr"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep"
	"github.com/hashicorp/terraform-provider-aws/internal/sweep/awsv2"
	sweepfw "github.com/hashicorp/terraform-provider-aws/internal/sweep/framework"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func RegisterSweepers() {
	awsv2.Register("aws_rekognition_dataset", sweepDatasets)
}

func sweepDatasets(ctx context.Context, client *conns.AWSClient) ([]sweep.Sweepable, error) {
	input := rekognition.DescribeProjectsInput{}
	conn := client.RekognitionClient(ctx)
	var sweepResources []sweep.Sweepable

	pages := rekognition.NewDescribeProjectsPaginator(conn, &input)
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, smarterr.NewError(err)
		}

		for _, prj := range page.ProjectDescriptions {
			for _, ds := range prj.Datasets {
				sweepResources = append(sweepResources, sweepfw.NewSweepResource(newDatasetResource, client,
					sweepfw.NewAttribute(names.AttrARN, aws.ToString(ds.DatasetArn))),
				)
			}
		}
	}

	return sweepResources, nil
}
