package aws

import (
	"fmt"
	"infracost/pkg/schema"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

func AwsEcsService(d *schema.ResourceData) *schema.Resource {
	launchType := d.Get("launch_type").String()
	if launchType != "FARGATE" {
		return nil
	}

	region := d.Get("region").String()
	desiredCount := d.Get("desired_count").Int()
	taskDefinition := d.References("task_definition")[0]
	memory := convertResourceString(taskDefinition.Get("memory").String())
	cpu := convertResourceString(taskDefinition.Get("cpu").String())

	costComponents := []*schema.CostComponent{
		{
			Name:           "Per GB per hour",
			Unit:           "GB-hours",
			HourlyQuantity: decimalPtr(decimal.NewFromInt(desiredCount).Mul(memory)),
			ProductFilter: &schema.ProductFilter{
				VendorName:    strPtr("aws"),
				Region:        strPtr(region),
				Service:       strPtr("AmazonECS"),
				ProductFamily: strPtr("Compute"),
				AttributeFilters: &[]schema.AttributeFilter{
					{Key: "usagetype", ValueRegex: strPtr("/Fargate-GB-Hours/")},
				},
			},
		},
		{
			Name:           "Per vCPU per hour",
			Unit:           "CPU-hours",
			HourlyQuantity: decimalPtr(decimal.NewFromInt(desiredCount).Mul(cpu)),
			ProductFilter: &schema.ProductFilter{
				VendorName:    strPtr("aws"),
				Region:        strPtr(region),
				Service:       strPtr("AmazonECS"),
				ProductFamily: strPtr("Compute"),
				AttributeFilters: &[]schema.AttributeFilter{
					{Key: "usagetype", ValueRegex: strPtr("/Fargate-vCPU-Hours:perCPU/")},
				},
			},
		},
	}

	if taskDefinition.Get("inference_accelerator.0").Exists() {
		deviceType := taskDefinition.Get("inference_accelerator.0.device_type").String()
		costComponents = append(costComponents, &schema.CostComponent{
			Name:           fmt.Sprintf("Inference accelerator (%s)", deviceType),
			Unit:           "hours",
			HourlyQuantity: decimalPtr(decimal.NewFromInt(desiredCount)),
			ProductFilter: &schema.ProductFilter{
				VendorName:    strPtr("aws"),
				Region:        strPtr(region),
				Service:       strPtr("AmazonEI"),
				ProductFamily: strPtr("Elastic Inference"),
				AttributeFilters: &[]schema.AttributeFilter{
					{Key: "usagetype", ValueRegex: strPtr(fmt.Sprintf("/%s/", deviceType))},
				},
			},
		})
	}

	return &schema.Resource{
		Name:           d.Address,
		CostComponents: costComponents,
	}
}

func convertResourceString(rawValue string) decimal.Decimal {
	var quantity decimal.Decimal
	noSpaceString := strings.ReplaceAll(rawValue, " ", "")
	reg := regexp.MustCompile(`(?i)vcpu|gb`)
	if reg.MatchString(noSpaceString) {
		quantity, _ = decimal.NewFromString(reg.ReplaceAllString(noSpaceString, ""))
	} else {
		quantity, _ = decimal.NewFromString(noSpaceString)
		quantity = quantity.Div(decimal.NewFromInt(1024))
	}
	return quantity
}
