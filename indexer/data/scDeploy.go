package data

// ScDeployInfo is the DTO that holds information about a smart contract deployment
type ScDeployInfo struct {
	TxHash            string     `json:"deployTxHash"`
	Creator           string     `json:"deployer"`
	Timestamp         uint64     `json:"timestamp"`
	Upgrades          []*Upgrade `json:"upgrades"`
	TotalTransactions uint64     `json:"totalTransactions"`
}

// Upgrade is the DTO that holds information about a smart contract upgrade
type Upgrade struct {
	TxHash    string `json:"upgradeTxHash"`
	Upgrader  string `json:"upgrader"`
	Timestamp uint64 `json:"timestamp"`
}

// PreparedLogsResults is the DTO that holds all the results after processing
type PreparedLogsResults struct {
	ScDeploys  map[string]*ScDeployInfo
	AlteredSCs AlteredSmartContractsHandler
}
