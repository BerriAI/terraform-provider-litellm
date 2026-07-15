package litellm

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// setTeamMemberBudgetPayload handles Terraform's optional numeric semantics:
// - create with field unset: omit the field entirely
// - update with field removed from config: send null to clear the budget row
// - explicit 0 remains 0
func setTeamMemberBudgetPayload(payload map[string]interface{}, d *schema.ResourceData, clearWhenUnset bool) {
	if v, ok := d.GetOkExists("max_budget_in_team"); ok {
		payload["max_budget_in_team"] = v.(float64)
		return
	}

	if clearWhenUnset {
		payload["max_budget_in_team"] = nil
	}
}

func readTeamMemberState(client *Client, teamID, userID, currentEmail string) (bool, string, string, *float64, error) {
	teamResp, err := client.GetTeam(teamID)
	if err != nil {
		if strings.Contains(err.Error(), "status code 404") {
			return false, "", "", nil, nil
		}
		return false, "", "", nil, fmt.Errorf("error reading team %s: %w", teamID, err)
	}

	found := false
	userEmail := currentEmail
	role := ""
	var maxBudget *float64

	if membersWithRoles, ok := teamResp["members_with_roles"].([]interface{}); ok {
		for _, rawMember := range membersWithRoles {
			member, ok := rawMember.(map[string]interface{})
			if !ok {
				continue
			}

			memberUserID, _ := member["user_id"].(string)
			memberUserEmail, _ := member["user_email"].(string)
			if memberUserID != userID && !strings.EqualFold(memberUserEmail, currentEmail) {
				continue
			}

			found = true
			if memberUserEmail != "" {
				userEmail = memberUserEmail
			}
			if memberRole, ok := member["role"].(string); ok {
				role = memberRole
			}
			break
		}
	}

	if teamMemberships, ok := teamResp["team_memberships"].([]interface{}); ok {
		for _, rawMembership := range teamMemberships {
			membership, ok := rawMembership.(map[string]interface{})
			if !ok {
				continue
			}

			memberUserID, _ := membership["user_id"].(string)
			if memberUserID != userID {
				continue
			}

			found = true
			budgetTable, _ := membership["litellm_budget_table"].(map[string]interface{})
			if budgetTable == nil {
				break
			}

			if rawBudget, ok := budgetTable["max_budget"].(float64); ok {
				maxBudget = &rawBudget
			}
			break
		}
	}

	return found, userEmail, role, maxBudget, nil
}

func syncTeamMemberState(d *schema.ResourceData, client *Client) error {
	teamID := d.Get("team_id").(string)
	userID := d.Get("user_id").(string)

	if teamID == "" || userID == "" {
		parts := strings.SplitN(d.Id(), ":", 2)
		if len(parts) == 2 {
			if teamID == "" {
				teamID = parts[0]
			}
			if userID == "" {
				userID = parts[1]
			}
		}
	}

	if teamID == "" || userID == "" {
		return fmt.Errorf("team member ID %q is missing team_id or user_id", d.Id())
	}

	found, userEmail, role, maxBudget, err := readTeamMemberState(client, teamID, userID, d.Get("user_email").(string))
	if err != nil {
		return err
	}

	if !found {
		log.Printf("[WARN] Team member with ID %s not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err := d.Set("team_id", teamID); err != nil {
		return err
	}
	if err := d.Set("user_id", userID); err != nil {
		return err
	}
	if userEmail != "" {
		if err := d.Set("user_email", userEmail); err != nil {
			return err
		}
	}
	if role != "" {
		if err := d.Set("role", role); err != nil {
			return err
		}
	}
	if err := d.Set("max_budget_in_team", maxBudget); err != nil {
		return err
	}

	return nil
}
