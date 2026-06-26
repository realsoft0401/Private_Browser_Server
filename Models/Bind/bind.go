package Bind

import NodeModel "private_browser_server/Models/Node"

type BindRequest struct {
	AccountID         string `json:"accountId"`
	ClientIP          string `json:"clientIp"`
	NodeServerBaseURL string `json:"nodeServerBaseUrl"`
}

type PushClientIDRequest struct {
	AccountID         string `json:"accountId"`
	ClientID          string `json:"clientId"`
	NodeServerBaseURL string `json:"nodeServerBaseUrl"`
	Source            string `json:"source"`
	AssignedAt        int64  `json:"assignedAt"`
}

type UnbindRequest struct {
	Source string `json:"source"`
}

type BindResponse struct {
	ClientID    string                `json:"clientId"`
	AccountID   string                `json:"accountId"`
	Status      string                `json:"status"`
	ClientIP    string                `json:"clientIp"`
	BaseURL     string                `json:"baseUrl"`
	BindStatus  string                `json:"bindStatus"`
	PushStatus  string                `json:"pushStatus"`
	PushMessage string                `json:"pushMessage,omitempty"`
	Node        *NodeModel.EdgeClient `json:"node,omitempty"`
}

type UnbindResponse struct {
	ClientID                 string `json:"clientId"`
	AccountID                string `json:"accountId"`
	Status                   string `json:"status"`
	ClearRegistrationStatus  string `json:"clearRegistrationStatus"`
	ClearRegistrationMessage string `json:"clearRegistrationMessage,omitempty"`
	UnboundAt                int64  `json:"unboundAt"`
}
