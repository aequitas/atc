package db

type Team struct {
	Name string
	BasicAuth
	GithubAuth
}

type BasicAuth struct {
	BasicAuthUsername string `json:"basic_auth_username"`
	BasicAuthPassword string `json:"basic_auth_password"`
}

type GithubAuth struct {
	ClientID      string       `json:"client_id"`
	ClientSecret  string       `json:"client_secret"`
	Organizations []string     `json:"organizations"`
	Teams         []GitHubTeam `json:"teams"`
	Users         []string     `json:"users"`
}

type GitHubTeam struct {
	OrganizationName string `json:"organization_name"`
	TeamName         string `json:"team_name"`
}

type SavedTeam struct {
	ID int
	Team
}
