// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package rekognition_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/YakDriver/regexache"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	sdkacctest "github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	tfrekognition "github.com/hashicorp/terraform-provider-aws/internal/service/rekognition"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccRekognitionDataset_basic(t *testing.T) {
	ctx := acctest.Context(t)

	var dataset rekognition.DescribeDatasetOutput
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_rekognition_dataset.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			acctest.PreCheckPartitionHasService(t, names.RekognitionEndpointID)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.RekognitionServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDatasetExists(ctx, t, resourceName, &dataset),
					resource.TestCheckResourceAttrSet(resourceName, "dataset_type"),
					acctest.MatchResourceAttrRegionalARN(ctx, resourceName, "project_arn", "rekognition", regexache.MustCompile(fmt.Sprintf("project/%s.+$", rName))),
					acctest.MatchResourceAttrRegionalARN(ctx, resourceName, names.AttrARN, "rekognition", regexache.MustCompile(fmt.Sprintf("project/%s/dataset.+$", rName))),
				),
			},
		},
	})
}

func TestAccRekognitionDataset_disappears(t *testing.T) {
	ctx := acctest.Context(t)

	var dataset rekognition.DescribeDatasetOutput
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_rekognition_dataset.test"

	acctest.ParallelTest(ctx, t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			acctest.PreCheckPartitionHasService(t, names.RekognitionEndpointID)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.RekognitionServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckDatasetExists(ctx, t, resourceName, &dataset),
					acctest.CheckFrameworkResourceDisappears(ctx, t, tfrekognition.ResourceDataset, resourceName),
				),
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}

func TestAccRekognitionDataset_DatasetType(t *testing.T) {
	ctx := acctest.Context(t)

	var dataset1, dataset2 rekognition.DescribeDatasetOutput
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_rekognition_dataset.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.RekognitionServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetConfig_DatasetType(rName, "TEST"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatasetExists(ctx, t, resourceName, &dataset1),
					resource.TestCheckResourceAttr(resourceName, "dataset_type", "TEST"),
				),
			},
			{
				Config: testAccDatasetConfig_DatasetType(rName, "TRAIN"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatasetExists(ctx, t, resourceName, &dataset2),
					testAccCheckDatasetRecreated(&dataset1, &dataset2),
					resource.TestCheckResourceAttr(resourceName, "dataset_type", "TRAIN"),
				),
			},
		},
	})
}

func TestAccRekognitionDataset_ProjectARN(t *testing.T) {
	ctx := acctest.Context(t)

	var dataset1, dataset2 rekognition.DescribeDatasetOutput
	rName1 := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	rName2 := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_rekognition_dataset.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.RekognitionServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetConfig_ProjectARN(rName1, rName2, rName1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatasetExists(ctx, t, resourceName, &dataset1),
					acctest.MatchResourceAttrRegionalARN(ctx, resourceName, "project_arn", "rekognition", regexache.MustCompile(fmt.Sprintf("project/%s.+$", rName1))),
				),
			},
			{
				Config: testAccDatasetConfig_ProjectARN(rName1, rName2, rName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatasetExists(ctx, t, resourceName, &dataset2),
					testAccCheckDatasetRecreated(&dataset1, &dataset2),
					acctest.MatchResourceAttrRegionalARN(ctx, resourceName, "project_arn", "rekognition", regexache.MustCompile(fmt.Sprintf("project/%s.+$", rName2))),
				),
			},
		},
	})
}

