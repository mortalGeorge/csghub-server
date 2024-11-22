package types

type Clabel struct {
	RepositoryID   int64  `json:"-"`
	Path           string `json:"path"`
	Ref            string `json:"ref"`
	FileName       string `json:"file_name"`
	RepoNamespace  string `json:"repo_namespace"`
	RepoName       string `json:"repo_name"`
	Label          string `json:"label"`
	AnnotationPath string `json:"annotation_path"`
}

type UpsertOneClabelReq struct {
	//RepositoryID   int    `json:"repository_id"`
	Path           string `json:"-"`
	Namespace      string `json:"-"`
	Name           string `json:"-"`
	Label          string `json:"label"`
	AnnotationPath string `json:"annotation_path"`
	CurrentUser    string `json:"-"`
	Ref            string `json:"ref"`
	RepoType       RepositoryType
}

type GetClabelReq struct {
	Path        string `json:"-"`
	Namespace   string `json:"-"`
	Name        string `json:"-"`
	Ref         string `json:"ref"`
	CurrentUser string `json:"-"`
	RepoType    RepositoryType
}

type CmccFilesFilter struct {
	Tags       []TagReq
	Sort       string
	Search     string
	FileSearch string
	Source     string
	Username   string
}
