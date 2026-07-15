package litellm

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTeam_AllowsNonUUIDTeamID(t *testing.T) {
	const teamID = "external-team-id"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET request, got %s", r.Method)
		}
		if got := r.URL.Query().Get("team_id"); got != teamID {
			t.Fatalf("expected team_id %q, got %q", teamID, got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"team_id":"external-team-id"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", true)

	resp, err := client.GetTeam(teamID)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := resp["team_id"]; got != teamID {
		t.Fatalf("expected team_id %q in response, got %#v", teamID, got)
	}
}
