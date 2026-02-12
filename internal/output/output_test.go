package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestUI() (*UI, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &UI{Out: out, ErrOut: errOut}, out, errOut
}

func TestInfo(t *testing.T) {
	u, out, _ := newTestUI()
	u.Info("hello %s", "world")
	assert.Contains(t, out.String(), "hello world")
}

func TestSuccess(t *testing.T) {
	u, out, _ := newTestUI()
	u.Success("done %d", 42)
	assert.Contains(t, out.String(), "done 42")
}

func TestWarning(t *testing.T) {
	u, _, errOut := newTestUI()
	u.Warning("careful %s", "now")
	assert.Contains(t, errOut.String(), "careful now")
}

func TestError(t *testing.T) {
	u, _, errOut := newTestUI()
	u.Error("failed %s", "badly")
	assert.Contains(t, errOut.String(), "failed badly")
}

func TestVerboseLog_Enabled(t *testing.T) {
	u, out, _ := newTestUI()
	u.Verbose = true
	u.VerboseLog("detail %d", 1)
	assert.Contains(t, out.String(), "detail 1")
}

func TestVerboseLog_Disabled(t *testing.T) {
	u, out, _ := newTestUI()
	u.Verbose = false
	u.VerboseLog("detail %d", 1)
	assert.Empty(t, out.String())
}

func TestDryRunMsg_Enabled(t *testing.T) {
	u, _, errOut := newTestUI()
	u.DryRun = true
	u.DryRunMsg("would create %s", "file")
	assert.Contains(t, errOut.String(), "[DRY-RUN]")
	assert.Contains(t, errOut.String(), "would create file")
}

func TestDryRunMsg_Disabled(t *testing.T) {
	u, _, errOut := newTestUI()
	u.DryRun = false
	u.DryRunMsg("would create %s", "file")
	assert.Empty(t, errOut.String())
}

func TestColorHelpers(t *testing.T) {
	// Color helpers should return non-empty strings
	assert.NotEmpty(t, Cyan("test"))
	assert.NotEmpty(t, Green("test"))
	assert.NotEmpty(t, Yellow("test"))
	assert.NotEmpty(t, Red("test"))
}

func TestStatusColor(t *testing.T) {
	assert.NotEmpty(t, StatusColor("open"))
	assert.NotEmpty(t, StatusColor("in_progress"))
	assert.NotEmpty(t, StatusColor("done"))
	assert.NotEmpty(t, StatusColor("closed"))
	assert.Equal(t, "unknown", StatusColor("unknown"))
}

func TestHealthColor(t *testing.T) {
	assert.NotEmpty(t, HealthColor(90))
	assert.NotEmpty(t, HealthColor(60))
	assert.NotEmpty(t, HealthColor(30))
}

func TestTable(t *testing.T) {
	u, out, _ := newTestUI()
	table := u.Table([]string{"Name", "Status"})
	require.NotNil(t, table)

	table.Append([]string{"pm", "active"})
	table.Append([]string{"wt", "stable"})
	err := table.Render()
	require.NoError(t, err)

	result := out.String()
	assert.True(t, strings.Contains(result, "pm") || strings.Contains(result, "PM"),
		"table output should contain project names")
	assert.True(t, strings.Contains(result, "wt") || strings.Contains(result, "WT"),
		"table output should contain project names")
}
