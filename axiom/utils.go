package axiom

import (
	"context"
	"errors"
	"fmt"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func typeStringSliceToStringSlice(ctx context.Context, s []attr.Value) ([]string, diag.Diagnostics) {
	result := make([]string, 0, len(s))
	var diags diag.Diagnostics
	for _, v := range s {
		val, err := v.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		var str string
		if err = val.As(&str); err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		result = append(result, str)
	}
	if diags.HasError() {
		return nil, diags
	}
	return result, nil
}

func frameworkDatasourceSchemaFromFrameworkResourceSchema(rs resourceschema.Schema) datasourceschema.Schema {
	attributes := convertAttributes(rs.Attributes)
	if idSchema, ok := rs.Attributes["id"]; ok {
		attributes["id"] = datasourceschema.StringAttribute{
			Required:            true,
			Description:         idSchema.GetDescription(),
			MarkdownDescription: idSchema.GetMarkdownDescription(),
		}
	}

	return datasourceschema.Schema{
		Attributes:          attributes,
		Description:         rs.Description,
		MarkdownDescription: rs.MarkdownDescription,
		DeprecationMessage:  rs.DeprecationMessage,
	}
}

func convertAttributes(attributes map[string]resourceschema.Attribute) map[string]datasourceschema.Attribute {
	result := make(map[string]datasourceschema.Attribute, len(attributes))
	for k, v := range attributes {
		result[k] = convertAttribute(v)
	}
	return result
}

func convertAttribute(resourceAttribute resourceschema.Attribute) datasourceschema.Attribute {
	switch attr := resourceAttribute.(type) {
	case resourceschema.BoolAttribute:
		return datasourceschema.BoolAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}
	case resourceschema.Float64Attribute:
		return datasourceschema.Float64Attribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}
	case resourceschema.Int64Attribute:
		return datasourceschema.Int64Attribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}
	case resourceschema.NumberAttribute:
		return datasourceschema.NumberAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}
	case resourceschema.StringAttribute:
		return datasourceschema.StringAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}
	case resourceschema.MapAttribute:
		return datasourceschema.MapAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			ElementType:         attr.ElementType,
		}
	case resourceschema.ObjectAttribute:
		return datasourceschema.ObjectAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			AttributeTypes:      attr.AttributeTypes,
		}
	case resourceschema.SetAttribute:
		return datasourceschema.SetAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			ElementType:         attr.ElementType,
		}
	case resourceschema.ListNestedAttribute:
		return datasourceschema.ListNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			NestedObject: datasourceschema.NestedAttributeObject{
				Attributes: convertAttributes(attr.NestedObject.Attributes),
			},
		}
	case resourceschema.ListAttribute:
		return datasourceschema.ListAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			ElementType:         attr.ElementType,
		}
	case resourceschema.MapNestedAttribute:
		return datasourceschema.MapNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			NestedObject: datasourceschema.NestedAttributeObject{
				Attributes: convertAttributes(attr.NestedObject.Attributes),
			},
		}
	case resourceschema.SetNestedAttribute:
		return datasourceschema.SetNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			NestedObject: datasourceschema.NestedAttributeObject{
				Attributes: convertAttributes(attr.NestedObject.Attributes),
			},
		}
	case resourceschema.SingleNestedAttribute:
		return datasourceschema.SingleNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			Attributes:          convertAttributes(attr.Attributes),
		}
	default:
		panic(fmt.Sprintf("unknown resource attribute type: %T", resourceAttribute))
	}
}

func isNotFoundError(err error) bool {
	var apiError axiom.HTTPError
	if errors.As(err, &apiError) {
		return apiError.Status == 404
	}
	return false
}
