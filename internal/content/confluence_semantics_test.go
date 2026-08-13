package content

import (
	"strings"
	"testing"
)

const tenantJiraInlineFixture = `<ac:structured-macro ac:name="jira" ac:schema-version="1" ac:local-id="21586ce4dc4c" ac:macro-id="87b1246b-6ab2-46bc-bcfc-ba24efb52118"><ac:parameter ac:name="key">PSS-3369</ac:parameter><ac:parameter ac:name="serverId">4a67abd8-f396-3524-919a-398ffb606bf7</ac:parameter><ac:parameter ac:name="server">System Jira</ac:parameter></ac:structured-macro>`

const tenantExpandedTableFixture = `<table data-table-width="1347" data-layout="center" ac:local-id="ef87037a6d27"><tbody><tr><th><p>Epic</p></th><th><p>Ticket</p></th></tr><tr><td><p>PSS-3369</p></td><td><p>PSS-3520</p></td></tr></tbody></table>`

func TestRenderStorageExposesTenantJiraInlineMacro(t *testing.T) {
	artifact, err := RenderStorage(testStoragePage(tenantJiraInlineFixture))
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}
	want := `[PSS-3369](jira:PSS-3369){conflux-display=inline jira-server="System Jira" jira-server-id="4a67abd8-f396-3524-919a-398ffb606bf7"}`
	if strings.TrimSpace(artifact.Markdown) != want {
		t.Fatalf("Markdown = %q, want %q", strings.TrimSpace(artifact.Markdown), want)
	}
	if len(artifact.Metadata.PreservedFragments) != 0 {
		t.Fatalf("Jira macro remained opaque: %#v", artifact.Metadata.PreservedFragments)
	}
}

func TestRenderArtifactRestoresTenantJiraInlineMacro(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	markdown := `[PSS-3369](jira:PSS-3369){conflux-display=inline jira-server="System Jira" jira-server-id="4a67abd8-f396-3524-919a-398ffb606bf7"}`
	artifact, err := RenderArtifact(markdown, metadata, nil)
	if err != nil {
		t.Fatalf("RenderArtifact returned error: %v", err)
	}
	for _, expected := range []string{`ac:name="jira"`, `<ac:parameter ac:name="key">PSS-3369</ac:parameter>`, `<ac:parameter ac:name="server">System Jira</ac:parameter>`} {
		if !strings.Contains(artifact.Storage, expected) {
			t.Fatalf("storage does not contain %q: %s", expected, artifact.Storage)
		}
	}
}

func TestRenderStorageExposesTenantExpandedTable(t *testing.T) {
	artifact, err := RenderStorage(testStoragePage(tenantExpandedTableFixture))
	if err != nil {
		t.Fatalf("RenderStorage returned error: %v", err)
	}
	for _, expected := range []string{`<!-- conflux:table layout="center" width="1347" -->`, "| Epic", "PSS-3520"} {
		if !strings.Contains(artifact.Markdown, expected) {
			t.Fatalf("Markdown does not contain %q: %s", expected, artifact.Markdown)
		}
	}
}

func TestRenderArtifactRestoresTenantExpandedTable(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	markdown := "<!-- conflux:table layout=\"center\" width=\"1347\" -->\n| Epic | Ticket |\n| --- | --- |\n| PSS-3369 | PSS-3520 |\n"
	artifact, err := RenderArtifact(markdown, metadata, nil)
	if err != nil {
		t.Fatalf("RenderArtifact returned error: %v", err)
	}
	for _, expected := range []string{`<table data-layout="center" data-table-width="1347">`, "<th><p>Epic</p></th>", "<td><p>PSS-3520</p></td>"} {
		if !strings.Contains(artifact.Storage, expected) {
			t.Fatalf("storage does not contain %q: %s", expected, artifact.Storage)
		}
	}
}

func TestRenderArtifactRejectsDetachedOrMalformedTableDirective(t *testing.T) {
	metadata := pushMetadata()
	metadata.PreservedFragments = map[string]string{}
	for name, markdown := range map[string]string{
		"detached":   "<!-- conflux:table layout=\"center\" width=\"1347\" -->\n\n| A |\n| --- |\n",
		"bad rows":   "<!-- conflux:table layout=\"center\" -->\n| A | B |\n| --- | --- |\n| only one |\n",
		"bad layout": "<!-- conflux:table layout=\"huge\" -->\n| A |\n| --- |\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := RenderArtifact(markdown, metadata, nil)
			if err == nil {
				t.Fatal("RenderArtifact unexpectedly succeeded")
			}
		})
	}
}
