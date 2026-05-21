// Copyright IBM Corp. 2014, 2026
// SPDX-License-Identifier: MPL-2.0

package pinpoint

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/pinpoint"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// newTestClient builds a Pinpoint SDK client that talks to an httptest server.
// SigV4 is defused via static credentials; the mock handler ignores signatures.
// The region is metadata only — BaseEndpoint targets localhost.
func newTestClient(t *testing.T, h http.HandlerFunc) *pinpoint.Client {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return pinpoint.New(pinpoint.Options{
		Region:       "us-east-1", //lintignore:AWSAT003
		BaseEndpoint: aws.String(ts.URL),
		Credentials:  credentials.NewStaticCredentialsProvider("AKID", "SECRET", "SESSION"),
	})
}

// awsErrorJSON returns a handler that mimics a Pinpoint REST-JSON-1.1 error
// response. The deserializer at
// aws-sdk-go-v2/service/pinpoint/deserializers.go:7207-7234 prefers the
// X-Amzn-ErrorType header for code extraction and falls back to the body's
// __type field; setting both makes the response robust to deserializer
// changes that drop the body parse.
func awsErrorJSON(code, message string) http.HandlerFunc {
	body := fmt.Sprintf(`{"__type":%q,"message":%q}`, code, message)
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Amzn-ErrorType", code)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(body))
	}
}

// jsonHandler returns a handler that responds with the given status and a JSON
// body produced by formatting body verbatim. No serialization helper — keep
// the wire format obvious in each test.
func jsonHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// routeByPath dispatches by HTTP method + URL path suffix, matching pinpoint's
// REST-JSON-1.1 wire format. Pinpoint serializers populate request.Method +
// request.URL.Path; they do NOT set X-Amz-Target (which only AWS JSON 1.0/1.1
// protocols use).
//
// Path templates (from aws-sdk-go-v2/service/pinpoint/serializers.go):
//
//	GET  /v1/apps/{ApplicationId}            → GetApp
//	GET  /v1/apps/{ApplicationId}/settings   → GetApplicationSettings
//	PUT  /v1/apps/{ApplicationId}/settings   → UpdateApplicationSettings
//
// Route keys use the form "METHOD <suffix>" where suffix is the path remaining
// after /v1/apps/{appID} (empty string for GetApp). Examples: "GET ",
// "GET /settings", "PUT /settings".
func routeByPath(t *testing.T, routes map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + suffixAfterAppID(r.URL.Path)
		if h, ok := routes[key]; ok {
			h(w, r)
			return
		}
		t.Errorf("unexpected request: %s %s (key=%q)", r.Method, r.URL.Path, key)
		w.WriteHeader(http.StatusNotImplemented)
	}
}

// suffixAfterAppID returns the substring after /v1/apps/{anything}, or ""
// for /v1/apps/{anything} itself (GetApp's URL). Tolerates trailing slash.
func suffixAfterAppID(path string) string {
	const prefix = "/v1/apps/"
	if !strings.HasPrefix(path, prefix) {
		return path
	}
	rest := path[len(prefix):]
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[i:] // includes leading "/", e.g. "/settings"
	}
	return ""
}

// appResponseJSON is the canonical GetApp success body used by mock tests.
// The aws-sdk-go-v2 deserializer for ApplicationResponse is tolerant of
// missing optional fields (deserializers.go:22137-22207 iterates the map
// rather than asserting required keys), so this minimal shape is sufficient.
func appResponseJSON(id, arn, name string) string {
	return fmt.Sprintf(`{"Id":%q,"Arn":%q,"Name":%q}`, id, arn, name)
}

// appSettingsResponseJSON returns a GetApplicationSettings success body. The
// Pinpoint deserializer (deserializers.go:7183) decodes the response body
// directly as ApplicationSettingsResource (no envelope wrapper). All three
// Settings sub-objects must be present as non-nil empty objects because the
// in-repo flatteners (flattenCampaignHook, flattenCampaignLimits,
// flattenQuietTime in app.go) dereference top-level struct fields without
// a nil-guard. Empty objects deserialize as zero-valued structs which the
// flatteners handle safely via aws.ToString / aws.ToInt32 of nil pointers.
func appSettingsResponseJSON(id string) string {
	return fmt.Sprintf(`{"ApplicationId":%q,"CampaignHook":{},"Limits":{},"QuietTime":{}}`, id)
}

const (
	testAppID  = "abcdef0123456789abcdef0123456789"
	testAppARN = "arn:aws:mobiletargeting:us-east-1:123456789012:apps/abcdef0123456789abcdef0123456789" //lintignore:AWSAT003,AWSAT005
)

