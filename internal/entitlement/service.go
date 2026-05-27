package entitlement

type Entitlement struct {
	Key       string  `json:"key"`
	Status    string  `json:"status"`
	ExpiresAt *string `json:"expires_at"`
}

type Result struct {
	AccountType  string        `json:"account_type"`
	Entitlements []Entitlement `json:"entitlements"`
}

func (r Result) Has(key string) bool {
	for _, entitlement := range r.Entitlements {
		if entitlement.Key == key && entitlement.Status == "active" {
			return true
		}
	}
	return false
}

type Service struct{}

func NewService(_ any) *Service {
	return &Service{}
}

func (s *Service) ListForUser(userID uint64, accountType string) (Result, error) {
	if accountType == "" {
		accountType = "normal"
	}
	return Result{
		AccountType: accountType,
		Entitlements: []Entitlement{
			{Key: "backup.reading_data", Status: "active", ExpiresAt: nil},
		},
	}, nil
}
