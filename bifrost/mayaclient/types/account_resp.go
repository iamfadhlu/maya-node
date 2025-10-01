package types

// AccountResp the response from mayaclient
type AccountResp struct {
	Height string `json:"height"`
	Result struct {
		Value struct {
			AccountNumber uint64 `json:"account_number,string"`
			Sequence      uint64 `json:"sequence,string"`
		} `json:"value"`
	} `json:"result"`
}