// TestFindAppSettingsByID_DeprecationError_Tolerated proves the wire-level
// plumbing: when GetApplicationSettings returns an AccessDeniedException
// envelope, findAppSettingsByID propagates an error whose code the matcher
// catches. This is the lower bound — if this fails, every R1/U1 test fails
// for the same reason.
func TestFindAppSettingsByID_DeprecationError_Tolerated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	conn := newTestClient(t, routeByPath(t, map[string]http.HandlerFunc{
		"GET /settings": awsErrorJSON("AccessDeniedException", "Settings API retired"),
	}))

	_, err := findAppSettingsByID(ctx, conn, testAppID)

	if err == nil {
		t.Fatalf("findAppSettingsByID returned nil error; want non-nil")
	}
	if !isSettingsAPIDeprecationError(err) {
		t.Fatalf("isSettingsAPIDeprecationError(%v) = false; want true", err)
	}
}

// TestReadAppWithConn_PreservesPriorStateOnSettingsDeprecation proves R1's
// core invariant: on a deprecation-class GetApplicationSettings error, Read
// preserves prior state for the three Settings attributes by NOT calling
// d.Set on them. App-API attributes are still populated from the successful
// GetApp response.
func TestReadAppWithConn_PreservesPriorStateOnSettingsDeprecation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	conn := newTestClient(t, routeByPath(t, map[string]http.HandlerFunc{
		"GET ":          jsonHandler(http.StatusOK, appResponseJSON(testAppID, testAppARN, "name-from-aws")),
		"GET /settings": awsErrorJSON("AccessDeniedException", "Settings API retired"),
	}))

	d := resourceApp().TestResourceData()
	d.SetId(testAppID)
	if err := d.Set("quiet_time", []any{map[string]any{"start": "00:00", "end": "06:00"}}); err != nil {
		t.Fatalf("seeding quiet_time: %s", err)
	}
	if err := d.Set("limits", []any{map[string]any{"daily": 5}}); err != nil {
		t.Fatalf("seeding limits: %s", err)
	}

	diags := readAppWithConn(ctx, conn, d)

	if diags.HasError() {
		t.Fatalf("diags.HasError() = true; want false. diags=%v", diags)
	}
	// App-API attr was set from the GetApp response.
	if got := d.Get(names.AttrName); got != "name-from-aws" {
		t.Errorf("Name = %q; want %q (App-API attr should be set from AWS)", got, "name-from-aws")
	}
	// Settings attrs preserved from pre-test seed (R1 skipped d.Set).
	qt := d.Get("quiet_time").([]any)
	if len(qt) != 1 {
		t.Fatalf("quiet_time len = %d; want 1 (R1 should have preserved seeded value)", len(qt))
	}
	if got := qt[0].(map[string]any)["end"]; got != "06:00" {
		t.Errorf("quiet_time.0.end = %q; want %q (R1 should preserve seeded value)", got, "06:00")
	}
	limits := d.Get("limits").([]any)
	if len(limits) != 1 {
		t.Fatalf("limits len = %d; want 1 (R1 should have preserved seeded value)", len(limits))
	}
	if got := limits[0].(map[string]any)["daily"]; got != 5 {
		t.Errorf("limits.0.daily = %v; want 5 (R1 should preserve seeded value)", got)
	}
}

// TestReadAppWithConn_NonDeprecationSettingsErrorStillEscalates proves R1's
// negative invariant: on a non-deprecation-class GetApplicationSettings error,
// the existing error path is preserved — the call escalates to a diag.Error.
// Coverage gap 4b.
func TestReadAppWithConn_NonDeprecationSettingsErrorStillEscalates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	conn := newTestClient(t, routeByPath(t, map[string]http.HandlerFunc{
		"GET ":          jsonHandler(http.StatusOK, appResponseJSON(testAppID, testAppARN, "n")),
		"GET /settings": awsErrorJSON("ValidationException", "bad request"),
	}))

	d := resourceApp().TestResourceData()
	d.SetId(testAppID)

	diags := readAppWithConn(ctx, conn, d)

	if !diags.HasError() {
		t.Fatalf("diags.HasError() = false; want true. diags=%v", diags)
	}
}

