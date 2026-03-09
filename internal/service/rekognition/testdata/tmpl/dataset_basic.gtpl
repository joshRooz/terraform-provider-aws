resource "aws_rekognition_dataset" "test" {
{{- template "region" }}

  project_arn = aws_rekognition_project.test.arn
  dataset_type = "TRAIN"

{{- template "tags" . }}
}

resource "aws_rekognition_project" "test" {
{{- template "region" }}

  name = var.rName
  feature = "CUSTOM_LABELS"
}