package updatemanager

type UpdateResult struct {
	Operation  UpdateServiceOperation `json:"-"`
	Successful bool                   `json:"successful"`
	Error      error                  `json:"-"`
	Message    string                 `json:"message"`
}
