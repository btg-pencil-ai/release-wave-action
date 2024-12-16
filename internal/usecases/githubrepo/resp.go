package githubrepo

type RespWorkflow struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Path string `json:"path"`
	Repo string `json:"repo"`
}
