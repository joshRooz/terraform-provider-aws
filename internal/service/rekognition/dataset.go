// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

// DONOTCOPY: Copying old resources spreads bad habits. Use skaff instead.

package rekognition

import (
	"context"
	"errors"
	"time"

	"github.com/YakDriver/smarterr"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	awstypes "github.com/aws/aws-sdk-go-v2/service/rekognition/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/fwdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	"github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/smerr"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_rekognition_dataset", name="Dataset")
// @ArnIdentity
// @Tags(identifierAttribute="arn")
// @Testing(existsType="github.com/aws/aws-sdk-go-v2/service/rekognition;rekognition.DescribeDatasetOutput")
// @Testing(hasNoPreExistingResource=true)
// @Testing(tagsTest=false)
// @NoImport
func newDatasetResource(_ context.Context) (resource.ResourceWithConfigure, error) {
	r := &datasetResource{}

	r.SetDefaultCreateTimeout(30 * time.Minute)
	r.SetDefaultUpdateTimeout(30 * time.Minute)
	r.SetDefaultDeleteTimeout(30 * time.Minute)

	return r, nil
}

const (
	ResNameDataset = "Dataset"
)

type datasetResource struct {
	framework.ResourceWithModel[datasetResourceModel]
	framework.WithTimeouts
	framework.WithNoUpdate
}

func (r *datasetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			names.AttrARN: framework.ARNAttributeComputedOnly(),
			"dataset_type": schema.StringAttribute{
				Description: "The type of the dataset.",
				CustomType:  fwtypes.StringEnumType[awstypes.DatasetType](),
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project_arn": schema.StringAttribute{
				Description: "The ARN of the Amazon Rekognition Custom Labels project to which you want to asssign the dataset.",
				CustomType:  fwtypes.ARNType,
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			names.AttrTags:    tftags.TagsAttribute(),
			names.AttrTagsAll: tftags.TagsAttributeComputedOnly(),
		},
		Blocks: map[string]schema.Block{
			"dataset_source": schema.ListNestedBlock{
				Description: "The source files for the dataset. You can specify the ARN of an existing dataset or specify the Amazon S3 bucket location of an Amazon Sagemaker format manifest file. If you don't specify datasetSource , an empty dataset is created.",
				CustomType:  fwtypes.NewListNestedObjectTypeOf[datasetSourceModel](ctx),
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"dataset_arn": schema.StringAttribute{
							Description: "The ARN of an Amazon Rekognition Custom Labels dataset that you want to copy.",
							CustomType:  fwtypes.ARNType,
							Optional:    true,
							Validators: []validator.String{
								stringvalidator.ConflictsWith(
									path.MatchRelative().AtParent().AtName("ground_truth_manifest"),
								),
							},
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
					},
					Blocks: map[string]schema.Block{
						"ground_truth_manifest": schema.ListNestedBlock{
							Description: "The Amazon S3 bucket that contains an AWS Sagemaker Ground Truth format manifest file.",
							CustomType:  fwtypes.NewListNestedObjectTypeOf[groundTruthManifestModel](ctx),
							NestedObject: schema.NestedBlockObject{
								Blocks: map[string]schema.Block{
									"s3_object": schema.ListNestedBlock{
										Description: "The Amazon S3 bucket that contains an AWS Sagemaker Ground Truth format manifest file.",
										CustomType:  fwtypes.NewListNestedObjectTypeOf[s3ObjectModel](ctx),
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"bucket": schema.StringAttribute{
													Description: "Name of the S3 bucket.",
													Required:    true,
												},
												"name": schema.StringAttribute{
													Description: "S3 object key name.",
													Required:    true,
												},
												"version": schema.StringAttribute{
													Description: "If versioning is enabled, you can specify the object version.",
													Optional:    true,
												},
											},
										},
										Validators: []validator.List{
											listvalidator.SizeAtMost(1),
										},
										PlanModifiers: []planmodifier.List{
											listplanmodifier.RequiresReplace(),
										},
									},
								},
							},
							Validators: []validator.List{
								listvalidator.SizeAtMost(1),
							},
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
			},
			names.AttrTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Delete: true,
			}),
		},
	}
}

func (r *datasetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	conn := r.Meta().RekognitionClient(ctx)

	var plan datasetResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.Plan.Get(ctx, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	input := rekognition.CreateDatasetInput{
		Tags: getTagsIn(ctx),
	}
	smerr.AddEnrich(ctx, &resp.Diagnostics, flex.Expand(ctx, plan, &input, flex.WithFieldNamePrefix("Dataset")))
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := conn.CreateDataset(ctx, &input)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, plan.ProjectARN.String(), plan.DatasetType.String())
		return
	}
	if out == nil || out.DatasetArn == nil {
		smerr.AddError(ctx, &resp.Diagnostics, errors.New("empty output"), plan.ProjectARN.String(), plan.DatasetType.String())
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, flex.Flatten(ctx, out, &plan))
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ARN = fwflex.StringToFramework(ctx, out.DatasetArn)

	createTimeout := r.CreateTimeout(ctx, plan.Timeouts)
	_, err = waitDatasetCreated(ctx, conn, plan.ARN.ValueString(), createTimeout)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, plan.ARN.String())
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, plan))
}