func TestAccRekognitionDataset_DatasetSource_GroundTruthManifest_S3Object(t *testing.T) {
	ctx := acctest.Context(t)

	var dataset1, dataset2 rekognition.DescribeDatasetOutput
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_rekognition_dataset.test"
	s3BucketResourceName := "aws_s3_bucket.test"
	s3ObjectResourceName := "aws_s3_object.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.RekognitionServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetConfig_DatasetSource_GroundTruthManifest_S3Object(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatasetExists(ctx, t, resourceName, &dataset1),
					resource.TestCheckResourceAttrPair(resourceName, "dataset_source.0.ground_truth_manifest.0.s3_object.0.bucket", s3ObjectResourceName, names.AttrBucket),
					resource.TestCheckResourceAttrPair(resourceName, "dataset_source.0.ground_truth_manifest.0.s3_object.0.name", s3ObjectResourceName, names.AttrKey),
					resource.TestCheckResourceAttrPair(resourceName, "dataset_source.0.ground_truth_manifest.0.s3_object.0.version", s3ObjectResourceName, "version_id"),
				),
			},
			{
				Config: testAccDatasetConfig_DatasetSource_GroundTruthManifest_S3Object(rName),
				Taint:  []string{s3BucketResourceName},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatasetExists(ctx, t, resourceName, &dataset2),
					resource.TestCheckResourceAttrPair(resourceName, "dataset_source.0.ground_truth_manifest.0.s3_object.0.bucket", s3ObjectResourceName, names.AttrBucket),
					resource.TestCheckResourceAttrPair(resourceName, "dataset_source.0.ground_truth_manifest.0.s3_object.0.name", s3ObjectResourceName, names.AttrKey),
					resource.TestCheckResourceAttrPair(resourceName, "dataset_source.0.ground_truth_manifest.0.s3_object.0.version", s3ObjectResourceName, "version_id"),
					testAccCheckDatasetRecreated(&dataset1, &dataset2),
				),
			},
		},
	})
}

func TestAccRekognitionDataset_DatasetSource_DatasetARN(t *testing.T) {
	ctx := acctest.Context(t)

	var dataset1, dataset2 rekognition.DescribeDatasetOutput
	rSrcName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	rTgtName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_rekognition_dataset.test_target"
	resourceSourceName := "aws_rekognition_dataset.test_source"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.RekognitionServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroy(ctx, t),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetConfig_DatasetSource_DatasetARN(rSrcName, rTgtName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatasetExists(ctx, t, resourceName, &dataset1),
					acctest.MatchResourceAttrRegionalARN(ctx, resourceName, "dataset_source.0.dataset_arn", "rekognition", regexache.MustCompile(fmt.Sprintf("project/%s/dataset/.+$", rSrcName))),
				),
			},
			{
				Config: testAccDatasetConfig_DatasetSource_DatasetARN(rSrcName, rTgtName),
				Taint:  []string{resourceSourceName},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatasetExists(ctx, t, resourceName, &dataset2),
					acctest.MatchResourceAttrRegionalARN(ctx, resourceName, "dataset_source.0.dataset_arn", "rekognition", regexache.MustCompile(fmt.Sprintf("project/%s/dataset/.+$", rSrcName))),
					testAccCheckDatasetRecreated(&dataset1, &dataset2),
				),
			},
		},
	})
}

func testAccCheckDatasetDestroy(ctx context.Context, t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.ProviderMeta(ctx, t).RekognitionClient(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aws_rekognition_dataset" {
				continue
			}

			arn := rs.Primary.Attributes[names.AttrARN]
			_, err := tfrekognition.FindDatasetByARN(ctx, conn, arn)
			if retry.NotFound(err) {
				return nil
			}
			if err != nil {
				return create.Error(names.Rekognition, create.ErrActionCheckingDestroyed, tfrekognition.ResNameDataset, arn, err)
			}

			return create.Error(names.Rekognition, create.ErrActionCheckingDestroyed, tfrekognition.ResNameDataset, arn, errors.New("not destroyed"))
		}

		return nil
	}
}

func testAccCheckDatasetExists(ctx context.Context, t *testing.T, name string, dataset *rekognition.DescribeDatasetOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.Rekognition, create.ErrActionCheckingExistence, tfrekognition.ResNameDataset, name, errors.New("not found"))
		}

		arn := rs.Primary.Attributes[names.AttrARN]
		if arn == "" {
			return create.Error(names.Rekognition, create.ErrActionCheckingExistence, tfrekognition.ResNameDataset, name, errors.New("not set"))
		}

		conn := acctest.ProviderMeta(ctx, t).RekognitionClient(ctx)

		resp, err := tfrekognition.FindDatasetByARN(ctx, conn, arn)
		if err != nil {
			return create.Error(names.Rekognition, create.ErrActionCheckingExistence, tfrekognition.ResNameDataset, arn, err)
		}

		*dataset = *resp

		return nil
	}
}

