# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "aws_rekognition_dataset" "test" {
  region = var.region


  project_arn = aws_rekognition_project.test.arn
  dataset_type = "TRAIN"
}

resource "aws_rekognition_project" "test" {
  region = var.region


  name = var.rName
  feature = "CUSTOM_LABELS"
}
variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}

variable "region" {
  description = "Region to deploy resource in"
  type        = string
  nullable    = false
}
