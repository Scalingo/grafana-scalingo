package main

#Dashboard: {
	// Unique numeric identifier for the dashboard. (generated by the db)
	id: int
	// Unique dashboard identifier that can be generated by anyone. string (8-40)
	uid: string
	// Title of dashboard.
	title?: string
	// Description of dashboard.
	description?: string
	// Tags associated with dashboard.
	tags?: [...string]
	// Theme of dashboard.
	style: *"light" | "dark"
	// Timezone of dashboard,
	timezone?: *"browser" | "utc"
	// Whether a dashboard is editable or not.
	editable: bool | *true
	// 0 for no shared crosshair or tooltip (default).
	// 1 for shared crosshair.
	// 2 for shared crosshair AND shared tooltip.
	graphTooltip: int >= 0 <= 2 | *0
	// Time range for dashboard, e.g. last 6 hours, last 7 days, etc
	time?: {
		from: string | *"now-6h"
		to:   string | *"now"
	}
	// Timepicker metadata.
	timepicker?: {
		// Whether timepicker is collapsed or not.
		collapse: bool | *false
		// Whether timepicker is enabled or not.
		enable: bool | *true
		// Whether timepicker is visible or not.
		hidden: bool | *false
		// Selectable intervals for auto-refresh.
		refresh_intervals: [...string] | *["5s", "10s", "30s", "1m", "5m", "15m", "30m", "1h", "2h", "1d"]
	}
	// Templating.
	templating?: list: [...{}]
	// Annotations.
	annotations?: list: [...{
		builtIn: int | *0
		// Datasource to use for annotation.
		datasource: string
		// Whether annotation is enabled.
		enable?: bool | *true
		// Whether to hide annotation.
		hide?: bool | *false
		// Annotation icon color.
		iconColor?: string
		// Name of annotation.
		name?: string
		// Query for annotation data.
		rawQuery: string
		showIn:   int | *0
	}] | *[]
	// Auto-refresh interval.
	refresh: string
	// Version of the JSON schema, incremented each time a Grafana update brings
	// changes to said schema.
	schemaVersion: int | *25
	// Version of the dashboard, incremented each time the dashboard is updated.
	version: string
	// Dashboard panels.
	panels?: [...{}]
}