func (r *datasetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	conn := r.Meta().RekognitionClient(ctx)

	var state datasetResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := findDatasetByARN(ctx, conn, state.ARN.ValueString())
	if retry.NotFound(err) {
		resp.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, state.ARN.String())
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, flex.Flatten(ctx, out, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	smerr.AddEnrich(ctx, &resp.Diagnostics, resp.State.Set(ctx, &state))
}

func (r *datasetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	conn := r.Meta().RekognitionClient(ctx)

	var state datasetResourceModel
	smerr.AddEnrich(ctx, &resp.Diagnostics, req.State.Get(ctx, &state))
	if resp.Diagnostics.HasError() {
		return
	}

	input := rekognition.DeleteDatasetInput{
		DatasetArn: state.ARN.ValueStringPointer(),
	}

	_, err := conn.DeleteDataset(ctx, &input)
	if err != nil {
		if errs.IsA[*awstypes.ResourceNotFoundException](err) {
			return
		}

		smerr.AddError(ctx, &resp.Diagnostics, err, state.ARN.String())
		return
	}

	deleteTimeout := r.DeleteTimeout(ctx, state.Timeouts)
	_, err = waitDatasetDeleted(ctx, conn, state.ARN.ValueString(), deleteTimeout)
	if err != nil {
		smerr.AddError(ctx, &resp.Diagnostics, err, state.ARN.String())
		return
	}
}

func waitDatasetCreated(ctx context.Context, conn *rekognition.Client, arn string, timeout time.Duration) (*awstypes.DatasetDescription, error) {
	stateConf := &retry.StateChangeConf{
		Pending:                   enum.Slice(awstypes.DatasetStatusCreateInProgress),
		Target:                    enum.Slice(awstypes.DatasetStatusCreateComplete),
		Refresh:                   statusDataset(conn, arn),
		Timeout:                   timeout,
		NotFoundChecks:            20,
		ContinuousTargetOccurence: 2,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)
	if out, ok := outputRaw.(*awstypes.DatasetDescription); ok {
		return out, smarterr.NewError(err)
	}

	return nil, smarterr.NewError(err)
}

func waitDatasetDeleted(ctx context.Context, conn *rekognition.Client, arn string, timeout time.Duration) (*awstypes.DatasetDescription, error) {
	stateConf := &retry.StateChangeConf{
		Pending: enum.Slice(awstypes.DatasetStatusDeleteInProgress),
		Target:  []string{},
		Refresh: statusDataset(conn, arn),
		Timeout: timeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)
	if out, ok := outputRaw.(*awstypes.DatasetDescription); ok {
		return out, smarterr.NewError(err)
	}

	return nil, smarterr.NewError(err)
}

func statusDataset(conn *rekognition.Client, arn string) retry.StateRefreshFunc {
	return func(ctx context.Context) (any, string, error) {
		out, err := findDatasetByARN(ctx, conn, arn)
		if retry.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", smarterr.NewError(err)
		}

		return out, string(out.DatasetDescription.Status), nil
	}
}

func findDatasetByARN(ctx context.Context, conn *rekognition.Client, arn string) (*rekognition.DescribeDatasetOutput, error) {
	input := rekognition.DescribeDatasetInput{
		DatasetArn: aws.String(arn),
	}

	out, err := conn.DescribeDataset(ctx, &input)
	if err != nil {
		if errs.IsA[*awstypes.ResourceNotFoundException](err) {
			return nil, smarterr.NewError(&retry.NotFoundError{
				LastError: err,
			})
		}

		return nil, smarterr.NewError(err)
	}

	if out == nil || out.DatasetDescription == nil {
		return nil, smarterr.NewError(tfresource.NewEmptyResultError())
	}

	return out, nil
}

type datasetResourceModel struct {
	framework.WithRegionModel
	ARN           types.String                                        `tfsdk:"arn"`
	DatasetSource fwtypes.ListNestedObjectValueOf[datasetSourceModel] `tfsdk:"dataset_source"`
	DatasetType   fwtypes.StringEnum[awstypes.DatasetType]            `tfsdk:"dataset_type"`
	ProjectARN    fwtypes.ARN                                         `tfsdk:"project_arn"`
	Tags          tftags.Map                                          `tfsdk:"tags"`
	TagsAll       tftags.Map                                          `tfsdk:"tags_all"`
	Timeouts      timeouts.Value                                      `tfsdk:"timeouts"`
}

type datasetSourceModel struct {
	DatasetARN          fwtypes.ARN                                               `tfsdk:"dataset_arn"`
	GroundTruthManifest fwtypes.ListNestedObjectValueOf[groundTruthManifestModel] `tfsdk:"ground_truth_manifest"`
}

type groundTruthManifestModel struct {
	S3Object fwtypes.ListNestedObjectValueOf[s3ObjectModel] `tfsdk:"s3_object"`
}

type s3ObjectModel struct {
	Bucket  types.String `tfsdk:"bucket"`
	Name    types.String `tfsdk:"name"`
	Version types.String `tfsdk:"version"`
}
