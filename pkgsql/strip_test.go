package pkgsql

import (
	"testing"
)

func TestStripFieldTables(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "three_column_field_table",
			in: `Some prose before.

**Exported fields**

| Field | Description | Type |
|---|---|---|
| @timestamp | Event timestamp. | date |
| message | Log message. | text |

Some prose after.`,
			want: `Some prose before.

Some prose after.`,
		},
		{
			name: "four_column_field_table_with_unit",
			in: `Before.

**Exported fields**

| Field | Description | Type | Unit |
|---|---|---|---|
| system.cpu.total.pct | Total CPU usage. | scaled_float | percent |

After.`,
			want: `Before.

After.`,
		},
		{
			name: "four_column_field_table_with_metric_type",
			in: `Before.

*Exported fields*

| Field | Description | Type | Metric Type |
|---|---|---|---|
| system.memory.total | Total memory. | long | gauge |

After.`,
			want: `Before.

After.`,
		},
		{
			name: "example_event_json_block",
			in: `Some docs.

An example event for ` + "`access`" + ` looks as following:

` + "```json" + `
{
    "@timestamp": "2022-12-09T10:39:23.000Z",
    "message": "test"
}
` + "```" + `

More docs.`,
			want: `Some docs.

More docs.`,
		},
		{
			name: "non_field_table_preserved",
			in: `Before.

| Job | Description |
|---|---|
| high_count | Detects anomalous access rates. |
| low_count | Detects anomalous drops. |

After.`,
			want: `Before.

| Job | Description |
|---|---|
| high_count | Detects anomalous access rates. |
| low_count | Detects anomalous drops. |

After.`,
		},
		{
			name: "prose_surrounding_stripped_sections",
			in: `# Setup

Configure the integration.

**Exported fields**

| Field | Description | Type |
|---|---|---|
| event.kind | Event kind. | keyword |

## Troubleshooting

Check your TLS configuration.`,
			want: `# Setup

Configure the integration.

## Troubleshooting

Check your TLS configuration.`,
		},
		{
			name: "content_with_no_tables_or_events",
			in: `# Simple Doc

This document has no field tables or example events.

Just plain markdown content.`,
			want: `# Simple Doc

This document has no field tables or example events.

Just plain markdown content.`,
		},
		{
			name: "field_table_without_exported_fields_header",
			in: `Before.

| Field | Description | Type |
|---|---|---|
| foo | A field. | keyword |

After.`,
			want: `Before.

After.`,
		},
		{
			name: "multiple_field_tables_and_events",
			in: `# Access logs

**Exported fields**

| Field | Description | Type |
|---|---|---|
| access.url | Request URL. | keyword |

An example event for ` + "`access`" + ` looks as following:

` + "```json" + `
{"url": "/test"}
` + "```" + `

# Error logs

**Exported fields**

| Field | Description | Type |
|---|---|---|
| error.message | Error message. | text |

An example event for ` + "`error`" + ` looks as following:

` + "```json" + `
{"error": "fail"}
` + "```" + `

End of docs.`,
			want: `# Access logs

# Error logs

End of docs.`,
		},
		{
			name: "example_event_with_blank_line_gap",
			in: `Intro text.

An example event for ` + "`test`" + ` looks as following:


` + "```json" + `
{"test": true}
` + "```" + `

End.`,
			want: `Intro text.

End.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripFieldTables(tt.in)
			if got != tt.want {
				t.Errorf("stripFieldTables() mismatch\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
