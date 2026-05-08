package config

import "testing"

// TestShouldEnforceCPUPreflight covers the safe-default semantics: when the
// YAML key is absent (pointer is nil), enforcement defaults to true so an
// existing operator config that omits the new key cannot silently
// downgrade the validator startup gate to warn-only.
func TestShouldEnforceCPUPreflight(t *testing.T) {
	t.Run("nil pointer (absent YAML key) defaults to true", func(t *testing.T) {
		p := PreferencesConfig{EnforceCPUPreflight: nil}
		if !p.ShouldEnforceCPUPreflight() {
			t.Fatal("expected ShouldEnforceCPUPreflight() to return true when EnforceCPUPreflight is nil")
		}
	})

	t.Run("explicit true returns true", func(t *testing.T) {
		v := true
		p := PreferencesConfig{EnforceCPUPreflight: &v}
		if !p.ShouldEnforceCPUPreflight() {
			t.Fatal("expected ShouldEnforceCPUPreflight() to return true when EnforceCPUPreflight is *true")
		}
	})

	t.Run("explicit false returns false", func(t *testing.T) {
		v := false
		p := PreferencesConfig{EnforceCPUPreflight: &v}
		if p.ShouldEnforceCPUPreflight() {
			t.Fatal("expected ShouldEnforceCPUPreflight() to return false when EnforceCPUPreflight is *false")
		}
	})
}
