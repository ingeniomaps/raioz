package production

import (
	"fmt"
	"strings"

	"raioz/internal/i18n"
)

// FormatComparisonResult formats a comparison result as a readable string
func FormatComparisonResult(result *ComparisonResult) string {
	var sb strings.Builder

	if len(result.Errors) > 0 {
		sb.WriteString("\n❌ " + i18n.T("compare.errors") + "\n")
		for _, err := range result.Errors {
			fmt.Fprintf(&sb, "  • %s\n", err)
		}
	}

	if len(result.ServiceDifferences) > 0 {
		sb.WriteString("\n📊 " + i18n.T("compare.service_differences") + "\n")
		for _, diff := range result.ServiceDifferences {
			fmt.Fprintf(&sb, "\n  %s\n", i18n.T("compare.service", diff.ServiceName))
			writePresence(&sb, diff.InLocalOnly, diff.InProductionOnly)

			if diff.ImageMismatch != nil {
				writeMismatch(&sb, "    "+i18n.T("compare.image_mismatch"),
					diff.ImageMismatch.Local, diff.ImageMismatch.Production)
				if diff.ImageMismatch.LocalTag != diff.ImageMismatch.ProdTag {
					fmt.Fprintf(&sb, "      %s\n", i18n.T("compare.tag_mismatch",
						diff.ImageMismatch.LocalTag, diff.ImageMismatch.ProdTag))
				}
			}
			if diff.PortMismatch != nil {
				writeMismatch(&sb, "    "+i18n.T("compare.port_mismatch"),
					diff.PortMismatch.Local, diff.PortMismatch.Production)
			}
			if diff.DependsMismatch != nil {
				writeMismatch(&sb, "    ⚠️  "+i18n.T("compare.depends_mismatch"),
					diff.DependsMismatch.Local, diff.DependsMismatch.Production)
			}
			if diff.VolumeMismatch != nil {
				writeMismatch(&sb, "    "+i18n.T("compare.volumes_mismatch"),
					diff.VolumeMismatch.Local, diff.VolumeMismatch.Production)
			}
		}
	}

	if len(result.InfraDifferences) > 0 {
		sb.WriteString("\n🏗️  " + i18n.T("compare.infra_differences") + "\n")
		for _, diff := range result.InfraDifferences {
			fmt.Fprintf(&sb, "\n  %s\n", i18n.T("compare.infra", diff.InfraName))
			writePresence(&sb, diff.InLocalOnly, diff.InProductionOnly)

			if diff.ImageMismatch != nil {
				writeMismatch(&sb, "    "+i18n.T("compare.image_mismatch"),
					diff.ImageMismatch.Local, diff.ImageMismatch.Production)
			}
			if diff.PortMismatch != nil {
				writeMismatch(&sb, "    "+i18n.T("compare.port_mismatch"),
					diff.PortMismatch.Local, diff.PortMismatch.Production)
			}
		}
	}

	if len(result.Warnings) > 0 {
		sb.WriteString("\n⚠️  " + i18n.T("compare.warnings") + "\n")
		for _, warning := range result.Warnings {
			fmt.Fprintf(&sb, "  • %s\n", warning)
		}
	}

	if len(result.ServiceDifferences) == 0 && len(result.InfraDifferences) == 0 &&
		len(result.Errors) == 0 && len(result.Warnings) == 0 {
		sb.WriteString("\n✅ " + i18n.T("compare.no_differences") + "\n")
	}

	return sb.String()
}

// writePresence notes an entry that exists on one side only.
func writePresence(sb *strings.Builder, localOnly, productionOnly bool) {
	if localOnly {
		sb.WriteString("    ⚠️  " + i18n.T("compare.only_local") + "\n")
	}
	if productionOnly {
		sb.WriteString("    ℹ️  " + i18n.T("compare.only_production") + "\n")
	}
}

// writeMismatch prints a heading followed by the local and production
// values, aligned.
func writeMismatch(sb *strings.Builder, heading string, local, production any) {
	fmt.Fprintf(sb, "%s\n", heading)
	fmt.Fprintf(sb, "      %-12s%v\n", i18n.T("compare.local_label"), local)
	fmt.Fprintf(sb, "      %-12s%v\n", i18n.T("compare.production_label"), production)
}
