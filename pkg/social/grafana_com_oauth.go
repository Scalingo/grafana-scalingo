package social

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/grafana/grafana/pkg/models"

	"golang.org/x/oauth2"
)

type SocialGrafanaCom struct {
	*oauth2.Config
	url                  string
	allowedOrganizations []string
	allowSignup          bool
}

type OrgRecord struct {
	Login string `json:"login"`
}

func (s *SocialGrafanaCom) Type() int {
	return int(models.GRAFANA_COM)
}

func (s *SocialGrafanaCom) IsEmailAllowed(email string) bool {
	return true
}

func (s *SocialGrafanaCom) IsSignupAllowed() bool {
	return s.allowSignup
}

func (s *SocialGrafanaCom) IsOrganizationMember(organizations []OrgRecord) bool {
	if len(s.allowedOrganizations) == 0 {
		return true
	}

	for _, allowedOrganization := range s.allowedOrganizations {
		for _, organization := range organizations {
			if organization.Login == allowedOrganization {
				return true
			}
		}
	}

	return false
}

func (s *SocialGrafanaCom) UserInfo(client *http.Client) (*BasicUserInfo, error) {
	var data struct {
		Name  string      `json:"name"`
		Login string      `json:"username"`
		Email string      `json:"email"`
		Role  string      `json:"role"`
		Orgs  []OrgRecord `json:"orgs"`
	}

	body, err := HttpGet(client, s.url+"/api/oauth2/user")
	if err != nil {
		return nil, fmt.Errorf("Error getting user info: %s", err)
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("Error getting user info: %s", err)
	}

	userInfo := &BasicUserInfo{
		Name:  data.Name,
		Login: data.Login,
		Email: data.Email,
		Role:  data.Role,
	}

	if !s.IsOrganizationMember(data.Orgs) {
		return nil, ErrMissingOrganizationMembership
	}

	return userInfo, nil
}
