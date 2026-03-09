---
subcategory: "Rekognition"
layout: "aws"
page_title: "AWS: aws_rekognition_dataset"
description: |-
  Manages an AWS Rekognition Dataset.
---

# Resource: aws_rekognition_dataset

Manages an AWS Rekognition Dataset.

## Example Usage

### Basic Usage

```terraform
resource "aws_rekognition_project" "example" {
  name    = "example-project"
  feature = "CUSTOM_LABELS"
}

resource "aws_rekognition_dataset" "example" {
  project_arn  = aws_rekognition_project.example.arn
  dataset_type = "TRAIN"

  dataset_source {
    ground_truth_manifest {
      s3_object {
        bucket = "example"
        name   = "datasets/example/manifests/output/output.manifest"
      }
    }
  }
}
```

## Argument Reference

The following arguments are required:

* `project_arn` - (Required) ARN of the Amazon Rekognition Custom Labels project to assign the dataset to.
* `dataset_type` - (Required) Type of the dataset.

The following arguments are optional:

* `dataset_source` - (Optional) Source of the dataset. Specify either an existing dataset ARN or an S3 location with a Sagemaker Ground Truth manifest file. Omit to create an empty dataset. See [`dataset_source`](#dataset_source).
* `tags` - (Optional) Map of tags assigned to the resource. If configured with a provider [`default_tags` configuration block](/docs/providers/aws/index.html#default_tags-configuration-block) present, tags with matching keys will overwrite those defined at the provider-level.


### `dataset_source`

* `dataset_arn` - (Optional) ARN of an existing Amazon Rekognition Custom Labels dataset to copy. Conflicts with `ground_truth_manifest`.
* `ground_truth_manifest` - (Optional) Sagemaker Ground Truth manifest file location. Conflicts with `dataset_arn`. See [`ground_truth_manifest`](#ground_truth_manifest).

### `ground_truth_manifest`

* `s3_object` - (Optional) S3 location of the manifest file. See [`s3_object`](#s3_object).

### `s3_object`

* `bucket` - (Required) S3 bucket name.
* `name` - (Required) S3 object key.
* `version` - (Optional) S3 object version ID.

## Attribute Reference

This resource exports the following attributes in addition to the arguments above:

* `arn` - Dataset ARN.
* `tags_all` - Map of tags assigned to the resource, including those inherited from the provider [`default_tags` configuration block](/docs/providers/aws/index.html#default_tags-configuration-block).

## Timeouts

[Configuration options](https://developer.hashicorp.com/terraform/language/resources/syntax#operation-timeouts):

* `create` - (Default `30m`)
* `delete` - (Default `30m`)

## Import

~> This resource does not support import.