func testAccCheckDatasetNotRecreated(before, after *rekognition.DescribeDatasetOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if before, after := aws.ToTime(before.DatasetDescription.CreationTimestamp), aws.ToTime(after.DatasetDescription.CreationTimestamp); !before.Equal(after) {
			return create.Error(names.Rekognition, create.ErrActionCheckingNotRecreated, tfrekognition.ResNameDataset, "", errors.New("recreated"))
		}

		return nil
	}
}

func testAccCheckDatasetRecreated(before, after *rekognition.DescribeDatasetOutput) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if before, after := aws.ToTime(before.DatasetDescription.CreationTimestamp), aws.ToTime(after.DatasetDescription.CreationTimestamp); before.Equal(after) {
			return create.Error(names.Rekognition, create.ErrActionCheckingRecreated, tfrekognition.ResNameDataset, "", errors.New("not recreated"))
		}

		return nil
	}
}

func testAccDatasetConfig_basic(rName string) string {
	return fmt.Sprintf(`
resource "aws_rekognition_project" "test" {
  name    = %[1]q
  feature = "CUSTOM_LABELS"
}

resource "aws_rekognition_dataset" "test" {
  dataset_type = "TEST"
  project_arn  = aws_rekognition_project.test.arn
}
`, rName)
}

func testAccDatasetConfig_DatasetType(rName string, dsType string) string {
	return fmt.Sprintf(`
resource "aws_rekognition_project" "test" {
  name    = %[1]q
  feature = "CUSTOM_LABELS"
}

resource "aws_rekognition_dataset" "test" {
  project_arn  = aws_rekognition_project.test.arn
  dataset_type = %[2]q
}
`, rName, dsType)
}

func testAccDatasetConfig_ProjectARN(rName1, rName2, tfRName string) string {
	return fmt.Sprintf(`
resource "aws_rekognition_project" %[1]q {
  name    = %[1]q
  feature = "CUSTOM_LABELS"
}

resource "aws_rekognition_project" %[2]q {
  name    = %[2]q
  feature = "CUSTOM_LABELS"
}

resource "aws_rekognition_dataset" "test" {
  project_arn  = aws_rekognition_project.%[3]s.arn
  dataset_type = "TEST"
}
`, rName1, rName2, tfRName)
}

func testAccDatasetConfig_DatasetSource_GroundTruthManifest_S3Object(rName string) string {
	return fmt.Sprintf(`
resource "aws_rekognition_project" "test" {
  name    = %[1]q
  feature = "CUSTOM_LABELS"
}

resource "aws_rekognition_dataset" "test" {
  project_arn  = aws_rekognition_project.test.arn
  dataset_type = "TEST"

  dataset_source {
    ground_truth_manifest {
      s3_object {
        bucket  = aws_s3_object.test.bucket
        name    = aws_s3_object.test.key
        version = aws_s3_object.test.version_id
      }
    }
  }
}

resource "aws_s3_bucket" "test" {
  force_destroy = true
}

resource "aws_s3_bucket_versioning" "test" {
  bucket = aws_s3_bucket.test.bucket
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_object" "test" {
  bucket = aws_s3_bucket.test.bucket
  key    = "manifest1.json"
  content = jsonencode({
    source-ref  = "s3://${aws_s3_bucket.test.bucket}/assets/helloworld.jpg"
    testdataset = 1
    testdataset-metadata = {
      confidence      = 1
      class-name      = "testacc"
      human-annotated = "yes"
      creation-date   = "2021-07-11T03:32:13.456Z"
      type            = "groundtruth/image-classification"
    }
  })
}
`, rName)
}

func testAccDatasetConfig_DatasetSource_DatasetARN(rSrcName string, rTgtName string) string {
	return acctest.ConfigCompose(
		fmt.Sprintf(`
resource "aws_rekognition_project" "test_source" {
	name    = %[1]q
	feature = "CUSTOM_LABELS"
}

resource "aws_rekognition_dataset" "test_source" {
  project_arn = aws_rekognition_project.test_source.arn
  dataset_type = "TEST"
}

resource "aws_rekognition_project" "test_target" {
	name    = %[2]q
	feature = "CUSTOM_LABELS"
}

resource "aws_rekognition_dataset" "test_target" {
  project_arn = aws_rekognition_project.test_target.arn
  dataset_type = "TEST"

	dataset_source {
		dataset_arn = aws_rekognition_dataset.test_source.arn
	}
}
`, rSrcName, rTgtName))
}
