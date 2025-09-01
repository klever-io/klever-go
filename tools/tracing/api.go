package tracing

import (
	"fmt"
)

// StartSpan starts a new span if tracing is enabled, with optional tags.
// Returns a no-op function if tracing is disabled.
// Usage: defer tracing.StartSpan("operation.name", "key1", "value1", "key2", "value2")()
func StartSpan(name string, tagsKV ...string) func() {
	if !IsEnabled() {
		return func() {
			// No-op function returned when tracing is disabled.
			// This allows callers to use defer without checking if tracing is enabled.
		}
	}

	tracer := GetConfiguredTracer()
	if tracer == nil {
		return func() {
			// No-op function returned when tracer is not configured.
			// This prevents nil pointer dereference in edge cases.
		}
	}

	// Parse tags from variadic key-value pairs
	tags := make(map[string]string)
	for i := 0; i < len(tagsKV)-1; i += 2 {
		tags[tagsKV[i]] = tagsKV[i+1]
	}

	if len(tags) > 0 {
		tracer.StartWithTags(name, tags)
	} else {
		tracer.Start(name)
	}

	// Return the stop function
	return func() {
		tracer.Stop(name)
	}
}

// StartSpanWithTags starts a new span if tracing is enabled with a map of tags.
// Returns a no-op function if tracing is disabled.
// Usage: defer tracing.StartSpanWithTags("operation.name", map[string]string{"key": "value"})()
func StartSpanWithTags(name string, tags map[string]string) func() {
	if !IsEnabled() {
		return func() {
			// No-op function returned when tracing is disabled.
			// This allows callers to use defer without checking if tracing is enabled.
		}
	}

	tracer := GetConfiguredTracer()
	if tracer == nil {
		return func() {
			// No-op function returned when tracer is not configured.
			// This prevents nil pointer dereference in edge cases.
		}
	}

	tracer.StartWithTags(name, tags)

	// Return the stop function
	return func() {
		tracer.Stop(name)
	}
}

// StartSpanf starts a new span with a formatted name if tracing is enabled.
// Returns a no-op function if tracing is disabled.
// Usage: defer tracing.StartSpanf("consensus.slot.%d", slotIndex)()
func StartSpanf(format string, args ...any) func() {
	if !IsEnabled() {
		return func() {
			// No-op function returned when tracing is disabled.
			// This allows callers to use defer without checking if tracing is enabled.
		}
	}

	name := fmt.Sprintf(format, args...)
	return StartSpan(name)
}

// StartSpanfWithTags starts a new span with formatted name and tags if tracing is enabled.
// Returns a no-op function if tracing is disabled.
// Usage: defer tracing.StartSpanfWithTags("consensus.slot.%d", slotIndex, map[string]string{"key": "value"})()
func StartSpanfWithTags(format string, args ...any) func() {
	if !IsEnabled() {
		return func() {
			// No-op function returned when tracing is disabled.
			// This allows callers to use defer without checking if tracing is enabled.
		}
	}

	// Last argument should be the tags map if provided
	var tags map[string]string
	var formatArgs []any

	if len(args) > 0 {
		if lastArg, ok := args[len(args)-1].(map[string]string); ok {
			tags = lastArg
			formatArgs = args[:len(args)-1]
		} else {
			formatArgs = args
		}
	}

	name := fmt.Sprintf(format, formatArgs...)

	if tags != nil {
		return StartSpanWithTags(name, tags)
	}
	return StartSpan(name)
}

// TraceIf executes the provided function only if tracing is enabled.
// This is useful for expensive operations that should only run when tracing.
func TraceIf(fn func(*ZipkinTracer)) {
	if !IsEnabled() {
		return
	}

	tracer := GetConfiguredTracer()
	if tracer == nil {
		return
	}

	fn(tracer)
}

// AddTag adds a tag to the current active span if tracing is enabled.
func AddTag(key, value string) {
	if !IsEnabled() {
		return
	}

	tracer := GetConfiguredTracer()
	if tracer == nil {
		return
	}

	tracer.AddTag(key, value)
}

// AddTagf adds a formatted tag to the current active span if tracing is enabled.
func AddTagf(key, format string, args ...any) {
	if !IsEnabled() {
		return
	}

	AddTag(key, fmt.Sprintf(format, args...))
}

// AddAnnotation adds an annotation to the current active span if tracing is enabled.
func AddAnnotation(value string) {
	if !IsEnabled() {
		return
	}

	tracer := GetConfiguredTracer()
	if tracer == nil {
		return
	}

	tracer.AddAnnotation(value)
}

// AddAnnotationf adds a formatted annotation to the current active span if tracing is enabled.
func AddAnnotationf(format string, args ...any) {
	if !IsEnabled() {
		return
	}

	AddAnnotation(fmt.Sprintf(format, args...))
}

// StopSpan stops a span by name if tracing is enabled.
// Safe to call even if tracing is disabled or the span doesn't exist.
func StopSpan(name string) {
	if !IsEnabled() {
		return
	}

	tracer := GetConfiguredTracer()
	if tracer == nil {
		return
	}

	tracer.Stop(name)
}

// StopSpanf stops a span with a formatted name if tracing is enabled.
// Safe to call even if tracing is disabled or the span doesn't exist.
func StopSpanf(format string, args ...any) {
	if !IsEnabled() {
		return
	}

	name := fmt.Sprintf(format, args...)
	StopSpan(name)
}