// TestReadAppWithConn_AppAPIForbiddenStillEscalates proves the R1 matcher
// does not extend to App-API errors. ForbiddenException on GetApp is caught
// by the existing error path before the Settings call is even reached.
// Coverage gap 4c — today preserved by code structure (GetApp error at the
// top); the test locks the invariant against future refactor drift.
func TestReadAppWithConn_AppAPIForbiddenStillEscalates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	conn := newTestClient(t, routeByPath(t, map[string]http.HandlerFunc{
		"GET ": awsErrorJSON("ForbiddenException", "denied by IAM"),
	}))

	d := resourceApp().TestResourceData()
	d.SetId(testAppID)

	diags := readAppWithConn(ctx, conn, d)

	if !diags.HasError() {
		t.Fatalf("diags.HasError() = false; want true. App-API errors must not be swallowed by R1.")
	}
}

// TestApplySettingsUpdate_SwallowsDeprecationEmitsWarning proves U1's positive
// invariant: on a deprecation-class UpdateApplicationSettings error,
// applySettingsUpdate swallows the error, emits exactly one diag.Warning,
// and returns no error. Seeds quiet_time on d so d.HasChange returns true
// (the SDKv2 diff layer isn't available under TestResourceData, but d.Set
// before the call simulates the SDKv2-lifted planned value).
func TestApplySettingsUpdate_SwallowsDeprecationEmitsWarning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	conn := newTestClient(t, routeByPath(t, map[string]http.HandlerFunc{
		"PUT /settings": awsErrorJSON("AccessDeniedException", "Settings API retired"),
	}))

	d := resourceApp().TestResourceData()
	d.SetId(testAppID)
	if err := d.Set("quiet_time", []any{map[string]any{"start": "22:00", "end": "09:00"}}); err != nil {
		t.Fatalf("seeding quiet_time: %s", err)
	}

	diags := applySettingsUpdate(ctx, conn, d)

	if diags.HasError() {
		t.Fatalf("diags.HasError() = true; want false. diags=%v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("len(diags) = %d; want 1. diags=%v", len(diags), diags)
	}
	if diags[0].Severity != diag.Warning {
		t.Errorf("diags[0].Severity = %v; want diag.Warning", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Summary, "Settings API has been retired") {
		t.Errorf("diags[0].Summary = %q; want it to mention \"Settings API has been retired\"", diags[0].Summary)
	}
}

// TestApplySettingsUpdate_RealErrorStillFails proves U1's negative invariant:
// non-deprecation errors from UpdateApplicationSettings still escalate to
// diag.Error.
func TestApplySettingsUpdate_RealErrorStillFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	conn := newTestClient(t, routeByPath(t, map[string]http.HandlerFunc{
		"PUT /settings": awsErrorJSON("ValidationException", "bad request"),
	}))

	d := resourceApp().TestResourceData()
	d.SetId(testAppID)
	if err := d.Set("quiet_time", []any{map[string]any{"start": "22:00", "end": "09:00"}}); err != nil {
		t.Fatalf("seeding quiet_time: %s", err)
	}

	diags := applySettingsUpdate(ctx, conn, d)

	if !diags.HasError() {
		t.Fatalf("diags.HasError() = false; want true. diags=%v", diags)
	}
}

// TestUpdateAppWithConn_TagOnlyChangeDoesNotCallSettings proves the
// HasChangesExcept(tags) guard at the top of updateAppWithConn: when no
// non-tag change is present, applySettingsUpdate is not entered and
// UpdateApplicationSettings is not called. Coverage gap 4h.
//
// Implementation: routeByPath has NO entry for "PUT /settings", so if the
// code path reaches UpdateApplicationSettings the harness's default t.Errorf
// fires. The Read tail-call still runs, so GET / and GET /settings routes
// are registered with happy-path responses.
//
// Note: with a bare TestResourceData (d.diff == nil), HasChangesExcept
// returns false, mimicking the tag-only-diff short-circuit. A strict
// tag-only-diff test would require hand-building a *terraform.InstanceDiff;
// the marginal extra signal isn't worth the infrastructure cost.
func TestUpdateAppWithConn_TagOnlyChangeDoesNotCallSettings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	conn := newTestClient(t, routeByPath(t, map[string]http.HandlerFunc{
		"GET ":          jsonHandler(http.StatusOK, appResponseJSON(testAppID, testAppARN, "n")),
		"GET /settings": jsonHandler(http.StatusOK, appSettingsResponseJSON(testAppID)),
		// No "PUT /settings" route — routeByPath's t.Errorf will fire if
		// updateAppWithConn reaches UpdateApplicationSettings despite the guard.
	}))

	d := resourceApp().TestResourceData()
	d.SetId(testAppID)

	diags := updateAppWithConn(ctx, conn, d)

	if diags.HasError() {
		t.Fatalf("diags.HasError() = true; want false. diags=%v", diags)
	}
}
