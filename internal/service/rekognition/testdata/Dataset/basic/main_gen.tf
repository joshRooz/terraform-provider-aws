# Copyright IBM Corp. 2014, 2026
# SPDX-License-Identifier: MPL-2.0

resource "aws_rekognition_dataset" "test" {

  project_arn = aws_rekognition_project.test.arn
  dataset_type = "TRAIN"
}

resource "aws_rekognition_project" "test" {

  name = var.rName
  feature = "CUSTOM_LABELS"
}
variable "rName" {
  description = "Name for resource"
  type        = string
  nullable    = false
}
